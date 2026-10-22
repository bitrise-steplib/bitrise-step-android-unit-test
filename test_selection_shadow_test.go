package main

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/bitrise-io/go-android/v2/gradle"
	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-steplib/bitrise-step-test-selection/selector"
	"github.com/stretchr/testify/require"
)

func TestRunsModule(t *testing.T) {
	keep := runsModule(gradle.Variants{"app": {"DebugUnitTest"}, "feature:home": {"DebugUnitTest"}, "": {"DebugUnitTest"}})
	test := func(module string) selector.Test {
		return selector.Test{Ident: selector.Identifiers{GradleModule: module}}
	}
	require.True(t, keep(test(":app"), ""))
	require.True(t, keep(test(":feature:home"), ""))
	require.True(t, keep(test(":"), ""), "the root project")
	require.False(t, keep(test(":domain"), ""), "a module the step does not run")
}

func TestModuleOutcomes(t *testing.T) {
	root := t.TempDir()
	write := func(rel, content string) {
		p := filepath.Join(root, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	}
	write("app/build/test-results/testDebugUnitTest/TEST-a.xml", `<testsuite><testcase name="x" time="1.5"/></testsuite>`)
	write("feature/home/build/test-results/testDebugUnitTest/TEST-b.xml", `<testsuite><testcase name="y" time="0.5"><failure/></testcase></testsuite>`)
	write("feature/home/build/test-results/testReleaseUnitTest/TEST-c.xml", `<testsuite><testcase name="z" time="9"/></testsuite>`)

	outcomes := moduleOutcomes(root, gradle.Variants{"app": {"DebugUnitTest"}, "feature:home": {"DebugUnitTest"}, "core": {"DebugUnitTest"}}, log.NewLogger())
	sort.Slice(outcomes, func(i, j int) bool { return outcomes[i].Key < outcomes[j].Key })
	require.Equal(t, []selector.UnitOutcome{
		{Key: ":app", Tests: 1, Duration: 1500 * time.Millisecond},
		{Key: ":feature:home", Tests: 1, Failed: 1, Duration: 500 * time.Millisecond},
	}, outcomes, "only the variants the step ran count, and a module without results is left out")
}
