package output

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/bitrise-io/go-android/v2/testresult/junitxml"
	"github.com/bitrise-io/go-steputils/v2/testattachment"
	"github.com/bitrise-io/go-steputils/v2/testreport"
)

func (e exporter) exportAttachments(testDeployDir string, resultXMLsByReportDir map[string][]string, projectDir string) {
	if len(resultXMLsByReportDir) == 0 {
		return
	}

	absProjectDir, err := filepath.Abs(projectDir)
	if err != nil {
		e.logger.Warnf("Failed to resolve project dir (%s), attachments are not exported: %s", projectDir, err)
		return
	}

	deployDir := e.envRepository.Get("BITRISE_TEST_DEPLOY_DIR")
	if deployDir == "" {
		e.logger.Warnf("BITRISE_TEST_DEPLOY_DIR is not set, attachments exported by earlier steps are not filtered out")
	}

	reportDirs := make([]string, 0, len(resultXMLsByReportDir))
	for reportDir := range resultXMLsByReportDir {
		reportDirs = append(reportDirs, reportDir)
	}
	sort.Strings(reportDirs)

	for _, reportDir := range reportDirs {
		e.exportReportAttachments(absProjectDir, deployDir, testDeployDir, reportDir, resultXMLsByReportDir[reportDir])
	}
}

func (e exporter) exportReportAttachments(projectDir, deployDir, testDeployDir, reportDir string, resultXMLs []string) {
	report, err := readTestReport(resultXMLs)
	if err != nil {
		e.logger.Warnf("Failed to read test cases, attachments of %s are not exported: %s", reportDir, err)
		return
	}

	result, err := e.attachmentCollector.Collect(projectDir, deployDir, testattachment.NewIndex(&report))
	if err != nil {
		e.logger.Warnf("Failed to collect attachments of %s: %s", reportDir, err)
		return
	}
	if result.GitCheckErr != nil {
		e.logger.Warnf("Failed to check which files are tracked by git, committed files are not filtered out: %s", result.GitCheckErr)
	}
	e.logSkippedAttachments(result.Skipped)

	if len(result.Candidates) == 0 {
		return
	}

	e.logger.Printf("Exporting %d attachments => %s", len(result.Candidates), filepath.Join("$BITRISE_TEST_RESULT_DIR", reportDir))
	for _, failed := range e.attachmentCollector.CopyToReport(filepath.Join(testDeployDir, reportDir), result.Candidates) {
		e.logger.Warnf("Failed to export attachment %s: %s", failed.Path, failed.Reason)
	}
}

func readTestReport(resultXMLs []string) (testreport.TestReport, error) {
	var converter junitxml.Converter
	if !converter.Detect(resultXMLs) {
		return testreport.TestReport{}, fmt.Errorf("no JUnit XML among %v", resultXMLs)
	}
	return converter.Convert()
}

func (e exporter) logSkippedAttachments(skipped []testattachment.Skipped) {
	for _, s := range skipped {
		switch {
		case errors.Is(s.Reason, testattachment.ErrDuplicateName),
			errors.Is(s.Reason, testattachment.ErrAmbiguousTest),
			errors.Is(s.Reason, testattachment.ErrUnknownRun),
			errors.Is(s.Reason, testattachment.ErrMissingLabel):
			e.logger.Warnf("Skipping attachment %s: %s", s.Path, s.Reason)
		default:
			e.logger.Debugf("Skipping attachment %s: %s", s.Path, s.Reason)
		}
	}
}
