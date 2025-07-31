package testaddon

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
)

// OtherDirName is a directory name of non Android Unit test results
const OtherDirName = "other"

func getExportDir(artifactPath string) string {
	modules, variant, err := getModuleAndVariant(artifactPath)
	if err != nil {
		return OtherDirName
	}

	return modules + "-" + variant
}

func lowercaseFirstLetter(str string) string {
	for i, v := range str {
		return string(unicode.ToLower(v)) + str[i+1:]
	}
	return ""
}

func parseVariantName(pthParts []string, testResultsPartIdx int) (string, error) {
	// example: ./app/build/test-results/testDebugUnitTest/TEST-sample.results.test.multiple.bitrise.com.multipletestresultssample.UnitTest0.xml
	if testResultsPartIdx+1 > len(pthParts) {
		return "", fmt.Errorf("unknown path (%s): Local Unit Test task output dir should follow the test-results part", filepath.Join(pthParts...))
	}

	taskOutputDir := pthParts[testResultsPartIdx+1]
	if !strings.HasPrefix(taskOutputDir, "test") || !strings.HasSuffix(taskOutputDir, "UnitTest") {
		return "", fmt.Errorf("unknown path (%s): Local Unit Test task output dir should match test*UnitTest pattern", filepath.Join(pthParts...))
	}

	variant := strings.TrimPrefix(taskOutputDir, "test")
	variant = strings.TrimSuffix(variant, "UnitTest")

	if len(variant) == 0 {
		return "", fmt.Errorf("unknown path (%s): Local Unit Test task output dir should match test<Variant>UnitTest pattern", filepath.Join(pthParts...))
	}

	return lowercaseFirstLetter(variant), nil
}

func parseModuleName(pthParts []string, testResultsPartIdx int) (string, error) {
	if testResultsPartIdx < 2 {
		return "", fmt.Errorf(`unknown path (%s): Local Unit Test task output dir should match <moduleName>/build/test-results`, filepath.Join(pthParts...))
	}
	return pthParts[testResultsPartIdx-2], nil
}

// getVariantDir parses model and variant from the given artifact path.
func getModuleAndVariant(path string) (string, string, error) {
	parts := strings.Split(path, "/")

	i := indexOfTestResultsDirName(parts)
	if i == -1 {
		return "", "", fmt.Errorf("path (%s) does not contain 'test-results' folder", path)
	}

	variant, err := parseVariantName(parts, i)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse variant name: %s", err)
	}

	module, err := parseModuleName(parts, i)
	if err != nil {
		return "", variant, fmt.Errorf("failed to parse module name: %s", err)
	}

	return module, variant, nil
}

// indexOfTestResultsDirName finds the index of "test-results" in the given path parts, othervise returns -1
func indexOfTestResultsDirName(pthParts []string) int {
	// example: ./app/build/test-results/testDebugUnitTest/TEST-sample.results.test.multiple.bitrise.com.multipletestresultssample.UnitTest0.xml
	for i, part := range pthParts {
		if part == "test-results" {
			return i
		}
	}
	return -1
}
