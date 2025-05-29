package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/bitrise-io/go-android/v2/gradle"
	"github.com/bitrise-io/go-steputils/v2/stepconf"
	"github.com/bitrise-io/go-utils/v2/command"
	"github.com/bitrise-io/go-utils/v2/env"
	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-utils/v2/pathutil"
	"github.com/bitrise-steplib/steps-deploy-to-bitrise-io/test/junit"

	"github.com/bitrise-steplib/bitrise-step-android-unit-test/testaddon"
	"github.com/bitrise-steplib/steps-deploy-to-bitrise-io/test/converters/junitxml"

	"github.com/kballard/go-shellquote"
)

// Configs ...
type Configs struct {
	ProjectLocation      string `env:"project_location,dir"`
	HTMLResultDirPattern string `env:"report_path_pattern"`
	XMLResultDirPattern  string `env:"result_path_pattern"`
	Variant              string `env:"variant"`
	Module               string `env:"module"`
	Arguments            string `env:"arguments"`
	IsDebug              bool   `env:"is_debug,opt[true,false]"`

	DeployDir     string `env:"BITRISE_DEPLOY_DIR"`
	TestResultDir string `env:"BITRISE_TEST_RESULT_DIR"`
}

var cmdFactory = command.NewFactory(env.NewRepository())
var logger = log.NewLogger()

func failf(f string, args ...interface{}) {
	logger.Errorf(f, args...)
	os.Exit(1)
}

func getArtifacts(gradleProject gradle.Project, started time.Time, pattern string, includeModuleName bool, isDirectoryMode bool) (artifacts []gradle.Artifact, err error) {
	for _, t := range []time.Time{started, {}} {
		if isDirectoryMode {
			artifacts, err = gradleProject.FindDirs(t, pattern, includeModuleName)
		} else {
			artifacts, err = gradleProject.FindArtifacts(t, pattern, includeModuleName)
		}
		if err != nil {
			return
		}
		if len(artifacts) == 0 {
			if t == started {
				logger.Warnf("No artifacts found with pattern: %s that has modification time after: %s", pattern, t)
				logger.Warnf("Retrying without modtime check....")
				fmt.Println()
				continue
			}
			logger.Warnf("No artifacts found with pattern: %s without modtime check", pattern)
			logger.Warnf("If you have changed default report export path in your gradle files then you might need to change ReportPathPattern accordingly.")
		}
	}
	return
}

func workDirRel(pth string) (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Rel(wd, pth)
}

func exportArtifacts(pathChecker pathutil.PathChecker, deployDir string, artifacts []gradle.Artifact) error {
	for _, artifact := range artifacts {
		artifact.Name += ".zip"
		exists, err := pathChecker.IsPathExists(filepath.Join(deployDir, artifact.Name))
		if err != nil {
			return fmt.Errorf("failed to check path, error: %v", err)
		}

		if exists {
			timestamp := time.Now().Format("20060102150405")
			artifact.Name = fmt.Sprintf("%s-%s%s", strings.TrimSuffix(artifact.Name, ".zip"), timestamp, ".zip")
		}

		src := filepath.Base(artifact.Path)
		if rel, err := workDirRel(artifact.Path); err == nil {
			src = "./" + rel
		}

		logger.Printf("  Export [ %s => $BITRISE_DEPLOY_DIR/%s ]", src, artifact.Name)

		if err := artifact.ExportZIP(deployDir); err != nil {
			logger.Warnf("failed to export artifact (%s), error: %v", artifact.Path, err)
			continue
		}
	}
	return nil
}

func filterVariants(module, variant string, variantsMap gradle.Variants) (gradle.Variants, error) {
	// if module set: drop all the other modules
	if module != "" {
		v, ok := variantsMap[module]
		if !ok {
			return nil, fmt.Errorf("module not found: %s", module)
		}
		variantsMap = gradle.Variants{module: v}
	}
	// if variant not set: use all variants
	if variant == "" {
		return variantsMap, nil
	}
	filteredVariants := gradle.Variants{}
	for m, variants := range variantsMap {
		for _, v := range variants {
			if strings.ToLower(v) == strings.ToLower(variant+"UnitTest") {
				filteredVariants[m] = append(filteredVariants[m], v)
			}
		}
	}
	if len(filteredVariants) == 0 {
		return nil, fmt.Errorf("variant %s not found in any module", variant)
	}
	return filteredVariants, nil
}

func tryExportTestAddonArtifact(artifactPth, outputDir string, lastOtherDirIdx int) int {
	dir := getExportDir(artifactPth)

	if dir == OtherDirName {
		// start indexing other dir name, to avoid overrideing it
		// e.g.: other, other-1, other-2
		lastOtherDirIdx++
		if lastOtherDirIdx > 0 {
			dir = dir + "-" + strconv.Itoa(lastOtherDirIdx)
		}
	}

	if err := testaddon.ExportArtifact(artifactPth, outputDir, dir); err != nil {
		logger.Warnf("Failed to export test results for test addon: %s", err)
	} else {
		src := artifactPth
		if rel, err := workDirRel(artifactPth); err == nil {
			src = "./" + rel
		}
		logger.Printf("  Export [%s => %s]", src, filepath.Join("$BITRISE_TEST_RESULT_DIR", dir, filepath.Base(artifactPth)))
	}
	return lastOtherDirIdx
}

func main() {
	var config Configs

	envRepository := env.NewRepository()
	pathChecker := pathutil.NewPathChecker()
	inputParser := stepconf.NewInputParser(envRepository)

	if err := inputParser.Parse(&config); err != nil {
		failf("Process config: couldn't create step config: %v\n", err)
	}

	stepconf.Print(config)
	fmt.Println()

	logger.EnableDebugLog(config.IsDebug)

	gradleProject, err := gradle.NewProject(config.ProjectLocation, cmdFactory)
	if err != nil {
		failf("Process config: failed to open project, error: %s", err)
	}

	testTask := gradleProject.GetTask("test")

	args, err := shellquote.Split(config.Arguments)
	if err != nil {
		failf("Process config: failed to parse arguments, error: %s", err)
	}

	logger.Infof("Variants:")
	fmt.Println()

	variants, err := testTask.GetVariants(args...)
	if err != nil {
		failf("Run: failed to fetch variants, error: %s", err)
	}

	filteredVariants, err := filterVariants(config.Module, config.Variant, variants)
	if err != nil {
		failf("Run: failed to find buildable variants, error: %s", err)
	}

	for module, variants := range variants {
		logger.Printf("%s:", module)
		for _, variant := range variants {
			if slices.Contains(filteredVariants[module], variant) {
				logger.Donef("✓ %s", strings.TrimSuffix(variant, "UnitTest"))
			} else {
				logger.Printf("- %s", strings.TrimSuffix(variant, "UnitTest"))
			}
		}
	}
	fmt.Println()

	started := time.Now()

	var testErr error

	logger.Infof("Run test:")
	testCommand := testTask.GetCommand(filteredVariants, args...)

	fmt.Println()
	logger.Donef("$ " + testCommand.PrintableCommandArgs())
	fmt.Println()

	testErr = testCommand.Run()
	if testErr != nil {
		logger.Errorf("Run: test task failed, error: %v", testErr)
	}
	fmt.Println()
	logger.Infof("Export HTML results:")
	fmt.Println()

	reports, err := getArtifacts(gradleProject, started, config.HTMLResultDirPattern, true, true)
	if err != nil {
		failf("Export outputs: failed to find reports, error: %v", err)
	}

	if err := exportArtifacts(pathChecker, config.DeployDir, reports); err != nil {
		failf("Export outputs: failed to export reports, error: %v", err)
	}

	fmt.Println()
	logger.Infof("Export XML results:")
	fmt.Println()

	results, err := getArtifacts(gradleProject, started, config.XMLResultDirPattern, true, true)
	if err != nil {
		failf("Export outputs: failed to find results, error: %v", err)
	}

	if err := exportArtifacts(pathChecker, config.DeployDir, results); err != nil {
		failf("Export outputs: failed to export results, error: %v", err)
	}

	xmlResultFilePattern := config.XMLResultDirPattern
	if !strings.HasSuffix(xmlResultFilePattern, "*.xml") {
		xmlResultFilePattern += "*.xml"
	}

	resultXMLs, err := getArtifacts(gradleProject, started, xmlResultFilePattern, false, false)
	if err != nil {
		logger.Warnf("Failed to find test XML test results, error: %s", err)
	}

	testRetried := false
	var failingTestCases []junit.TestCase
	var flakyTestCases []junit.TestCase
	failedTestSuits, err := detectFailingTests(resultXMLs)
	if err != nil {
		logger.Warnf("Failed to detect failed tests: %s", err)
	} else if len(failedTestSuits) > 0 {
		logger.Printf("")
		logger.Warnf("Detected %d failed test suit(s)", len(failedTestSuits))

		for _, testSuit := range failedTestSuits {
			logger.Warnf("Detected %d failed test case(s) in test suit: %s", len(testSuit.TestCases), testSuit.Name)

			for _, testCase := range testSuit.TestCases {
				logger.Infof("Retrying failing test (%s):", testCase.ClassName+"."+testCase.Name)

				retryArgs := append(args, []string{"--tests", testCase.ClassName + "." + testCase.Name}...)
				retryTestCommand := testTask.GetCommand(filteredVariants, retryArgs...)
				// TODO: retry more than once
				logger.Donef("$ %s", retryTestCommand.PrintableCommandArgs())
				if err := retryTestCommand.Run(); err != nil {
					logger.Errorf("Run: test task failed, error: %v", testErr)
					failingTestCases = append(failingTestCases, testCase)
				} else {
					flakyTestCases = append(flakyTestCases, testCase)
				}
				testRetried = true
			}
		}
	}

	if config.TestResultDir != "" {
		// Test Addon is turned on
		fmt.Println()
		logger.Infof("Export XML results for test addon:")
		fmt.Println()

		xmlResultFilePattern := config.XMLResultDirPattern
		if !strings.HasSuffix(xmlResultFilePattern, "*.xml") {
			xmlResultFilePattern += "*.xml"
		}

		if len(resultXMLs) > 0 {
			lastOtherDirIdx := -1
			for _, artifact := range resultXMLs {
				lastOtherDirIdx = tryExportTestAddonArtifact(artifact.Path, config.TestResultDir, lastOtherDirIdx)
			}
		}
	}

	if len(flakyTestCases) > 0 {
		logger.Printf("")
		logger.Warnf("Detected %d flaky tests:", len(flakyTestCases))
		// TODO: limit the number of flaky tests to log
		for _, testCase := range flakyTestCases {
			logger.Warnf("- %s.%s", testCase.ClassName, testCase.Name)
		}
	}

	if len(failingTestCases) > 0 {
		logger.Printf("")
		logger.Warnf("Detected %d failing tests:", len(failingTestCases))
		for _, testCase := range failingTestCases {
			logger.Warnf("- %s.%s", testCase.ClassName, testCase.Name)
		}
	}

	if testRetried {
		if len(failingTestCases) > 0 {
			logger.Errorf("%d tests failed after retrying, please check the logs for details.", len(failingTestCases))
			os.Exit(1)
		} else {
			logger.Donef("All tests passed after retrying flaky tests.")
		}
	} else {
		if testErr != nil {
			logger.Errorf("Test task failed, error: %v", testErr)
			os.Exit(1)
		} else {
			logger.Donef("All tests passed.")
		}
	}
}

func detectFailingTests(artifacts []gradle.Artifact) ([]junit.TestSuite, error) {
	var reportFiles []string
	for _, artifacts := range artifacts {
		if strings.HasSuffix(artifacts.Name, ".xml") {
			reportFiles = append(reportFiles, artifacts.Path)
		}
	}
	if len(reportFiles) == 0 {
		return nil, nil
	}

	converter := junitxml.Converter{}
	if !converter.Detect(reportFiles) {
		return nil, nil
	}

	// TODO: Does Debug and Release variant produce identical XML files?
	junitXML, err := converter.XML()
	if err != nil {
		return nil, err
	}

	var failingTestSuits []junit.TestSuite
	for _, testSuit := range junitXML.TestSuites {
		var failingTestCases []junit.TestCase
		for _, testCase := range testSuit.TestCases {
			// TODO: align on criteria for a failing test
			if testCase.Failure != nil {
				failingTestCases = append(failingTestCases, testCase)
			}
		}
		if len(failingTestCases) > 0 {
			failingTestSuit := junit.TestSuite{
				XMLName:   testSuit.XMLName,
				Name:      testSuit.Name,
				Tests:     testSuit.Tests,
				Failures:  testSuit.Failures,
				Skipped:   testSuit.Skipped,
				Errors:    testSuit.Errors,
				Time:      testSuit.Time,
				TestCases: failingTestCases,
			}
			failingTestSuits = append(failingTestSuits, failingTestSuit)
		}
	}

	return failingTestSuits, nil
}
