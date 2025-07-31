package output

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bitrise-io/go-android/v2/gradle"
	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-utils/v2/pathutil"
	"github.com/bitrise-steplib/bitrise-step-android-unit-test/testaddon"
)

type Exporter interface {
	ExportArtifacts(deployDir string, artifacts []gradle.Artifact) error
	ExportTestAddonArtifacts(testDeployDir string, artifacts []gradle.Artifact)
}

type exporter struct {
	pathChecker     pathutil.PathChecker
	logger          log.Logger
	lastOtherDirIdx int
}

func NewExporter(pathChecker pathutil.PathChecker, logger log.Logger) Exporter {
	return &exporter{
		pathChecker: pathChecker,
		logger:      logger,
	}
}

func (e exporter) ExportArtifacts(deployDir string, artifacts []gradle.Artifact) error {
	for _, artifact := range artifacts {
		artifact.Name += ".zip"
		exists, err := e.pathChecker.IsPathExists(filepath.Join(deployDir, artifact.Name))
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

		e.logger.Printf("  Export [ %s => $BITRISE_DEPLOY_DIR/%s ]", src, artifact.Name)

		if err := artifact.ExportZIP(deployDir); err != nil {
			e.logger.Warnf("failed to export artifact (%s), error: %v", artifact.Path, err)
			continue
		}
	}
	return nil
}

func (e exporter) ExportTestAddonArtifacts(testDeployDir string, artifacts []gradle.Artifact) {
	lastOtherDirIdx := -1
	for _, artifact := range artifacts {
		var err error
		lastOtherDirIdx, err = testaddon.ExportTestAddonArtifact(artifact.Path, testDeployDir, lastOtherDirIdx, e.logger)
		if err != nil {
			e.logger.Warnf("Failed to export test results for test addon: %s", err)
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
