package output

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/bitrise-io/go-android/v2/gradle"
	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-utils/v2/pathutil"
	"github.com/bitrise-steplib/bitrise-step-android-unit-test/mocks"
	"github.com/bitrise-steplib/steps-deploy-to-bitrise-io/test/converters/junitxml"
	"github.com/bitrise-steplib/steps-deploy-to-bitrise-io/test/testreport"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func Test_exporter_ExportFlakyTestsEnvVar(t *testing.T) {
	_, b, _, _ := runtime.Caller(0)
	outputPackageDir := filepath.Dir(b)
	testDataDir := filepath.Join(outputPackageDir, "testdata")
	testResultXML := filepath.Join(testDataDir, "TEST-io.bitrise.kotlinresponsiveviewsactivity.UniTest.xml")

	envRepository := mocks.NewRepository(t)
	envRepository.On("Set", flakyTestCasesEnvVarKey, "- io.bitrise.kotlinresponsiveviewsactivity.UniTest.io.bitrise.kotlinresponsiveviewsactivity.UniTest.flaky\n").Return(nil)

	e := exporter{
		envRepository: envRepository,
		pathChecker:   pathutil.NewPathChecker(),
		logger:        log.NewLogger(),
		converter:     junitxml.Converter{},
	}
	err := e.ExportFlakyTestsEnvVar([]gradle.Artifact{{Path: testResultXML}})
	require.NoError(t, err)
}

func Test_exporter_getFlakyTestSuites(t *testing.T) {
	tests := []struct {
		name       string
		testReport testreport.TestReport
		want       []testreport.TestSuite
	}{
		{
			name:       "No flaky test cases",
			testReport: testreport.TestReport{},
			want:       nil,
		},
		{
			name: "Single flaky test suite",
			testReport: testreport.TestReport{
				TestSuites: []testreport.TestSuite{
					{
						Name: "Suite1",
						TestCases: []testreport.TestCase{
							{
								ClassName: "com.example.TestClass",
								Name:      "testMethod1",
								Failure: &testreport.Failure{
									Value: "Test failed",
								},
							},
							{
								ClassName: "com.example.TestClass",
								Name:      "testMethod1",
							},
						},
					},
				},
			},
			want: []testreport.TestSuite{
				{
					Name: "Suite1",
					TestCases: []testreport.TestCase{
						{
							ClassName: "com.example.TestClass",
							Name:      "testMethod1",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := exporter{}
			got := e.getFlakyTestSuites(tt.testReport)
			require.Equal(t, tt.want, got)
		})
	}
}

func Test_exporter_exportFlakyTestCasesEnvVar(t *testing.T) {
	longTestSuitName1 := "Suite1"
	longTestCaseName1 := strings.Repeat("a", flakyTestCasesEnvVarSizeLimitInBytes-len(fmt.Sprintf("- %s.\n", longTestSuitName1)))

	tests := []struct {
		name                   string
		flakyTestSuites        []testreport.TestSuite
		wantEnvVarValue        string
		expectedWarningLogArgs []any
	}{
		{
			name:            "No flaky test cases",
			flakyTestSuites: nil,
			wantEnvVarValue: "",
		},
		{
			name: "Single flaky test",
			flakyTestSuites: []testreport.TestSuite{
				{
					Name: "Suite1",
					TestCases: []testreport.TestCase{
						{
							ClassName: "com.example.TestClass",
							Name:      "testMethod1",
						},
					},
				},
			},
			wantEnvVarValue: "- Suite1.com.example.TestClass.testMethod1\n",
		},
		{
			name: "Multiple flaky tests",
			flakyTestSuites: []testreport.TestSuite{
				{
					Name: "Suite1",
					TestCases: []testreport.TestCase{
						{
							ClassName: "com.example.TestClass1",
							Name:      "testMethod1",
						},
						{
							ClassName: "com.example.TestClass1",
							Name:      "testMethod2",
						},
						{
							ClassName: "com.example.TestClass2",
							Name:      "testMethod1",
						},
					},
				},
				{
					Name: "Suite2",
					TestCases: []testreport.TestCase{
						{
							ClassName: "com.example.TestClass1",
							Name:      "testMethod1",
						},
					},
				},
			},
			wantEnvVarValue: `- Suite1.com.example.TestClass1.testMethod1
- Suite1.com.example.TestClass1.testMethod2
- Suite1.com.example.TestClass2.testMethod1
- Suite2.com.example.TestClass1.testMethod1
`,
		},
		{
			name: "Tests with the same Test ID exported only once",
			flakyTestSuites: []testreport.TestSuite{
				{
					Name: "Suite1",
					TestCases: []testreport.TestCase{
						{
							ClassName: "com.example.TestClass1",
							Name:      "testMethod1",
						},
						{
							ClassName: "com.example.TestClass1",
							Name:      "testMethod1",
						},
					},
				},
				{
					Name: "Suite1",
					TestCases: []testreport.TestCase{
						{
							ClassName: "com.example.TestClass1",
							Name:      "testMethod1",
						},
					},
				},
			},
			wantEnvVarValue: "- Suite1.com.example.TestClass1.testMethod1\n",
		},
		{
			name: "Flaky test cases env var size is limited",
			flakyTestSuites: []testreport.TestSuite{
				{
					Name: longTestSuitName1,
					TestCases: []testreport.TestCase{
						{
							Name: longTestCaseName1,
						},
					},
				},
				{
					Name: "Suite2",
					TestCases: []testreport.TestCase{
						{
							Name: "testMethod1",
						},
					},
				},
			},
			wantEnvVarValue:        fmt.Sprintf("- %s.%s\n", longTestSuitName1, longTestCaseName1),
			expectedWarningLogArgs: []any{"%s env var size limit (%d characters) exceeded. Skipping %d test cases.", flakyTestCasesEnvVarKey, flakyTestCasesEnvVarSizeLimitInBytes, 1},
		},
	}
	for _, tt := range tests {
		logger := mocks.NewLogger(t)
		envRepository := mocks.NewRepository(t)
		if tt.wantEnvVarValue != "" {
			logger.On("Donef", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			envRepository.On("Set", flakyTestCasesEnvVarKey, tt.wantEnvVarValue).Return(nil)
		}
		if tt.expectedWarningLogArgs != nil {
			logger.On("Warnf", tt.expectedWarningLogArgs...).Return(tt.expectedWarningLogArgs...)
		}

		t.Run(tt.name, func(t *testing.T) {
			e := exporter{
				envRepository: envRepository,
				logger:        logger,
			}
			err := e.exportFlakyTestCasesEnvVar(tt.flakyTestSuites)
			require.NoError(t, err)
		})
	}
}
