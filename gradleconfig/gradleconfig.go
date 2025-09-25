package gradleconfig

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/bitrise-io/go-utils/v2/fileutil"
	"github.com/bitrise-io/go-utils/v2/pathutil"
)

const (
	gradleHomeNonExpanded                    = "~/.gradle"
	testSkippingGradleInitScriptTemplateText = `allprojects {
    tasks.withType<Test>().configureEach {
        {{- range .ExcludedTests }}
        filter.excludeTestsMatching("{{ . }}")
        {{- end }}
    }
}`
)

type skipTestingTemplateData struct {
	ExcludedTests []string
}

func WriteSkipTestingInitScript(skipTesting []string) (string, error) {
	gradleHome, err := pathutil.NewPathModifier().AbsPath(gradleHomeNonExpanded)
	if err != nil {
		return "", fmt.Errorf("expand Gradle home path (%s): %w", gradleHome, err)
	}

	gradleInitDPath := filepath.Join(gradleHome, "init.d")
	if err := os.MkdirAll(gradleInitDPath, 0o755); err != nil {
		return "", fmt.Errorf("create Gradle init.d dir (%s): %w", gradleInitDPath, err)
	}

	initScriptContent, err := generateTestSkippingGradleInitScriptContent(skipTesting)
	if err != nil {
		return "", fmt.Errorf("generate Gradle init script content: %w", err)
	}

	initGradlePath := filepath.Join(gradleInitDPath, "bitrise-test-skipping.init.gradle.kts")
	err = fileutil.NewFileManager().Write(initGradlePath, initScriptContent, 0o755)
	if err != nil {
		return "", fmt.Errorf("write Gradle init script (%s): %w", initGradlePath, err)
	}

	return initGradlePath, nil
}

func generateTestSkippingGradleInitScriptContent(skipTesting []string) (string, error) {
	tmpl, err := template.New("bitrise-test-skipping.init.gradle.kts").Parse(testSkippingGradleInitScriptTemplateText)
	if err != nil {
		return "", err
	}

	resultBuffer := bytes.Buffer{}
	templateData := skipTestingTemplateData{ExcludedTests: skipTesting}
	if err := tmpl.Execute(&resultBuffer, templateData); err != nil {
		return "", err
	}

	return resultBuffer.String(), nil
}
