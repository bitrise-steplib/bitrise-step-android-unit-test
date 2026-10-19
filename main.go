package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/bitrise-io/bitrise-build-cache-cli/v2/pkg/reactnative/wrap"
	"github.com/bitrise-io/go-android/v2/gradle"
	"github.com/bitrise-io/go-steputils/v2/stepconf"
	"github.com/bitrise-io/go-steputils/v2/testquarantine"
	"github.com/bitrise-io/go-utils/v2/command"
	"github.com/bitrise-io/go-utils/v2/env"
	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-utils/v2/pathutil"
	"github.com/bitrise-steplib/bitrise-step-android-unit-test/gradleconfig"
	"github.com/bitrise-steplib/bitrise-step-android-unit-test/output"
	"github.com/bitrise-steplib/bitrise-step-android-unit-test/testselection"
	"github.com/kballard/go-shellquote"
)

// Configs ...
type Configs struct {
	ProjectLocation string `env:"project_location,dir"`
	Module          string `env:"module"`
	Variant         string `env:"variant"`
	// Options
	Arguments            string `env:"arguments"`
	HTMLResultDirPattern string `env:"report_path_pattern"`
	XMLResultDirPattern  string `env:"result_path_pattern"`
	// Debug
	IsDebug          bool   `env:"is_debug,opt[true,false]"`
	QuarantinedTests string `env:"quarantined_tests"`
	TestSelection    string `env:"test_selection"`
	OnEmptySelection string `env:"on_empty_selection"`
	// Defaults
	DeployDir     string `env:"BITRISE_DEPLOY_DIR"`
	TestResultDir string `env:"BITRISE_TEST_RESULT_DIR"`
}

func main() {
	logger := log.NewLogger()

	if err := runStep(logger); err != nil {
		logger.Errorf("%s", err)
		os.Exit(1)
	}
}

func runStep(logger log.Logger) error {
	var config Configs

	envRepository := env.NewRepository()
	cmdFactory := command.NewFactory(envRepository)
	pathChecker := pathutil.NewPathChecker()
	inputParser := stepconf.NewInputParser(envRepository)
	exporter := output.NewExporter(envRepository, pathChecker, logger)

	if err := inputParser.Parse(&config); err != nil {
		return fmt.Errorf("Process config: couldn't create step config: %v\n", err)
	}

	stepconf.Print(config)

	logger.EnableDebugLog(config.IsDebug)

	selection, err := processTestSelection(config.TestSelection, config.OnEmptySelection, logger)
	if err != nil {
		return fmt.Errorf("Process config: %s", err)
	}
	if selection.skip {
		logger.Println()
		logger.Infof("Skipping the test run: %s", selection.reason)
		return nil
	}

	gradleProject, err := gradle.NewProject(config.ProjectLocation, cmdFactory, logger)
	if err != nil {
		return fmt.Errorf("Process config: failed to open project: %s", err)
	}

	testTask := gradleProject.GetTask("test")

	args, err := shellquote.Split(config.Arguments)
	if err != nil {
		return fmt.Errorf("Process config: failed to parse arguments: %s", err)
	}

	logger.Println()
	logger.Infof("Variants:")

	variants, err := testTask.GetVariants(args...)
	if err != nil {
		return fmt.Errorf("Run: failed to fetch variants: %s", err)
	}

	filteredVariants, err := filterVariants(config.Module, config.Variant, variants)
	if err != nil {
		return fmt.Errorf("Run: failed to find buildable variants: %s", err)
	}

	var extraTasks []string
	if selection.modules != nil {
		filteredVariants, extraTasks = applyTestSelection(filteredVariants, variants, selection, logger)
		if len(filteredVariants) == 0 && len(extraTasks) == 0 {
			logger.Println()
			logger.Warnf("None of the selected modules has a unit test task in this project; nothing to run")
			return nil
		}
	}

	for module, variants := range variants {
		logger.Printf("%s:", module)
		for _, variant := range variants {
			if slices.Contains(filteredVariants[module], variant) {
				logger.Donef("✓ %s", strings.TrimSuffix(variant, "UnitTest"))
			} else {
				logger.Printf("- %s", strings.TrimSuffix(variant, "UnitTest"))
			}
		}
	}

	testIdentifiers, err := parseQuarantinedTests(config.QuarantinedTests)
	if err != nil {
		return fmt.Errorf("Run: failed to parse quarantined tests: %s", err)
	}

	var initScriptPth string
	if len(testIdentifiers) > 0 {
		for _, arg := range args {
			if strings.HasPrefix(arg, "--init-script") {
				return fmt.Errorf("Run: --init-script argument cannot be used together with quarantined_tests input")
			} else if strings.HasPrefix(arg, "-I") {
				return fmt.Errorf("Run: -I argument cannot be used together with quarantined_tests input")
			}
		}

		logger.Println()
		logger.Infof("%d quarantined test(s) found", len(testIdentifiers))
		logger.Printf("Writing Gradle init script for excluding quarantined tests...")

		initScriptPth, err = gradleconfig.WriteSkipTestingInitScript(testIdentifiers)
		if err != nil {
			return fmt.Errorf("Run: failed to write quarantine init script: %s", err)
		}

		args = append(args, "--init-script", initScriptPth)

		defer func() {
			logger.Println()
			logger.Printf("Removing test quarantine init script: %s", initScriptPth)
			if err := os.RemoveAll(initScriptPth); err != nil {
				logger.Warnf("Run: failed to remove skip testing init script (%s): %s", initScriptPth, err)
			}
		}()
	}

	started := time.Now()

	var testErr error

	logger.Println()
	logger.Infof("Run test:")

	// Inline of testTask.GetCommand(filteredVariants, args...) so the gradlew
	// path and argv are explicit — needed to route the invocation through
	// `bitrise-build-cache react-native run -- ...` when RN cache is active.
	// Resolve gradlew to an absolute path: the command also sets Dir to the
	// project location, and a relative gradlew name (e.g. "_tmp/gradlew")
	// gets re-resolved relative to that Dir at exec time, producing
	// "_tmp/_tmp/gradlew" and an ENOENT.
	gradlewPath, err := filepath.Abs(filepath.Join(config.ProjectLocation, "gradlew"))
	if err != nil {
		return fmt.Errorf("resolve gradlew path: %v", err)
	}
	var taskNames []string
	for module, variants := range filteredVariants {
		modulePrefix := ""
		if module != "" {
			modulePrefix = ":" + module + ":"
		}
		for _, variant := range variants {
			taskNames = append(taskNames, modulePrefix+"test"+variant)
		}
	}
	taskNames = append(taskNames, extraTasks...)
	gradleArgs := append(taskNames, args...)
	det := wrap.Detect(context.Background(), wrap.DetectParams{Logger: logger})
	if det.ReactNativeEnabled {
		logger.Infof("Bitrise Build Cache: React Native cache active — wrapping gradle with %s", det.CLIPath)
	}
	name, wrappedArgs := wrap.Wrap(det, gradlewPath, gradleArgs)
	testCommand := cmdFactory.Create(name, wrappedArgs, &command.Opts{
		Dir:    config.ProjectLocation,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	})
	logger.Donef("$ " + testCommand.PrintableCommandArgs())

	testErr = testCommand.Run()
	if testErr != nil {
		logger.Errorf("Run: test task failed: %v", testErr)
	} else {
		logger.Donef("Successful test run")
	}

	logger.Println()
	logger.Infof("Export HTML results:")

	reports, err := getArtifacts(gradleProject, started, config.HTMLResultDirPattern, true, true, logger)
	if err != nil {
		return fmt.Errorf("Export outputs: failed to find reports: %v", err)
	}

	if err := exporter.ExportArtifacts(config.DeployDir, reports); err != nil {
		return fmt.Errorf("Export outputs: failed to export reports: %v", err)
	}

	logger.Println()
	logger.Infof("Export XML results:")

	// <project_dir>/app/build/test-results
	results, err := getArtifacts(gradleProject, started, config.XMLResultDirPattern, true, true, logger)
	if err != nil {
		return fmt.Errorf("Export outputs: failed to find results: %v", err)
	}

	if err := exporter.ExportArtifacts(config.DeployDir, results); err != nil {
		return fmt.Errorf("Export outputs: failed to export results: %v", err)
	}

	if config.TestResultDir != "" {
		// Test Addon is turned on
		logger.Println()
		logger.Infof("Export XML results for test addon:")

		xmlResultFilePattern := config.XMLResultDirPattern
		if !strings.HasSuffix(xmlResultFilePattern, "*.xml") {
			xmlResultFilePattern += "*.xml"
		}

		// - <project_dir>/app/build/test-results/testDebugUnitTest/TEST-io.bitrise.kotlinresponsiveviewsactivity.UniTest.xml
		// - <project_dir>/app/build/test-results/testReleaseUnitTest/TEST-io.bitrise.kotlinresponsiveviewsactivity.UniTest.xml
		resultXMLs, err := getArtifacts(gradleProject, started, xmlResultFilePattern, false, false, logger)
		if err != nil {
			logger.Warnf("Failed to find test XML test results: %s", err)
		} else {
			exportedResultXMLs, err := exporter.ExportTestAddonArtifacts(config.TestResultDir, resultXMLs, config.ProjectLocation)
			if err != nil {
				logger.Warnf("Failed to export test XML test results: %s", err)
			}

			if err := exporter.ExportFlakyTestsEnvVar(exportedResultXMLs); err != nil {
				logger.Warnf("Failed to export flaky tests env var: %s", err)
			}
		}
	}

	if testErr != nil {
		return fmt.Errorf("Running tests failed: %w", testErr)
	}

	return nil
}

func getArtifacts(gradleProject gradle.Project, started time.Time, pattern string, includeModuleName bool, isDirectoryMode bool, logger log.Logger) (artifacts []gradle.Artifact, err error) {
	for _, t := range []time.Time{started, {}} {
		if isDirectoryMode {
			artifacts, err = gradleProject.FindDirs(t, pattern, includeModuleName)
		} else {
			artifacts, err = gradleProject.FindArtifacts(t, pattern, includeModuleName)
		}
		if err != nil {
			return
		}
		if len(artifacts) == 0 {
			if t == started {
				logger.Warnf("No artifacts found with pattern: %s that has modification time after: %s", pattern, t)
				logger.Warnf("Retrying without modtime check....")
				logger.Println()
				continue
			}
			logger.Warnf("No artifacts found with pattern: %s without modtime check", pattern)
			logger.Warnf("If you have changed default report export path in your gradle files then you might need to change ReportPathPattern accordingly.")
		}
	}
	return
}

type testSelectionResult struct {
	skip   bool
	reason string
	// modules maps a selected Gradle project path without the leading colon to the task
	// the Test Selection step suggested for it; nil when there is no selection.
	modules map[string]string
}

func processTestSelection(path, onEmpty string, logger log.Logger) (testSelectionResult, error) {
	sel, err := testselection.Load(path)
	if err != nil {
		return testSelectionResult{}, err
	}
	decision, err := testselection.Decide(sel, onEmpty, "gradle")
	if err != nil {
		return testSelectionResult{}, err
	}
	if sel == nil {
		return testSelectionResult{}, nil
	}

	logger.Println()
	logger.Infof("Test selection: %s", decision.Reason)
	switch decision.Action {
	case testselection.Skip:
		return testSelectionResult{skip: true, reason: decision.Reason}, nil
	case testselection.RunSelected:
		modules := map[string]string{}
		for _, t := range decision.Tests {
			module := strings.TrimPrefix(t.Identifiers.GradleModule, ":")
			logger.Printf("- %s (%s)", t.Identifiers.GradleModule, t.Reason)
			modules[module] = t.Identifiers.GradleTask
		}
		return testSelectionResult{modules: modules}, nil
	default:
		return testSelectionResult{}, nil
	}
}

// applyTestSelection keeps the unit test variants of the selected modules. A selected
// module without unit test variants (a plain JVM module) runs the task the selection
// suggested for it instead.
func applyTestSelection(filtered, all gradle.Variants, sel testSelectionResult, logger log.Logger) (gradle.Variants, []string) {
	kept := gradle.Variants{}
	for module, variants := range filtered {
		if _, ok := sel.modules[module]; ok {
			kept[module] = variants
		}
	}
	var extra []string
	for module, task := range sel.modules {
		if _, ok := all[module]; ok || task == "" {
			continue
		}
		logger.Printf("%s: no unit test variant found, running %s", module, task)
		extra = append(extra, task)
	}
	sort.Strings(extra)
	return kept, extra
}

func filterVariants(module, variant string, variantsMap gradle.Variants) (gradle.Variants, error) {
	// if module set: drop all the other modules
	if module != "" {
		v, ok := variantsMap[module]
		if !ok {
			return nil, fmt.Errorf("module not found: %s", module)
		}
		variantsMap = gradle.Variants{module: v}
	}
	// if variant not set: use all variants
	if variant == "" {
		return variantsMap, nil
	}
	filteredVariants := gradle.Variants{}
	for m, variants := range variantsMap {
		for _, v := range variants {
			if strings.EqualFold(v, variant+"UnitTest") {
				filteredVariants[m] = append(filteredVariants[m], v)
			}
		}
	}
	if len(filteredVariants) == 0 {
		return nil, fmt.Errorf("variant %s not found in any module", variant)
	}
	return filteredVariants, nil
}

func parseQuarantinedTests(input string) ([]string, error) {
	if input == "" {
		return nil, nil
	}

	quarantinedTests, err := testquarantine.ParseQuarantinedTests(input)
	if err != nil {
		return nil, fmt.Errorf("failed to parse quarantined tests input: %w", err)
	}

	var skippedTests []string
	for _, qt := range quarantinedTests {
		if qt.ClassName == "" || qt.TestCaseName == "" {
			continue
		}

		packageAndClassName := qt.ClassName
		testMethodName := qt.TestCaseName

		skippedTests = append(skippedTests, fmt.Sprintf("%s.%s", packageAndClassName, testMethodName))
	}
	return skippedTests, nil
}
