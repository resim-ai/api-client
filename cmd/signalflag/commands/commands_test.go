package commands

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var matchContext = mock.MatchedBy(func(maybeContext any) bool {
	if _, ok := maybeContext.(context.Context); ok {
		return true
	}
	return false
})

func sampleCommand() cobra.Command {
	var testCmd = cobra.Command{
		Use:           "signalflag",
		Short:         "signalflag - Command Line Interface",
		Long:          ``,
		SilenceErrors: true,
		SilenceUsage:  true,
		PreRun:        RegisterViperFlagsAndSetClient,
	}
	return testCmd
}

func writeStubConfig(params map[string]interface{}) string {
	// Writes a viper config to a temporary directory and returns the path
	tempDir, _ := os.MkdirTemp(os.TempDir(), "signalflag-")
	os.MkdirAll(filepath.Join(tempDir, ".signalflag"), os.ModePerm)
	os.Setenv("HOME", tempDir)
	v := viper.New()
	v.MergeConfigMap(params)
	v.WriteConfigAs(ConfigFilePath())
	return tempDir
}

func TestGetConfigDir(t *testing.T) {
	t.Run("uses current directory when it exists", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		os.Mkdir(filepath.Join(home, ".signalflag"), 0700)
		os.Mkdir(filepath.Join(home, ".resim"), 0700)
		dir, err := GetConfigDir()
		assert.NoError(t, err)
		assert.Equal(t, filepath.Join(home, ".signalflag"), dir)
		assert.Equal(t, "signalflag", ConfigFileNameFor(dir))
		assert.Equal(t, filepath.Join(home, ".signalflag", "signalflag.yaml"), ConfigFilePath())
	})
	t.Run("falls back to legacy directory", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		os.Mkdir(filepath.Join(home, ".resim"), 0700)
		dir, err := GetConfigDir()
		assert.NoError(t, err)
		assert.Equal(t, filepath.Join(home, ".resim"), dir)
		assert.Equal(t, "resim", ConfigFileNameFor(dir))
		assert.Equal(t, filepath.Join(home, ".resim", "resim.yaml"), ConfigFilePath())
	})
	t.Run("creates current directory when neither exists", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		dir, err := GetConfigDir()
		assert.NoError(t, err)
		assert.Equal(t, filepath.Join(home, ".signalflag"), dir)
		assert.DirExists(t, dir)
	})
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
