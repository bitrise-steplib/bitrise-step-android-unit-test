package testaddon

import (
	"fmt"
	"strings"

	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-tools/go-android/gradle"
)

var baseDir string

// getModule deduces the module name from a path like:
// path_to_your_project/module_name/build/test-results/testDebugUnitTest/TEST-com.bitrise_io.sample_apps_android_simple.ExampleUnitTest.xml
func getModule(path string) (string, error) {
	parts := strings.Split(path, "/")
	i := len(parts) - 1
	for i > 0 && parts[i] != "test-results" {
		i--
	}

	if i == 0 {
		return "", fmt.Errorf("could not determine module based on path")
	}

	return parts[i - 2], nil
}

func ExportArtifacts(artifacts []gradle.Artifact) error {
	for _, artifact := range artifacts {
		module, err := getModule(artifact.Path)
		if err != nil {
			log.Warnf("skipping artifact (%s): %s", artifact.Path, err)
			continue
		}

		flavour, buildType := "", ""  // todo: figure out how to get the required data to build the dir name
		uniqueDir := fmt.Sprintf("%d-%s%s", module, flavour, buildType)
		exportDir := strings.Join([]string{baseDir, uniqueDir}, "/")
		if err := artifact.Export(exportDir); err != nil {
			log.Warnf("failed to export artifact (%s), error: %v", artifact.Path, err)
		}
	}
	return nil
}