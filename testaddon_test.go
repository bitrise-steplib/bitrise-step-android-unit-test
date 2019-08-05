package main

import (
	"testing"
)

func TestGetVariantDir(t *testing.T) {
	tc := []struct{
		title string
		path string
		wantStr string
		isErr bool
	}{
		{
			title: "should return error on empty string",
			path: "",
			wantStr: "",
			isErr: true, 
		},
		{
			title: "should return error if artifact path ends in test results folder with trailing slash",
			path: "/path/to/test-results/",
			wantStr: "",
			isErr: true, 
		},
	}
	
	for _, tt := range tc {
		str, err := getVariantDir(tt.path)
		if str != tt.wantStr || (err != nil) != tt.isErr {
			t.Fatalf("%s: got (%s, %s)", tt.title, str, err)
		}
	}

}

func TestGetExportDir(t *testing.T) {
	tc := []struct{
		title string
		artifactPath string
		want string
	}{
		{
			title: "should return 'other' for non mappable result path",
			artifactPath: "/path/to/non-android/result.xml",
			want: "other",
		},
		{
			title: "should return string in <module>-<variant> for android result path",
			artifactPath: "/path/to/app/module/test-results/testDemoDebugUnitTest/result.xml",
			want: "app-demoDebug",
		},
	}

	for _, tt := range tc {
		if got := getExportDir(tt.artifactPath); got != tt.want {
			t.Fatalf("%s: got '%s' want '%s'", tt.title, got, tt.want)
		}
	}
}