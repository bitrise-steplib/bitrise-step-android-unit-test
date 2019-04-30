package testaddon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-tools/go-android/gradle"
)

const (
	// ResultArtifactPathPattern is a glob pattern matching test result XMLs.
	ResultArtifactPathPattern = "*TEST*.xml"
	// ResultDescriptorFileName is the name of the test result descriptor file.
	ResultDescriptorFileName  = "test-info.json"
)

// getModule deduces the module name from a path like:
// path example: <PATH_TO_YOUR_PROJECT>/<MODULE_NAME>/build/test-results/testDemoDebugUnitTest/TEST-example.com.helloworld.ExampleUnitTest.xml
func getModule(path string) (string, error) {
	parts := strings.Split(path, "/")
	i := len(parts) - 1
	for i > 0 && parts[i] != "test-results" {
		i--
	}

	if i == 0 {
		return "", fmt.Errorf("could not determine module based on path")
	}

	return parts[i-2], nil
}

func extractVariant(path string) (string, error) {
	// path example: <PATH_TO_YOUR_PROJECT>/<MODULE_NAME>/build/test-results/testDemoDebugUnitTest/TEST-example.com.helloworld.ExampleUnitTest.xml
	parts := strings.Split(path, "/")
	i := len(parts) - 1
	for i > 0 && parts[i] != "test-results" {
		i--
	}

	if i == 0 {
		return "", fmt.Errorf("could not determine variant based on path")
	}

	variant := parts[i+1]
	variant = strings.TrimPrefix(variant, "test")
	variant = strings.TrimSuffix(variant, "UnitTest")

	runes := []rune(variant)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes), nil
}

func generateTestInfoFile(dir string, data []byte) error {
	f, err := os.Create(filepath.Join(dir, ResultDescriptorFileName))
	if err != nil {
		return err
	}

	if _, err := f.Write(data); err != nil {
		return err
	}

	if err := f.Sync(); err != nil {
		return err
	}

	if err := f.Close(); err != nil {
		return err
	}

	return nil
}

// GetArtifacts retrieves the test output artifacts produced by the gradle
// task(s) ran
func GetArtifacts(gradleProject gradle.Project, started time.Time, pattern string) (artifacts []gradle.Artifact, err error) {
	for _, t := range []time.Time{started, time.Time{}} {
		artifacts, err = gradleProject.FindArtifacts(t, pattern, false)
		if err != nil {
			return
		}
		if len(artifacts) == 0 {
			if t == started {
				log.Warnf("No artifacts found with pattern: %s that has modification time after: %s", pattern, t)
				log.Warnf("Retrying without modtime check....")
				fmt.Println()
				continue
			}
			log.Warnf("No artifacts found with pattern: %s without modtime check", pattern)
			log.Warnf("If you have changed default report export path in your gradle files then you might need to change ReportPathPattern accordingly.")
		}
	}
	return
}

// ExportArtifacts exports the artifacts in a directory structure rooted at the
// specified directory. The directory where each artifact is exported depends
// on which module and build variant produced it.
func ExportArtifacts(artifacts []gradle.Artifact, baseDir string) error {
	for _, artifact := range artifacts {
		log.Debugf("processing artifact: %s", artifact.Path)
		module, err := getModule(artifact.Path)
		if err != nil {
			log.Warnf("skipping artifact (%s): %s", artifact.Path, err)
			continue
		}

		variant, err := extractVariant(artifact.Path)
		if err != nil {
			log.Warnf("skipping artifact (%s): could not extract variant name: %s", artifact.Path, err)
			continue
		}

		log.Debugf("artifact (%s) produced by %s variant", artifact.Path, variant)
		uniqueDir := module + "-" + variant
		exportDir := strings.Join([]string{baseDir, uniqueDir}, "/")

		if err := os.MkdirAll(exportDir, os.ModePerm); err != nil {
			log.Warnf("skipping artifact (%s): could not ensure unique export dir (%s): %s", artifact.Path, exportDir, err)
		}

		if _, err := os.Stat(filepath.Join(exportDir, ResultDescriptorFileName)); os.IsNotExist(err) {
			m := map[string]string{"test-name": uniqueDir}
			data, err := json.Marshal(m)
			if err != nil {
				log.Warnf("create test info descriptor: json marshal data (%s): %s", m, err)
				continue
			}
			if err := generateTestInfoFile(exportDir, data); err != nil {
				log.Warnf("create test info descriptor: generate file: %s", err)
			}
		}

		if err := artifact.Export(exportDir); err != nil {
			log.Warnf("failed to export artifact (%s), error: %v", artifact.Name, err)
		}
	}
	return nil
}
