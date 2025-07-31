package output

import (
	"testing"

	"github.com/bitrise-steplib/bitrise-step-android-unit-test/mocks"
	"github.com/bitrise-steplib/steps-deploy-to-bitrise-io/test/testreport"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

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
	tests := []struct {
		name            string
		flakyTestSuites []testreport.TestSuite
		wantEnvVarValue string
	}{
		{
			name:            "No flaky test cases",
			flakyTestSuites: nil,
			wantEnvVarValue: "",
		},
		{
			name: "Single flaky test suite",
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
	}
	for _, tt := range tests {
		logger := mocks.NewLogger(t)
		envRepository := mocks.NewRepository(t)
		if tt.wantEnvVarValue != "" {
			logger.On("Donef", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			envRepository.On("Set", flakyTestCasesEnvVarKey, tt.wantEnvVarValue).Return(nil)
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
