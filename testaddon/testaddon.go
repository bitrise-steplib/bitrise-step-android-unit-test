package testaddon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/log"
)

const (
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

// ExportArtifacts exports the artifacts in a directory structure rooted at the
// specified directory. The directory where each artifact is exported depends
// on which module and build variant produced it.
func ExportArtifacts(path, name, baseDir string) error {
		log.Debugf("processing artifact: %s", path)
		module, err := getModule(path)
		if err != nil {
			return fmt.Errorf("skipping artifact (%s): %s", path, err)
		}

		variant, err := extractVariant(path)
		if err != nil {
			return fmt.Errorf("skipping artifact (%s): could not extract variant name: %s", path, err)
		}

		log.Debugf("artifact (%s) produced by %s variant", path, variant)
		uniqueDir := module + "-" + variant
		exportDir := strings.Join([]string{baseDir, uniqueDir}, "/")

		if err := os.MkdirAll(exportDir, os.ModePerm); err != nil {
			return fmt.Errorf("skipping artifact (%s): could not ensure unique export dir (%s): %s", path, exportDir, err)
		}

		if _, err := os.Stat(filepath.Join(exportDir, ResultDescriptorFileName)); os.IsNotExist(err) {
			m := map[string]string{"test-name": uniqueDir}
			data, err := json.Marshal(m)
			if err != nil {
				return fmt.Errorf("create test info descriptor: json marshal data (%s): %s", m, err)
			}
			if err := generateTestInfoFile(exportDir, data); err != nil {
				return fmt.Errorf("create test info descriptor: generate file: %s", err)
			}
		}

		if err := command.CopyFile(path, filepath.Join(exportDir, name)); err != nil {
			return fmt.Errorf("failed to export artifact (%s), error: %v", name, err)
		}
	return nil
}
