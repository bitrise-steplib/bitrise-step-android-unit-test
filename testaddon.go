package main

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/bitrise-io/go-utils/log"
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

func getUniqueDir(path, baseDir string) (string, error) {
	log.Debugf("processing artifact: %s", path)
	module, err := getModule(path)
	if err != nil {
		return "", fmt.Errorf("skipping artifact (%s): %s", path, err)
	}

	variant, err := extractVariant(path)
	if err != nil {
		return "", fmt.Errorf("skipping artifact (%s): could not extract variant name: %s", path, err)
	}

	log.Debugf("artifact (%s) produced by %s variant", path, variant)
	return module + "-" + variant, nil
}
