package main

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/bitrise-io/go-android/v2/gradle"
	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-steplib/bitrise-step-test-selection/selector"
)

// startTestSelectionShadow works out which modules a change-based test selection would
// have run. It never changes what the step runs, and it only acts when the backend
// injects a shadow configuration; otherwise it returns nil. The Gradle analysis is the
// expensive part, so the backend can sample builds and bound it with a timeout.
func startTestSelectionShadow(config Configs, filteredVariants gradle.Variants, logger log.Logger) *selector.Shadow {
	sh, ok := selector.StartShadow(selector.ShadowOptions{
		Step: "android-unit-test",
		Config: selector.Config{
			ProjectType:      "gradle",
			GradleProjectDir: config.ProjectLocation,
		},
		WorkDir: config.ProjectLocation,
		Keep:    runsModule(filteredVariants),
		Logf: func(format string, v ...interface{}) {
			logger.Println()
			logger.Infof(format, v...)
		},
		Debugf: logger.Debugf,
	})
	if !ok {
		return nil
	}
	return sh
}

// finishTestSelectionShadow reports each module's test results so the backend can tell
// what a selection would have saved and whether it would have skipped a failure.
func finishTestSelectionShadow(sh *selector.Shadow, config Configs, filteredVariants gradle.Variants, testRun time.Duration, logger log.Logger) {
	if sh == nil {
		return
	}
	sh.Finish(moduleOutcomes(config.ProjectLocation, filteredVariants, logger), testRun)
}

// runsModule keeps the modules the step runs unit tests in. The variant map is keyed by
// the Gradle project path without its leading colon, the root project being "".
func runsModule(filteredVariants gradle.Variants) func(selector.Test, string) bool {
	return func(t selector.Test, _ string) bool {
		_, ok := filteredVariants[strings.TrimPrefix(t.Ident.GradleModule, ":")]
		return ok
	}
}

// moduleOutcomes reads the JUnit reports of the variants the step ran. A module's
// duration is its test cases' time, so compilation is not counted and the saving is a
// lower bound. It assumes the default project layout, where :feature:home lives in
// feature/home.
func moduleOutcomes(projectLocation string, filteredVariants gradle.Variants, logger log.Logger) []selector.UnitOutcome {
	var outcomes []selector.UnitOutcome
	for module, variants := range filteredVariants {
		var files []string
		for _, variant := range variants {
			dir := filepath.Join(projectLocation, filepath.FromSlash(strings.ReplaceAll(module, ":", "/")), "build", "test-results", "test"+variant)
			matches, _ := filepath.Glob(filepath.Join(dir, "*.xml"))
			files = append(files, matches...)
		}
		if len(files) == 0 {
			continue
		}
		outcome, err := selector.JUnitOutcome(":"+module, files)
		if err != nil {
			logger.Debugf("Test selection shadow run: cannot read the test results of %s: %s", module, err)
			return nil
		}
		outcomes = append(outcomes, outcome)
	}
	return outcomes
}
