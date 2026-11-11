package main

import (
	"os"
	"path/filepath"
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

func TestProcessTestSelectionIgnoresInstrumentedUnits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test-selection.json")
	write := func(json string) {
		if err := os.WriteFile(path, []byte(json), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write(`{"version": 1, "mode": "selected", "tests": [
		{"id": "gradle::app:androidTest", "ecosystem": "gradle", "kind": "ui", "selected": true, "identifiers": {"gradle_module": ":app", "gradle_task": ":app:connectedAndroidTest"}},
		{"id": "gradle::app:unitTest", "ecosystem": "gradle", "kind": "unit", "selected": false, "identifiers": {"gradle_module": ":app", "gradle_task": ":app:testDebugUnitTest"}}
	]}`)
	res, err := processTestSelection(path, "skip", log.NewLogger())
	if err != nil {
		t.Fatal(err)
	}
	if !res.skip {
		t.Errorf("only an instrumented unit is selected, so the unit test run should be skipped: %+v", res)
	}

	write(`{"version": 1, "mode": "selected", "tests": [
		{"id": "gradle::app:androidTest", "ecosystem": "gradle", "kind": "ui", "selected": true, "identifiers": {"gradle_module": ":app", "gradle_task": ":app:connectedAndroidTest"}},
		{"id": "gradle::core:unitTest", "ecosystem": "gradle", "kind": "unit", "selected": true, "identifiers": {"gradle_module": ":core", "gradle_task": ":core:testDebugUnitTest"}}
	]}`)
	res, err = processTestSelection(path, "skip", log.NewLogger())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(res.modules, map[string]string{"core": ":core:testDebugUnitTest"}) {
		t.Errorf("modules = %v", res.modules)
	}
}
