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

func exportArtifacts(deployDir string, artifacts []gradle.Artifact) error {
	for _, artifact := range artifacts {
		artifact.Name += ".zip"

		exists, err := pathutil.IsPathExists(filepath.Join(deployDir, artifact.Name))
		if err != nil {
			return fmt.Errorf("failed to check path, error: %v", err)
		}

		artifactName := filepath.Base(artifact.Path)

		if exists {
			timestamp := time.Now().Format("20060102150405")
			ext := filepath.Ext(artifact.Name)
			name := strings.TrimSuffix(filepath.Base(artifact.Name), ext)
			artifact.Name = fmt.Sprintf("%s-%s%s", name, timestamp, ext)
		}

		log.Printf("  Export [ %s => $BITRISE_DEPLOY_DIR/%s ]", artifactName, artifact.Name)

		if err := artifact.ExportZIP(deployDir); err != nil {
			log.Warnf("failed to export artifact (%s), error: %v", artifact.Path, err)
			continue
		}
	}
	return nil
}

func getModuleMap(config Configs, variants gradle.Variants) map[string][]string {
	moduleVariants := map[string][]string{}
	for _, variant := range variants {
		split := strings.Split(variant, ":")
		if len(split) > 1 {
			moduleVariants[split[0]] = append(moduleVariants[split[0]], split[1])
		} else {
			moduleVariants[config.Module] = append(moduleVariants[config.Module], variant)
		}
	}
	return moduleVariants
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
	moduleMap := getModuleMap(config, variants)

	if len(filteredVariants) == 0 {
		errMsg := fmt.Sprintf("No variant matching for: (%s)", config.Variant)
		if config.Module != "" {
			errMsg += fmt.Sprintf(" in module: [%s]", config.Module)
		}
		failf(errMsg)
	}

	for module, variants := range moduleMap {
		log.Printf("%s:", module)
		for _, variant := range variants {
			if sliceutil.IsStringInSlice(module+":"+variant, filteredVariants) || sliceutil.IsStringInSlice(variant, filteredVariants) {
				log.Donef("✓ %s", strings.TrimSuffix(variant, "UnitTest"))
			} else {
				log.Printf("- %s", strings.TrimSuffix(variant, "UnitTest"))
			}
		}
	}
	fmt.Println()

	if config.Variant == "" {
		log.Warnf("No variant specified, test will run on all variants")
		fmt.Println()
	}

	started := time.Now()

	args, err := shellquote.Split(config.Arguments)
	if err != nil {
		failf("Failed to parse arguments, error: %s", err)
	}

	var testErr error

	log.Infof("Run test:")
	for module, variants := range moduleMap {
		testErr = gradleProject.GetModule(module).GetTask("test").Run(variants, args...)
		if testErr != nil {
			log.Errorf("Test task failed, error: %v", testErr)
		}
	}
	fmt.Println()

	log.Infof("Export reports:")
	fmt.Println()

	reports, err := getArtifacts(gradleProject, started, config.ReportPathPattern)
	if err != nil {
		failf("Failed to find reports, error: %v", err)
	}

	if err := exportArtifacts(deployDir, reports); err != nil {
		failf("Failed to export reports, error: %v", err)
	}

	fmt.Println()

	log.Infof("Export results:")
	fmt.Println()

	results, err := getArtifacts(gradleProject, started, config.ResultPathPattern)
	if err != nil {
		failf("Failed to find results, error: %v", err)
	}

	if err := exportArtifacts(deployDir, results); err != nil {
		failf("Failed to export results, error: %v", err)
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
