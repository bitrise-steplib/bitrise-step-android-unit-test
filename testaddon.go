package main

import (
	"fmt"
	"strings"
	"unicode"
)

// getModule returns the name of the module a given test result artifact was produced by
// based of the artifacts path.
// path example: <PATH_TO_YOUR_PROJECT>/<MODULE_NAME>/build/test-results/testDemoDebugUnitTest/TEST-example.com.helloworld.ExampleUnitTest.xml
func getModule(path string) (string, error) {
	parts := strings.Split(path, "/")
	i := len(parts) - 1
	for i > 0 && parts[i] != "test-results" {
		i--
	}

	if i == 0 {
		return "", fmt.Errorf("path (%s) does not contain 'test-results' folder")
	}

	return parts[i-2], nil
}

// getVariant returns the name of the build variant a given test result artifact was produced by
// based of the artifacts path.
// path example: <PATH_TO_YOUR_PROJECT>/<MODULE_NAME>/build/test-results/testDemoDebugUnitTest/TEST-example.com.helloworld.ExampleUnitTest.xml
func getVariant(path string) (string, error) {
	parts := strings.Split(path, "/")
	i := len(parts) - 1
	for i > 0 && parts[i] != "test-results" {
		i--
	}

	if i == 0 {
		return "", fmt.Errorf("path (%s) does not contain 'test-results' folder")
	}

	variant := parts[i+1]
	variant = strings.TrimPrefix(variant, "test")
	variant = strings.TrimSuffix(variant, "UnitTest")

	runes := []rune(variant)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes), nil
}

// getUniqueDir returns the unique subdirectory inside the test addon export diroctory for a given artifact.
func getUniqueDir(path string) (string, error) {
	module, err := getModule(path)
	if err != nil {
		return "", fmt.Errorf("get module from path (%s): %s", path, err)
	}

	variant, err := getVariant(path)
	if err != nil {
		return "", fmt.Errorf("get variant from path (%s): %s", path, err)
	}

	return module + "-" + variant, nil
}
