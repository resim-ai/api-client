package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	compose_types "github.com/compose-spec/compose-go/v2/types"
	"github.com/stretchr/testify/assert"
)

func TestParseParameterString(t *testing.T) {
	tests := []struct {
		name            string
		parameterString string
		expectedKey     string
		expectedValue   string
		shouldError     bool
	}{
		{
			name:            "Simple parameter with equals",
			parameterString: "key=value",
			expectedKey:     "key",
			expectedValue:   "value",
			shouldError:     false,
		},
		{
			name:            "Simple parameter with colon",
			parameterString: "key:value",
			expectedKey:     "key",
			expectedValue:   "value",
			shouldError:     false,
		},
		{
			name:            "Parameter with double colon in name using equals",
			parameterString: "namespace::key=value",
			expectedKey:     "namespace::key",
			expectedValue:   "value",
			shouldError:     false,
		},
		{
			name:            "Parameter with double colon in name and value using equals",
			parameterString: "namespace::key=prefix::value",
			expectedKey:     "namespace::key",
			expectedValue:   "prefix::value",
			shouldError:     false,
		},
		{
			name:            "Parameter with multiple double colons using equals",
			parameterString: "namespace::section::key=value",
			expectedKey:     "namespace::section::key",
			expectedValue:   "value",
			shouldError:     false,
		},
		{
			name:            "Value with equals sign",
			parameterString: "key=value=with=equals",
			expectedKey:     "key",
			expectedValue:   "value=with=equals",
			shouldError:     false,
		},
		{
			name:            "Value with colon",
			parameterString: "key:value:with:colons",
			expectedKey:     "key",
			expectedValue:   "value:with:colons",
			shouldError:     false,
		},
		{
			name:            "Empty value with equals",
			parameterString: "key=",
			expectedKey:     "key",
			expectedValue:   "",
			shouldError:     false,
		},
		{
			name:            "Empty value with colon",
			parameterString: "key:",
			expectedKey:     "key",
			expectedValue:   "",
			shouldError:     false,
		},
		{
			name:            "Preference for equals over colon",
			parameterString: "key=value:with:colon",
			expectedKey:     "key",
			expectedValue:   "value:with:colon",
			shouldError:     false,
		},
		{
			name:            "Invalid parameter - no delimiter",
			parameterString: "keyvalue",
			expectedKey:     "",
			expectedValue:   "",
			shouldError:     true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			key, value, err := ParseParameterString(test.parameterString)

			// Check error state
			if test.shouldError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !test.shouldError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			// If we don't expect an error, check the returned values
			if !test.shouldError {
				if key != test.expectedKey {
					t.Errorf("Expected key %q but got %q", test.expectedKey, key)
				}
				if value != test.expectedValue {
					t.Errorf("Expected value %q but got %q", test.expectedValue, value)
				}
			}
		})
	}
}

func TestParseBuildSpec(t *testing.T) {
	buildSpecBytes, err := ParseBuildSpec("../../../testing/data/test_build_spec.yaml", false, []string{})
	assert.NoError(t, err)
	assert.NotNil(t, buildSpecBytes)

	fmt.Println("First file:")
	fmt.Println(string(buildSpecBytes))

	buildSpecExpected, err := os.ReadFile("../../../testing/data/test_build_spec_combined.json")
	assert.NoError(t, err)
	assert.YAMLEq(t, string(buildSpecExpected), string(buildSpecBytes))

	fmt.Println("Expected:")
	fmt.Println(string(buildSpecExpected))
}

func TestParseBuildSpecWithOsEnv(t *testing.T) {
	os.Setenv("SET_BY_OUTSIDE_ENV", "test_value")
	defer os.Unsetenv("SET_BY_OUTSIDE_ENV")

	buildSpecBytes, err := ParseBuildSpec("../../../testing/data/test_build_spec.yaml", true, []string{})
	assert.NoError(t, err)
	assert.NotNil(t, buildSpecBytes)

	var buildSpec compose_types.Project
	err = json.Unmarshal(buildSpecBytes, &buildSpec)
	assert.NoError(t, err)
	assert.Equal(t, "test_value", *buildSpec.Services["system"].Environment["SET_BY_OUTSIDE_ENV"])
}

func TestParseBuildSpecWithEnvFiles(t *testing.T) {
	envName := "SET_BY_OUTSIDE_ENV"
	envValue := "another_test_value"
	envFile, err := os.CreateTemp(".", ".env")
	assert.NoError(t, err)
	defer os.Remove(envFile.Name())
	envFile.WriteString(fmt.Sprintf("%s=%s\n", envName, envValue))
	envFile.Close()

	buildSpecBytes, err := ParseBuildSpec("../../../testing/data/test_build_spec.yaml", false, []string{envFile.Name()})
	assert.NoError(t, err)
	assert.NotNil(t, buildSpecBytes)

	var buildSpec compose_types.Project
	err = json.Unmarshal(buildSpecBytes, &buildSpec)
	assert.NoError(t, err)
	assert.Equal(t, envValue, *buildSpec.Services["system"].Environment[envName])
}
