package testaddon

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetVariantDir(t *testing.T) {
	tc := []struct {
		title       string
		path        string
		wantModule  string
		wantVariant string
		isErr       bool
	}{
		{
			title:       "should return variant dir",
			path:        "./app/build/test-results/testDebugUnitTest/TEST-sample.results.test.multiple.bitrise.com.multipletestresultssample.UnitTest0.xml",
			wantModule:  "app",
			wantVariant: "debug",
			isErr:       false,
		},
		{
			title:       "should return error on empty string",
			path:        "",
			wantModule:  "",
			wantVariant: "",
			isErr:       true,
		},
		{
			title:       "should return error for non default Local Android Unit result XML path",
			path:        "/path/to/test-results/",
			wantModule:  "",
			wantVariant: "",
			isErr:       true,
		},
		{
			title:       "should return error for non default Local Android Unit result XML path",
			path:        "./app/build/test-results/jacocoTestReleaseUnitTestReport/jacocoTestReleaseUnitTestReport.xml",
			wantModule:  "",
			wantVariant: "",
			isErr:       true,
		},
	}

	for _, tt := range tc {
		gotModule, gotVariant, err := getModuleAndVariant(tt.path)
		if tt.isErr {
			require.Error(t, err)
		} else {
			require.Equal(t, tt.wantModule, gotModule)
			require.Equal(t, tt.wantVariant, gotVariant)
		}
	}

}

func TestGetExportDir(t *testing.T) {
	tc := []struct {
		title        string
		artifactPath string
		want         string
	}{
		{
			title:        "should return 'other' for non mappable result path",
			artifactPath: "./app/build/test-results/jacocoTestReleaseUnitTestReport/jacocoTestReleaseUnitTestReport.xml",
			want:         "other",
		},
		{
			title:        "should return string in <module>-<variant> for android result path",
			artifactPath: "./app/build/test-results/testDemoDebugUnitTest/TEST-sample.results.test.multiple.bitrise.com.multipletestresultssample.UnitTest0.xml",
			want:         "app-demoDebug",
		},
	}

	for _, tt := range tc {
		if got := getExportDir(tt.artifactPath); got != tt.want {
			t.Fatalf("%s: got '%s' want '%s'", tt.title, got, tt.want)
		}
	}
}
