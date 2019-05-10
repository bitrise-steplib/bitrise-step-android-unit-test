package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-io/go-utils/sliceutil"
	"github.com/bitrise-steplib/bitrise-step-android-unit-test/cache"
	"github.com/bitrise-steplib/bitrise-step-android-unit-test/testaddon"
	"github.com/bitrise-tools/go-android/gradle"
	"github.com/bitrise-tools/go-steputils/stepconf"
	shellquote "github.com/kballard/go-shellquote"
)

const resultArtifactPathPattern = "*TEST*.xml"

// Configs ...
type Configs struct {
	ProjectLocation   string `env:"project_location,dir"`
	ReportPathPattern string `env:"report_path_pattern"`
	ResultPathPattern string `env:"result_path_pattern"`
	Variant           string `env:"variant"`
	Module            string `env:"module"`
	Arguments         string `env:"arguments"`
	CacheLevel        string `env:"cache_level,opt[none,only_deps,all]"`
}

func failf(f string, args ...interface{}) {
	log.Errorf(f, args...)
	os.Exit(1)
}

func getArtifacts(gradleProject gradle.Project, started time.Time, pattern string, includeModuleName bool, isDirectoryMode bool) (artifacts []gradle.Artifact, err error) {
	for _, t := range []time.Time{started, time.Time{}} {
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
				log.Warnf("No artifacts found with pattern: %s that has modification time after: %s", pattern, t)
				log.Warnf("Retrying without modtime check....")
				fmt.Println()
				continue
			}
			log.Warnf("No artifacts found with pattern: %s without modtime check", pattern)
			log.Warnf("If you have changed default report export path in your gradle files then you might need to change ReportPathPattern accordingly.")
		}
	}
	return
}

func exportArtifacts(deployDir string, artifacts []gradle.Artifact) error {
	for _, artifact := range artifacts {
		artifact.Name += ".zip"
		exists, err := pathutil.IsPathExists(filepath.Join(deployDir, artifact.Name))
		if err != nil {
			return fmt.Errorf("failed to check path, error: %v", err)
		}

		if exists {
			timestamp := time.Now().Format("20060102150405")
			artifact.Name = fmt.Sprintf("%s-%s%s", strings.TrimSuffix(artifact.Name, ".zip"), timestamp, ".zip")
		}

		log.Printf("  Export [ %s => $BITRISE_DEPLOY_DIR/%s ]", filepath.Base(artifact.Path), artifact.Name)

		if err := artifact.ExportZIP(deployDir); err != nil {
			log.Warnf("failed to export artifact (%s), error: %v", artifact.Path, err)
			continue
		}
	}
	return nil
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
			if strings.ToLower(v) == strings.ToLower(variant+"UnitTest") {
				filteredVariants[m] = append(filteredVariants[m], v)
			}
		}
	}
	if len(filteredVariants) == 0 {
		return nil, fmt.Errorf("variant: %s not found in any module", variant)
	}
	return filteredVariants, nil
}

func main() {
	var config Configs

	if err := stepconf.Parse(&config); err != nil {
		failf("Couldn't create step config: %v\n", err)
	}

	stepconf.Print(config)

	deployDir := os.Getenv("BITRISE_DEPLOY_DIR")

	log.Printf("- Deploy dir: %s", deployDir)
	fmt.Println()

	gradleProject, err := gradle.NewProject(config.ProjectLocation)
	if err != nil {
		failf("Failed to open project, error: %s", err)
	}

	testTask := gradleProject.GetTask("test")

	log.Infof("Variants:")
	fmt.Println()

	variants, err := testTask.GetVariants()
	if err != nil {
		failf("Failed to fetch variants, error: %s", err)
	}

	filteredVariants, err := filterVariants(config.Module, config.Variant, variants)
	if err != nil {
		failf("Failed to find buildable variants, error: %s", err)
	}

	for module, variants := range variants {
		log.Printf("%s:", module)
		for _, variant := range variants {
			if sliceutil.IsStringInSlice(variant, filteredVariants[module]) {
				log.Donef("✓ %s", strings.TrimSuffix(variant, "UnitTest"))
			} else {
				log.Printf("- %s", strings.TrimSuffix(variant, "UnitTest"))
			}
		}
	}
	fmt.Println()

	started := time.Now()

	args, err := shellquote.Split(config.Arguments)
	if err != nil {
		failf("Failed to parse arguments, error: %s", err)
	}

	var testErr error

	log.Infof("Run test:")
	testCommand := testTask.GetCommand(filteredVariants, args...)

	fmt.Println()
	log.Donef("$ " + testCommand.PrintableCommandArgs())
	fmt.Println()

	testErr = testCommand.Run()
	if testErr != nil {
		log.Errorf("Test task failed, error: %v", testErr)
	}
	fmt.Println()

	log.Infof("Export reports:")
	fmt.Println()

	reports, err := getArtifacts(gradleProject, started, config.ReportPathPattern, true, true)
	if err != nil {
		failf("Failed to find reports, error: %v", err)
	}

	if err := exportArtifacts(deployDir, reports); err != nil {
		failf("Failed to export reports, error: %v", err)
	}

	fmt.Println()

	log.Infof("Export results:")
	fmt.Println()

	results, err := getArtifacts(gradleProject, started, config.ResultPathPattern, true, true)
	if err != nil {
		failf("Failed to find results, error: %v", err)
	}

	if err := exportArtifacts(deployDir, results); err != nil {
		failf("Failed to export results, error: %v", err)
	}

	log.Infof("Export test results for test addon:")
	fmt.Println()

	resultXMLs, err := getArtifacts(gradleProject, started, resultArtifactPathPattern, false, false)
	if err != nil {
		log.Warnf("Failed to find test result XMLs, error: %s", err)
	} else {
		if baseDir := os.Getenv("BITRISE_TEST_RESULT_DIR"); baseDir != "" {
			for _, artifact := range resultXMLs {
				uniqueDir, err := getUniqueDir(artifact.Path)
				if err != nil {
					log.Warnf("Failed to export test results for test addon: cannot get export directory for artifact (%s): %s", err)
					continue
				}
	
				if err := testaddon.ExportArtifact(artifact.Path, baseDir, uniqueDir); err != nil {
					log.Warnf("Failed to export test results for test addon: %s", err)
				}
			}
			log.Printf("  Exporting test results to test addon successful [ %s ] ", baseDir)
		}
	}


	if testErr != nil {
		os.Exit(1)
	}

	fmt.Println()
	log.Infof("Collecting cache:")
	if warning := cache.Collect(config.ProjectLocation, cache.Level(config.CacheLevel)); warning != nil {
		log.Warnf("%s", warning)
	}
	log.Donef("  Done")
}