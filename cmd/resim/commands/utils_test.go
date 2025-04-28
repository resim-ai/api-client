package commands

import (
	"fmt"
	"os"
	"testing"

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
	buildSpecBytes, err := ParseBuildSpec("../../../testing/data/test_build_spec.yaml")
	assert.NoError(t, err)
	assert.NotNil(t, buildSpecBytes)

	fmt.Println("First file:")
	fmt.Println(string(buildSpecBytes))

	buildSpecExpected, err := os.ReadFile("../../../testing/data/test_build_spec_combined.yaml")
	assert.NoError(t, err)
	assert.YAMLEq(t, string(buildSpecExpected), string(buildSpecBytes))

	fmt.Println("Expected:")
	fmt.Println(string(buildSpecExpected))
}
