package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func sampleCommand() cobra.Command {
	var testCmd = cobra.Command{
		Use:           "resim",
		Short:         "resim - Command Line Interface for ReSim",
		Long:          ``,
		SilenceErrors: true,
		SilenceUsage:  true,
		PreRun:        RegisterViperFlagsAndSetClient,
	}
	return testCmd
}

func writeStubConfig(params map[string]interface{}) string {
	// Writes a viper config to a temporary directory and returns the path
	tempDir, _ := os.MkdirTemp(os.TempDir(), "resim-")
	os.MkdirAll(filepath.Join(tempDir, ".resim"), os.ModePerm)
	os.Setenv("HOME", tempDir)
	v := viper.New()
	v.MergeConfigMap(params)
	v.WriteConfigAs(os.ExpandEnv(ConfigPath) + "/resim.yaml")
	return tempDir
}

func TestRequiredFlagNotProvided(t *testing.T) {
	assert := assert.New(t)
	var configParams = make(map[string]interface{})
	configDir := writeStubConfig(configParams)
	defer os.RemoveAll(configDir)
	os.Setenv("HOME", configDir)
	// SETUP
	testCmd := sampleCommand()
	testCmd.Flags().String("requiredFlag", "", "a required flag")
	testCmd.Flags().String("notRequiredFlag", "", "a not required flag")
	testCmd.MarkFlagRequired("requiredFlag")
	var args = []string{}
	RegisterViperFlags(&testCmd, args)
	// TEST
	assert.Equal(testCmd.Flag("requiredFlag").Annotations[cobra.BashCompOneRequiredFlag], []string{"true"})
	assert.Equal(testCmd.Flag("notRequiredFlag").Annotations[cobra.BashCompOneRequiredFlag], []string(nil))
}

func TestRequiredFlagProvided(t *testing.T) {
	assert := assert.New(t)
	var configParams = make(map[string]interface{})
	configParams["requiredFlag"] = "my saved value"
	configDir := writeStubConfig(configParams)
	defer os.RemoveAll(configDir)
	os.Setenv("HOME", configDir)
	// SETUP
	testCmd := sampleCommand()
	testCmd.Flags().String("requiredFlag", "", "a required flag")
	testCmd.Flags().String("notRequiredFlag", "", "a not required flag")
	testCmd.MarkFlagRequired("requiredFlag")
	RegisterViperFlags(&testCmd, []string{})
	// TEST
	// Note there is an implementation difference (but no behavioural difference) between a flag that was never set to required (nil) and one that was turned off.
	assert.Equal(testCmd.Flag("requiredFlag").Annotations[cobra.BashCompOneRequiredFlag], []string{"false"})
	assert.Equal(testCmd.Flag("notRequiredFlag").Annotations[cobra.BashCompOneRequiredFlag], []string(nil))
}
