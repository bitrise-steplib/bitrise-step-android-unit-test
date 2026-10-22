package main

import (
	"reflect"
	"testing"

	"github.com/bitrise-io/go-android/v2/gradle"
	"github.com/bitrise-io/go-utils/v2/log"
)

func TestApplyTestSelectionKeepsSelectedVariantsAndRunsJVMModulesByTask(t *testing.T) {
	all := gradle.Variants{
		"app":    {"DebugUnitTest", "ReleaseUnitTest"},
		"core":   {"DebugUnitTest", "ReleaseUnitTest"},
		"domain": {"Classes"},
	}
	filtered := gradle.Variants{"app": {"DebugUnitTest"}, "core": {"DebugUnitTest"}}
	sel := testSelectionResult{modules: map[string]string{"app": ":app:test", "domain": ":domain:test"}}

	kept, extra := applyTestSelection(filtered, all, sel, log.NewLogger())

	if !reflect.DeepEqual(kept, gradle.Variants{"app": {"DebugUnitTest"}}) {
		t.Errorf("kept = %v", kept)
	}
	if !reflect.DeepEqual(extra, []string{":domain:test"}) {
		t.Errorf("extra = %v", extra)
	}
}
