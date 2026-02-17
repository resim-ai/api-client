package commands

import (
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

func TestNormalizeMetricsConfigPath(t *testing.T) {
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Deprecated config-path maps to metrics-config-path",
			input:    "config-path",
			expected: "metrics-config-path",
		},
		{
			name:     "Canonical metrics-config-path unchanged",
			input:    "metrics-config-path",
			expected: "metrics-config-path",
		},
		{
			name:     "Unrelated flag unchanged",
			input:    "project",
			expected: "project",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := normalizeMetricsConfigPath(fs, tc.input)
			assert.Equal(t, pflag.NormalizedName(tc.expected), result)
		})
	}
}

func TestSyncMetricsCmdConfigPathAlias(t *testing.T) {
	// Verify that --config-path is resolved to --metrics-config-path on the actual command flags
	err := syncMetricsCmd.Flags().Set("config-path", "custom/path.yml")
	assert.NoError(t, err)

	val, err := syncMetricsCmd.Flags().GetStringSlice("metrics-config-path")
	assert.NoError(t, err)
	assert.Equal(t, []string{"custom/path.yml"}, val)
}
