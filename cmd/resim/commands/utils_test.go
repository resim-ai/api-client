package commands

import (
	"fmt"
	"os"
	"testing"

	"github.com/resim-ai/api-client/api"
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
	buildSpec, err := ParseBuildSpec("../../../testing/data/test_build_spec.yaml", false, []string{}, []string{"*"}, false)
	assert.NoError(t, err)
	assert.NotNil(t, buildSpec)

	buildSpecBytes, err := buildSpec.MarshalJSON()
	assert.NoError(t, err)
	assert.NotNil(t, buildSpecBytes)

	buildSpecExpected, err := os.ReadFile("../../../testing/data/test_build_spec_combined.json")
	assert.NoError(t, err)
	assert.YAMLEq(t, string(buildSpecExpected), string(buildSpecBytes))

	assert.Contains(t, buildSpec.Services, "system")
	assert.Contains(t, buildSpec.Services, "orchestrator")
	assert.Contains(t, buildSpec.Services, "command-orchestrator")
	assert.Contains(t, buildSpec.Services, "entrypoint-orchestrator")
}

func TestParseBuildSpecWithOsEnv(t *testing.T) {
	os.Setenv("SET_BY_OUTSIDE_ENV", "test_value")
	defer os.Unsetenv("SET_BY_OUTSIDE_ENV")

	buildSpec, err := ParseBuildSpec("../../../testing/data/test_build_spec.yaml", true, []string{}, []string{"*"}, false)
	assert.NoError(t, err)
	assert.NotNil(t, buildSpec)

	buildSpecBytes, err := buildSpec.MarshalJSON()
	assert.NoError(t, err)
	assert.NotNil(t, buildSpecBytes)

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

	buildSpec, err := ParseBuildSpec("../../../testing/data/test_build_spec.yaml", false, []string{envFile.Name()}, []string{"*"}, false)
	assert.NoError(t, err)
	assert.NotNil(t, buildSpec)

	buildSpecBytes, err := buildSpec.MarshalJSON()
	assert.NoError(t, err)
	assert.NotNil(t, buildSpecBytes)

	assert.Equal(t, envValue, *buildSpec.Services["system"].Environment[envName])
}

func TestParseBuildSpecWithProfiles(t *testing.T) {
	buildSpec, err := ParseBuildSpec("../../../testing/data/test_build_spec.yaml", false, []string{}, []string{"profile2"}, false)
	assert.NoError(t, err)
	assert.NotNil(t, buildSpec)

	assert.Contains(t, buildSpec.Services, "system")                  // no profile
	assert.Contains(t, buildSpec.Services, "orchestrator")            // profile2
	assert.NotContains(t, buildSpec.Services, "command-orchestrator") // profile1
	assert.Contains(t, buildSpec.Services, "entrypoint-orchestrator") // no profile
}

func TestMergeConfigs(t *testing.T) {
	tests := []struct {
		name        string
		configs     []MetricsConfig
		expected    MetricsConfig
		shouldError bool
		errContains string
	}{
		{
			name: "Single config returned as-is",
			configs: []MetricsConfig{
				{
					Version: 1,
					Topics:  map[string]interface{}{"topic_a": map[string]interface{}{"type": "float"}},
					Metrics: map[string]interface{}{"metric_a": map[string]interface{}{"topic": "topic_a"}},
					MetricsSets: map[string]interface{}{"set_a": map[string]interface{}{
						"metrics": []interface{}{"metric_a"},
					}},
				},
			},
			expected: MetricsConfig{
				Version: 1,
				Topics:  map[string]interface{}{"topic_a": map[string]interface{}{"type": "float"}},
				Metrics: map[string]interface{}{"metric_a": map[string]interface{}{"topic": "topic_a"}},
				MetricsSets: map[string]interface{}{"set_a": map[string]interface{}{
					"metrics": []interface{}{"metric_a"},
				}},
			},
		},
		{
			name: "Two configs with disjoint entries merge correctly",
			configs: []MetricsConfig{
				{
					Version: 1,
					Topics:  map[string]interface{}{"topic_a": "val_a"},
					Metrics: map[string]interface{}{"metric_a": "val_a"},
				},
				{
					Version:     1,
					Topics:      map[string]interface{}{"topic_b": "val_b"},
					Metrics:     map[string]interface{}{"metric_b": "val_b"},
					MetricsSets: map[string]interface{}{"set_b": "val_b"},
				},
			},
			expected: MetricsConfig{
				Version:     1,
				Topics:      map[string]interface{}{"topic_a": "val_a", "topic_b": "val_b"},
				Metrics:     map[string]interface{}{"metric_a": "val_a", "metric_b": "val_b"},
				MetricsSets: map[string]interface{}{"set_b": "val_b"},
			},
		},
		{
			name: "Duplicate topic name causes error",
			configs: []MetricsConfig{
				{Topics: map[string]interface{}{"dup_topic": "a"}},
				{Topics: map[string]interface{}{"dup_topic": "b"}},
			},
			shouldError: true,
			errContains: "duplicate topic name",
		},
		{
			name: "Duplicate metric name causes error",
			configs: []MetricsConfig{
				{Metrics: map[string]interface{}{"dup_metric": "a"}},
				{Metrics: map[string]interface{}{"dup_metric": "b"}},
			},
			shouldError: true,
			errContains: "duplicate metric name",
		},
		{
			name: "Duplicate metrics set name causes error",
			configs: []MetricsConfig{
				{MetricsSets: map[string]interface{}{"dup_set": "a"}},
				{MetricsSets: map[string]interface{}{"dup_set": "b"}},
			},
			shouldError: true,
			errContains: "duplicate metrics set name",
		},
		{
			name: "Mismatched versions cause error",
			configs: []MetricsConfig{
				{Version: 1},
				{Version: 3},
			},
			shouldError: true,
			errContains: "conflicting versions",
		},
		{
			name: "Same version across configs is fine",
			configs: []MetricsConfig{
				{Version: 1},
				{Version: 1},
			},
			expected: MetricsConfig{
				Version:     1,
				Topics:      map[string]interface{}{},
				Metrics:     map[string]interface{}{},
				MetricsSets: map[string]interface{}{},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := mergeConfigs(tc.configs)
			if tc.shouldError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.errContains)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected.Version, result.Version)
				assert.Equal(t, tc.expected.Topics, result.Topics)
				assert.Equal(t, tc.expected.Metrics, result.Metrics)
				assert.Equal(t, tc.expected.MetricsSets, result.MetricsSets)
			}
		})
	}
}

func TestMergeConfigFiles(t *testing.T) {
	t.Run("Single file backwards compatible", func(t *testing.T) {
		tmpDir := t.TempDir()
		file1 := fmt.Sprintf("%s/config.yml", tmpDir)
		os.WriteFile(file1, []byte("version: 1\ntopics:\n  t1:\n    type: float\n"), 0644)

		data, err := mergeConfigFiles([]string{file1}, false)
		assert.NoError(t, err)
		assert.Contains(t, string(data), "t1")
	})

	t.Run("Two files with additive content", func(t *testing.T) {
		tmpDir := t.TempDir()
		file1 := fmt.Sprintf("%s/a.yml", tmpDir)
		file2 := fmt.Sprintf("%s/b.yml", tmpDir)
		os.WriteFile(file1, []byte("version: 1\ntopics:\n  t1:\n    type: float\nmetrics:\n  m1:\n    topic: t1\n"), 0644)
		os.WriteFile(file2, []byte("version: 1\ntopics:\n  t2:\n    type: int\nmetrics:\n  m2:\n    topic: t2\n"), 0644)

		data, err := mergeConfigFiles([]string{file1, file2}, false)
		assert.NoError(t, err)
		content := string(data)
		assert.Contains(t, content, "t1")
		assert.Contains(t, content, "t2")
		assert.Contains(t, content, "m1")
		assert.Contains(t, content, "m2")
	})

	t.Run("File not found returns error", func(t *testing.T) {
		_, err := mergeConfigFiles([]string{"/nonexistent/path/config.yml"}, false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to find ReSim metrics config")
	})

	t.Run("Invalid YAML returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		file1 := fmt.Sprintf("%s/bad.yml", tmpDir)
		os.WriteFile(file1, []byte(":\n  :\n  invalid: [yaml: {"), 0644)

		_, err := mergeConfigFiles([]string{file1}, false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse config file")
	})

	t.Run("Glob pattern merges multiple files", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.WriteFile(fmt.Sprintf("%s/a.yml", tmpDir), []byte("version: 1\ntopics:\n  t1:\n    type: float\n"), 0644)
		os.WriteFile(fmt.Sprintf("%s/b.yml", tmpDir), []byte("version: 1\ntopics:\n  t2:\n    type: int\n"), 0644)

		globPattern := fmt.Sprintf("%s/*.yml", tmpDir)
		data, err := mergeConfigFiles([]string{globPattern}, false)
		assert.NoError(t, err)
		content := string(data)
		assert.Contains(t, content, "t1")
		assert.Contains(t, content, "t2")
	})

	t.Run("Mixed literal and glob paths", func(t *testing.T) {
		tmpDir := t.TempDir()
		subDir := fmt.Sprintf("%s/sub", tmpDir)
		os.Mkdir(subDir, 0755)

		baseFile := fmt.Sprintf("%s/base.yml", tmpDir)
		os.WriteFile(baseFile, []byte("version: 1\ntopics:\n  t_base:\n    type: float\n"), 0644)
		os.WriteFile(fmt.Sprintf("%s/ext1.yml", subDir), []byte("version: 1\ntopics:\n  t_ext1:\n    type: int\n"), 0644)
		os.WriteFile(fmt.Sprintf("%s/ext2.yml", subDir), []byte("version: 1\nmetrics:\n  m_ext2:\n    topic: t_base\n"), 0644)

		data, err := mergeConfigFiles([]string{baseFile, fmt.Sprintf("%s/*.yml", subDir)}, false)
		assert.NoError(t, err)
		content := string(data)
		assert.Contains(t, content, "t_base")
		assert.Contains(t, content, "t_ext1")
		assert.Contains(t, content, "m_ext2")
	})

	t.Run("Glob with duplicate keys across files returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.WriteFile(fmt.Sprintf("%s/a.yml", tmpDir), []byte("topics:\n  dup:\n    type: float\n"), 0644)
		os.WriteFile(fmt.Sprintf("%s/b.yml", tmpDir), []byte("topics:\n  dup:\n    type: int\n"), 0644)

		globPattern := fmt.Sprintf("%s/*.yml", tmpDir)
		_, err := mergeConfigFiles([]string{globPattern}, false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate topic name")
	})
}

func TestResolveConfigPath(t *testing.T) {
	t.Run("Relative .yml found", func(t *testing.T) {
		tmpDir := t.TempDir()
		file := fmt.Sprintf("%s/config.yml", tmpDir)
		os.WriteFile(file, []byte("version: 1"), 0644)

		resolved, err := resolveConfigPath(file)
		assert.NoError(t, err)
		assert.Len(t, resolved, 1)
		assert.Equal(t, file, resolved[0])
	})

	t.Run(".yaml fallback found", func(t *testing.T) {
		tmpDir := t.TempDir()
		yamlFile := fmt.Sprintf("%s/config.yaml", tmpDir)
		os.WriteFile(yamlFile, []byte("version: 1"), 0644)

		ymlPath := fmt.Sprintf("%s/config.yml", tmpDir)
		resolved, err := resolveConfigPath(ymlPath)
		assert.NoError(t, err)
		assert.Len(t, resolved, 1)
		assert.Equal(t, yamlFile, resolved[0])
	})

	t.Run("Absolute path used as-is", func(t *testing.T) {
		tmpDir := t.TempDir()
		file := fmt.Sprintf("%s/config.yml", tmpDir)
		os.WriteFile(file, []byte("version: 1"), 0644)

		resolved, err := resolveConfigPath(file)
		assert.NoError(t, err)
		assert.Len(t, resolved, 1)
		assert.Equal(t, file, resolved[0])
	})

	t.Run("Missing file returns error", func(t *testing.T) {
		_, err := resolveConfigPath("/nonexistent/path/config.yml")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to find ReSim metrics config")
	})

	t.Run("Glob matches multiple files sorted", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.WriteFile(fmt.Sprintf("%s/c.yml", tmpDir), []byte("version: 1"), 0644)
		os.WriteFile(fmt.Sprintf("%s/a.yml", tmpDir), []byte("version: 1"), 0644)
		os.WriteFile(fmt.Sprintf("%s/b.yml", tmpDir), []byte("version: 1"), 0644)

		resolved, err := resolveConfigPath(fmt.Sprintf("%s/*.yml", tmpDir))
		assert.NoError(t, err)
		assert.Len(t, resolved, 3)
		assert.Equal(t, fmt.Sprintf("%s/a.yml", tmpDir), resolved[0])
		assert.Equal(t, fmt.Sprintf("%s/b.yml", tmpDir), resolved[1])
		assert.Equal(t, fmt.Sprintf("%s/c.yml", tmpDir), resolved[2])
	})

	t.Run("Glob matches single file", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.WriteFile(fmt.Sprintf("%s/only.yml", tmpDir), []byte("version: 1"), 0644)

		resolved, err := resolveConfigPath(fmt.Sprintf("%s/only*.yml", tmpDir))
		assert.NoError(t, err)
		assert.Len(t, resolved, 1)
		assert.Equal(t, fmt.Sprintf("%s/only.yml", tmpDir), resolved[0])
	})

	t.Run("Glob with no matches returns error", func(t *testing.T) {
		tmpDir := t.TempDir()

		_, err := resolveConfigPath(fmt.Sprintf("%s/*.yml", tmpDir))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "matched no files")
	})
}

func TestAddMetrics2PoolLabels(t *testing.T) {
	poolLabels := []api.PoolLabel{}
	AddMetrics2PoolLabels(&poolLabels)
	assert.Contains(t, poolLabels, METRICS_2_POOL_LABEL)
}

func TestAddMetrics2PoolLabelsWithExistingLabels(t *testing.T) {
	poolLabels := []api.PoolLabel{"foo", "bar"}
	AddMetrics2PoolLabels(&poolLabels)
	assert.Contains(t, poolLabels, METRICS_2_POOL_LABEL)
	assert.Equal(t, []api.PoolLabel{"foo", "bar", METRICS_2_POOL_LABEL}, poolLabels)
}
