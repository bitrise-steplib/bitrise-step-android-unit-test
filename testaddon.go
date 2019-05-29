package main

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/bitrise-io/go-utils/log"
)

// getUniqueDir returns the unique subdirectory inside the test addon export diroctory for a given artifact.
func getUniqueDir(path string) (string, error) {
	log.Debugf("getUniqueDir(%s)", path)
	parts := strings.Split(path, "/")
	i := len(parts) - 1
	for i > 0 && parts[i] != "test-results" {
		i--
	}

	if i == 0 {
		return "", fmt.Errorf("path (%s) does not contain 'test-results' folder", path)
	}

	if i+1 > len(parts) {
		return "", fmt.Errorf("get variant name: out of index error")
	}

	variant := parts[i+1]
	variant = strings.TrimPrefix(variant, "test")
	variant = strings.TrimSuffix(variant, "UnitTest")

	runes := []rune(variant)

	if len(runes) == 0 {
		return "", fmt.Errorf("get variant name from task name: empty string after trimming")
	}
	runes[0] = unicode.ToLower(runes[0])
	variant = string(runes)

	if i < 2 {
		return "", fmt.Errorf("get module name: out of index error")
	}
	module := parts[i-2]
	ret := module + "-" + variant

	log.Debugf("getUniqueDir(%s): (%s,%v)", path, ret, nil)
	return ret, nil
}
