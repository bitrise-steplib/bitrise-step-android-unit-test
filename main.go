package main

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/bitrise-io/go-android/v2/gradle"
	"github.com/bitrise-io/go-steputils/v2/stepconf"
	"github.com/bitrise-io/go-steputils/v2/testquarantine"
	"github.com/bitrise-io/go-utils/v2/command"
	"github.com/bitrise-io/go-utils/v2/env"
	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-utils/v2/pathutil"
	"github.com/bitrise-steplib/bitrise-step-android-unit-test/gradleconfig"
	"github.com/bitrise-steplib/bitrise-step-android-unit-test/output"
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
	// Defaults
	DeployDir     string `env:"BITRISE_DEPLOY_DIR"`
	TestResultDir string `env:"BITRISE_TEST_RESULT_DIR"`
}

func main() {
	var config Configs

	logger := log.NewLogger()
	envRepository := env.NewRepository()
	cmdFactory := command.NewFactory(envRepository)
	pathChecker := pathutil.NewPathChecker()
	inputParser := stepconf.NewInputParser(envRepository)
	exporter := output.NewExporter(envRepository, pathChecker, logger)

	if err := inputParser.Parse(&config); err != nil {
		failf(logger, "Process config: couldn't create step config: %v\n", err)
	}

	stepconf.Print(config)

	logger.EnableDebugLog(config.IsDebug)

	logger.Println()

	gradleProject, err := gradle.NewProject(config.ProjectLocation, cmdFactory)
	if err != nil {
		failf(logger, "Process config: failed to open project, error: %s", err)
	}

	testTask := gradleProject.GetTask("test")

	args, err := shellquote.Split(config.Arguments)
	if err != nil {
		failf(logger, "Process config: failed to parse arguments, error: %s", err)
	}

	logger.Infof("Variants:")
	fmt.Println()

	variants, err := testTask.GetVariants(args...)
	if err != nil {
		failf(logger, "Run: failed to fetch variants, error: %s", err)
	}

	filteredVariants, err := filterVariants(config.Module, config.Variant, variants)
	if err != nil {
		failf(logger, "Run: failed to find buildable variants, error: %s", err)
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
	fmt.Println()

	testIdentifiers, err := parseQuarantinedTests(config.QuarantinedTests)
	if err != nil {
		failf(logger, "Run: failed to parse quarantined tests, error: %s", err)
	}

	if len(testIdentifiers) > 0 {
		logger.Infof("Skipping %d test(s)", len(testIdentifiers))

		if initScriptPth, err := gradleconfig.WriteSkipTestingInitScript(testIdentifiers); err != nil {
			failf(logger, "Run: failed to write init script: %s", err)
		} else {
			defer func() {
				logger.Printf("Removing skip testing init script: %s", initScriptPth)
				if err := os.RemoveAll(initScriptPth); err != nil {
					logger.Warnf("Run: failed to remove skip testing init script (%s): %s", initScriptPth, err)
				}
			}()
		}
	}

	started := time.Now()

	var testErr error

	logger.Infof("Run test:")
	testCommand := testTask.GetCommand(filteredVariants, args...)

	fmt.Println()
	logger.Donef("$ " + testCommand.PrintableCommandArgs())
	fmt.Println()

	testErr = testCommand.Run()
	if testErr != nil {
		logger.Errorf("Run: test task failed, error: %v", testErr)
	}
	fmt.Println()
	logger.Infof("Export HTML results:")
	fmt.Println()

	reports, err := getArtifacts(gradleProject, started, config.HTMLResultDirPattern, true, true, logger)
	if err != nil {
		failf(logger, "Export outputs: failed to find reports, error: %v", err)
	}

	if err := exporter.ExportArtifacts(config.DeployDir, reports); err != nil {
		failf(logger, "Export outputs: failed to export reports, error: %v", err)
	}

	fmt.Println()
	logger.Infof("Export XML results:")
	fmt.Println()

	// <project_dir>/app/build/test-results
	results, err := getArtifacts(gradleProject, started, config.XMLResultDirPattern, true, true, logger)
	if err != nil {
		failf(logger, "Export outputs: failed to find results, error: %v", err)
	}

	if err := exporter.ExportArtifacts(config.DeployDir, results); err != nil {
		failf(logger, "Export outputs: failed to export results, error: %v", err)
	}

	if config.TestResultDir != "" {
		// Test Addon is turned on
		fmt.Println()
		logger.Infof("Export XML results for test addon:")

		xmlResultFilePattern := config.XMLResultDirPattern
		if !strings.HasSuffix(xmlResultFilePattern, "*.xml") {
			xmlResultFilePattern += "*.xml"
		}

		// - <project_dir>/app/build/test-results/testDebugUnitTest/TEST-io.bitrise.kotlinresponsiveviewsactivity.UniTest.xml
		// - <project_dir>/app/build/test-results/testReleaseUnitTest/TEST-io.bitrise.kotlinresponsiveviewsactivity.UniTest.xml
		resultXMLs, err := getArtifacts(gradleProject, started, xmlResultFilePattern, false, false, logger)
		if err != nil {
			logger.Warnf("Failed to find test XML test results, error: %s", err)
		} else {
			exportedResultXMLs, err := exporter.ExportTestAddonArtifacts(config.TestResultDir, resultXMLs)
			if err != nil {
				logger.Warnf("Failed to export test XML test results, error: %s", err)
			}

			if err := exporter.ExportFlakyTestsEnvVar(exportedResultXMLs); err != nil {
				logger.Warnf("Failed to export flaky tests env var, error: %s", err)
			}
		}
	}

	if testErr != nil {
		os.Exit(1)
	}
}

func failf(logger log.Logger, f string, args ...interface{}) {
	logger.Errorf(f, args...)
	os.Exit(1)
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
				fmt.Println()
				continue
			}
			logger.Warnf("No artifacts found with pattern: %s without modtime check", pattern)
			logger.Warnf("If you have changed default report export path in your gradle files then you might need to change ReportPathPattern accordingly.")
		}
	}
	return
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
		if len(qt.TestSuiteName) == 0 || qt.TestSuiteName[0] == "" || qt.ClassName == "" || qt.TestCaseName == "" {
			continue
		}

		packageName := qt.TestSuiteName[0]
		className := qt.ClassName
		testMethod := qt.TestCaseName

		skippedTests = append(skippedTests, fmt.Sprintf("%s.%s.%s", packageName, className, testMethod))
	}
	return skippedTests, nil
}
