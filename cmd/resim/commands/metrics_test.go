package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSyncMetricsCmdHasMetricsConfigPathFlag(t *testing.T) {
	flag := syncMetricsCmd.Flags().Lookup("metrics-config-path")
	assert.NotNil(t, flag, "--metrics-config-path flag should exist on syncMetricsCmd")
}

func TestSyncMetricsCmdHasDeprecatedConfigPathFlag(t *testing.T) {
	flag := syncMetricsCmd.Flags().Lookup("config-path")
	assert.NotNil(t, flag, "--config-path flag should exist on syncMetricsCmd")
	assert.True(t, flag.Deprecated != "", "--config-path flag should be marked deprecated")
}
