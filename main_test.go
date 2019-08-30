package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bitrise-io/go-utils/pathutil"
)

func Test_tryExportTestAddonArtifact(t *testing.T) {
	tmpDir, err := pathutil.NormalizedOSTempDirPath("")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name            string
		artifactPth     string
		outputDir       string
		lastOtherDirIdx int

		wantIdx       int
		wantOutputPth string
	}{
		{
			name:            "Exports Local Unit Test result XML file",
			artifactPth:     filepath.Join(tmpDir, "./app/build/test-results/testDebugUnitTest/TEST-sample.results.test.multiple.bitrise.com.multipletestresultssample.UnitTest0.xml"),
			outputDir:       filepath.Join(tmpDir, "1"),
			lastOtherDirIdx: 0,
			wantIdx:         0,
			wantOutputPth:   filepath.Join(tmpDir, "1", "app-debug", "TEST-sample.results.test.multiple.bitrise.com.multipletestresultssample.UnitTest0.xml"),
		},
		{
			name:            "Exports Jacoco result XML file",
			artifactPth:     filepath.Join(tmpDir, "./app/build/test-results/jacocoTestReleaseUnitTestReport/jacocoTestReleaseUnitTestReport.xml"),
			outputDir:       filepath.Join(tmpDir, "2"),
			lastOtherDirIdx: 0,
			wantIdx:         1,
			wantOutputPth:   filepath.Join(tmpDir, "2", "other-1", "jacocoTestReleaseUnitTestReport.xml"),
		},
		{
			name:            "Exports Other XML file",
			artifactPth:     filepath.Join(tmpDir, "./app/build/test-results/TEST-sample.results.test.multiple.bitrise.com.multipletestresultssample.UnitTest0.xml"),
			outputDir:       filepath.Join(tmpDir, "3"),
			lastOtherDirIdx: 0,
			wantIdx:         1,
			wantOutputPth:   filepath.Join(tmpDir, "3", "other-1", "TEST-sample.results.test.multiple.bitrise.com.multipletestresultssample.UnitTest0.xml"),
		},
	}
	for _, tt := range tests {
		dir := filepath.Dir(tt.artifactPth)
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Error(err)
			continue
		}
		if _, err := os.Create(tt.artifactPth); err != nil {
			t.Error(err)
			continue
		}

		t.Run(tt.name, func(t *testing.T) {
			if got := tryExportTestAddonArtifact(tt.artifactPth, tt.outputDir, tt.lastOtherDirIdx); got != tt.wantIdx {
				t.Errorf("tryExportTestAddonArtifact() = %v, want %v", got, tt.wantIdx)
			}
			if exist, err := pathutil.IsPathExists(tt.wantOutputPth); err != nil {
				t.Error(err)
			} else if !exist {
				t.Errorf("expected output file (%s) does not exist", tt.wantOutputPth)
			}
		})
	}
}
