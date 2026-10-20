package output

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bitrise-io/go-android/v2/gradle"
	"github.com/bitrise-io/go-utils/v2/env"
	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-utils/v2/pathutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_exporter_ExportTestAddonArtifacts_exportsAttachmentsNamedAfterTheirTestCase(t *testing.T) {
	projectDir := t.TempDir()
	deployDir := t.TempDir()
	t.Setenv("BITRISE_TEST_DEPLOY_DIR", deployDir)
	testDeployDir := filepath.Join(deployDir, "step_1")

	debugXML := writeTestFile(t, filepath.Join(projectDir, "app/build/test-results/testDebugUnitTest/TEST-com.example.LoginTest.xml"),
		`<testsuite name="com.example.LoginTest"><testcase classname="com.example.LoginTest" name="emptyState"/></testsuite>`)
	releaseXML := writeTestFile(t, filepath.Join(projectDir, "app/build/test-results/testReleaseUnitTest/TEST-com.example.SignupTest.xml"),
		`<testsuite name="com.example.SignupTest"><testcase classname="com.example.SignupTest" name="weakPassword"/></testsuite>`)
	writeTestFile(t, filepath.Join(projectDir, "app/build/outputs/roborazzi/com.example.LoginTest__emptyState__1.png"), "screenshot")
	writeTestFile(t, filepath.Join(projectDir, "app/build/outputs/roborazzi/com.example.UnknownTest__someTest__1.png"), "screenshot")

	e := NewExporter(env.NewRepository(), pathutil.NewPathChecker(), log.NewLogger())
	exported, err := e.ExportTestAddonArtifacts(testDeployDir, []gradle.Artifact{{Path: debugXML}, {Path: releaseXML}}, projectDir)
	require.NoError(t, err)
	require.Len(t, exported, 2)

	assert.Equal(t, []string{"TEST-com.example.LoginTest.xml", "com.example.LoginTest__emptyState__1.png", "test-info.json"}, dirEntries(t, filepath.Join(testDeployDir, "app-debug")))
	assert.Equal(t, []string{"TEST-com.example.SignupTest.xml", "test-info.json"}, dirEntries(t, filepath.Join(testDeployDir, "app-release")))
}

func Test_exporter_ExportTestAddonArtifacts_skipsAttachmentsExportedByEarlierSteps(t *testing.T) {
	projectDir := t.TempDir()
	deployDir := t.TempDir()
	t.Setenv("BITRISE_TEST_DEPLOY_DIR", deployDir)
	testDeployDir := filepath.Join(deployDir, "step_2")

	resultXML := writeTestFile(t, filepath.Join(projectDir, "app/build/test-results/testDebugUnitTest/TEST-com.example.LoginTest.xml"),
		`<testsuite name="com.example.LoginTest"><testcase classname="com.example.LoginTest" name="emptyState"/></testsuite>`)
	attachment := writeTestFile(t, filepath.Join(projectDir, "app/build/outputs/roborazzi/com.example.LoginTest__emptyState__1.png"), "screenshot")
	exportedCopy := writeTestFile(t, filepath.Join(deployDir, "step_1", "Screenshots", "com.example.LoginTest__emptyState__1.png"), "screenshot")
	modTime := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	require.NoError(t, os.Chtimes(attachment, modTime, modTime))
	require.NoError(t, os.Chtimes(exportedCopy, modTime, modTime))

	e := NewExporter(env.NewRepository(), pathutil.NewPathChecker(), log.NewLogger())
	_, err := e.ExportTestAddonArtifacts(testDeployDir, []gradle.Artifact{{Path: resultXML}}, projectDir)
	require.NoError(t, err)

	assert.Equal(t, []string{"TEST-com.example.LoginTest.xml", "test-info.json"}, dirEntries(t, filepath.Join(testDeployDir, "app-debug")))
}

func writeTestFile(t *testing.T, path, content string) string {
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func dirEntries(t *testing.T, dir string) []string {
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}
