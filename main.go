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
	"github.com/bitrise-tools/go-android/gradle"
	"github.com/bitrise-tools/go-steputils/stepconf"
	shellquote "github.com/kballard/go-shellquote"
)

// Configs ...
type Configs struct {
	ProjectLocation   string `env:"project_location,required"`
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

	testTask := gradleProject.
		GetModule(config.Module).
		GetTask("test")

	log.Infof("Variants:")
	fmt.Println()

	variants, err := testTask.GetVariants()
	if err != nil {
		failf("Failed to fetch variants, error: %s", err)
	}

	filteredVariants := variants.Filter(config.Variant)
	var cleanedVariants gradle.Variants
	if config.Module != "" {
		cleanedVariants = filteredVariants
		for _, variant := range variants {
			if sliceutil.IsStringInSlice(variant, filteredVariants) {
				log.Donef("✓ %s", strings.TrimSuffix(variant, "UnitTest"))
			} else {
				log.Printf("- %s", strings.TrimSuffix(variant, "UnitTest"))
			}
		}
	} else {
		moduleVariants := map[string][]string{}
		for _, variant := range variants {
			split := strings.Split(variant, ":")
			if len(split) > 1 {
				moduleVariants[split[0]] = append(moduleVariants[split[0]], split[1])
			}
		}

		for module, variants := range moduleVariants {
			log.Printf("%s:", module)
			for _, variant := range variants {
				if sliceutil.IsStringInSlice(module+":"+variant, filteredVariants) {
					cleanedVariants = append(cleanedVariants, variant)
					log.Donef("✓ %s", strings.TrimSuffix(variant, "UnitTest"))
				} else {
					log.Printf("- %s", strings.TrimSuffix(variant, "UnitTest"))
				}
			}
		}
	}

	fmt.Println()

	if len(cleanedVariants) == 0 {
		errMsg := fmt.Sprintf("No variant matching for: (%s)", config.Variant)
		if config.Module != "" {
			errMsg += fmt.Sprintf(" in module: [%s]", config.Module)
		}
		failf(errMsg)
	}

	if config.Variant == "" {
		log.Warnf("No variant specified, test will run on all variants")
		fmt.Println()
	}

	started := time.Now()

	args, err := shellquote.Split(config.Arguments)
	if err != nil {
		failf("Failed to parse arguments, error: %s", err)
	}

	log.Infof("Run test:")
	testErr := testTask.Run(cleanedVariants, args...)
	if testErr != nil {
		log.Errorf("Test task failed, error: %v", testErr)
	}
	fmt.Println()

	log.Infof("Export reports:")
	fmt.Println()

	reports, err := getArtifacts(gradleProject, started, config.ReportPathPattern)
	if err != nil {
		failf("Failed to find reports, error: %v", err)
	}

	for _, report := range reports {
		report.Name += ".zip"

		exists, err := pathutil.IsPathExists(filepath.Join(deployDir, report.Name))
		if err != nil {
			failf("failed to check path, error: %v", err)
		}

		artifactName := filepath.Base(report.Path)

		if exists {
			timestamp := time.Now().Format("20060102150405")
			ext := filepath.Ext(report.Name)
			name := strings.TrimSuffix(filepath.Base(report.Name), ext)
			report.Name = fmt.Sprintf("%s-%s%s", name, timestamp, ext)
		}

		log.Printf("  Export [ %s => $BITRISE_DEPLOY_DIR/%s ]", artifactName, report.Name)

		if err := report.ExportZIP(deployDir); err != nil {
			log.Warnf("failed to export report (%s), error: %v", report.Path, err)
			continue
		}
	}

	fmt.Println()

	log.Infof("Export results:")
	fmt.Println()

	results, err := getArtifacts(gradleProject, started, config.ResultPathPattern)
	if err != nil {
		failf("Failed to find results, error: %v", err)
	}

	for _, result := range results {
		result.Name += ".zip"

		exists, err := pathutil.IsPathExists(filepath.Join(deployDir, result.Name))
		if err != nil {
			failf("failed to check path, error: %v", err)
		}

		artifactName := filepath.Base(result.Path)

		if exists {
			timestamp := time.Now().Format("20060102150405")
			ext := filepath.Ext(result.Name)
			name := strings.TrimSuffix(filepath.Base(result.Name), ext)
			result.Name = fmt.Sprintf("%s-%s%s", name, timestamp, ext)
		}

		log.Printf("  Export [ %s => $BITRISE_DEPLOY_DIR/%s ]", artifactName, result.Name)

		if err := result.ExportZIP(deployDir); err != nil {
			log.Warnf("failed to export result (%s), error: %v", result.Path, err)
			continue
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

func getArtifacts(gradleProject gradle.Project, started time.Time, pattern string) (artifacts []gradle.Artifact, err error) {
	for _, t := range []time.Time{started, time.Time{}} {
		artifacts, err = gradleProject.FindDirs(t, pattern, true)
		if err != nil {
			return
		}
		if len(artifacts) == 0 {
			if t == started {
				log.Warnf("No reports found with pattern: %s that has modification time after: %s", pattern, t)
				log.Warnf("Retrying without modtime check....")
				fmt.Println()
				continue
			}
			log.Warnf("No reports found with pattern: %s without modtime check", pattern)
			log.Warnf("If you have changed default report export path in your gradle files then you might need to change ReportPathPattern accordingly.")
		}
	}
	return
}
