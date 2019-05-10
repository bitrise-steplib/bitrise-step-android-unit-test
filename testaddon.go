package main

import (
	"fmt"
	"strings"
	"unicode"
)

// getUniqueDir returns the unique subdirectory inside the test addon export diroctory for a given artifact.
func getUniqueDir(path string) (string, error) {
	parts := strings.Split(path, "/")
	i := len(parts) - 1
	for i > 0 && parts[i] != "test-results" {
		i--
	}

	if i == 0 {
		return "", fmt.Errorf("path (%s) does not contain 'test-results' folder", path)
	}

	variant := parts[i+1]
	variant = strings.TrimPrefix(variant, "test")
	variant = strings.TrimSuffix(variant, "UnitTest")

	runes := []rune(variant)
	runes[0] = unicode.ToLower(runes[0])
	variant = string(runes)

	module := parts[i-2]

	return module + "-" + variant, nil
}
