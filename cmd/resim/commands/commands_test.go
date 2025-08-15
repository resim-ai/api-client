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

func TestNormalize_ComponentAliases_MapToComponentId(t *testing.T) {
	assert := assert.New(t)
	// SETUP
	testCmd := sampleCommand()
	// Underlying id flags
	testCmd.Flags().String("batch-id", "", "the batch id")
	testCmd.Flags().String("test-id", "", "the test id")
	// Apply normalization
	RegisterViperFlags(&testCmd, []string{})

	// --batch should map to --batch-id
	args := []string{"--batch", "b-123"}
	err := testCmd.Flags().Parse(args)
	assert.NoError(err)
	assert.Equal("b-123", testCmd.Flags().Lookup("batch-id").Value.String())

	// --test should map to --test-id
	testCmd.Flags().Set("batch-id", "")
	testCmd.Flags().Set("test-id", "")
	args = []string{"--test", "t-456"}
	err = testCmd.Flags().Parse(args)
	assert.NoError(err)
	assert.Equal("t-456", testCmd.Flags().Lookup("test-id").Value.String())
}

func TestNormalize_IdAlias_UniqueTarget(t *testing.T) {
	assert := assert.New(t)
	// SETUP: only one <component>-id exists
	testCmd := sampleCommand()
	testCmd.Flags().String("report-id", "", "the report id")
	RegisterViperFlags(&testCmd, []string{})

	// --id should map uniquely to --report-id
	args := []string{"--id", "r-789"}
	err := testCmd.Flags().Parse(args)
	assert.NoError(err)
	assert.Equal("r-789", testCmd.Flags().Lookup("report-id").Value.String())
}

func TestNormalize_IdAlias_AmbiguousErrors(t *testing.T) {
	assert := assert.New(t)
	// SETUP: two <component>-id flags present → --id should be ambiguous and not map
	testCmd := sampleCommand()
	testCmd.Flags().String("build-id", "", "the build id")
	testCmd.Flags().String("branch-id", "", "the branch id")
	RegisterViperFlags(&testCmd, []string{})

	// --id should not be normalized (ambiguous) and parsing should error on unknown flag
	args := []string{"--id", "x"}
	err := testCmd.Flags().Parse(args)
	assert.Error(err)
}

func TestNormalize_BackCompatAliases(t *testing.T) {
	assert := assert.New(t)
	// SETUP
	testCmd := sampleCommand()
	testCmd.Flags().String("project", "", "the project")
	testCmd.Flags().String("test-id", "", "the test id")
	RegisterViperFlags(&testCmd, []string{})

	// --project-id should map to --project
	args := []string{"--project-id", "my-project"}
	err := testCmd.Flags().Parse(args)
	assert.NoError(err)
	assert.Equal("my-project", testCmd.Flags().Lookup("project").Value.String())

	// --job-id should map to --test-id
	testCmd.Flags().Set("test-id", "")
	args = []string{"--job-id", "job-123"}
	err = testCmd.Flags().Parse(args)
	assert.NoError(err)
	assert.Equal("job-123", testCmd.Flags().Lookup("test-id").Value.String())
}

func TestNormalize_SystemAliases(t *testing.T) {
	assert := assert.New(t)
	// SETUP: systems use --system (name or id). Accept --system-id and --id
	testCmd := sampleCommand()
	testCmd.Flags().String("system", "", "system name or id")
	RegisterViperFlags(&testCmd, []string{})

	// --system-id should map to --system
	args := []string{"--system-id", "sys-1"}
	err := testCmd.Flags().Parse(args)
	assert.NoError(err)
	assert.Equal("sys-1", testCmd.Flags().Lookup("system").Value.String())

	// --id should map to --system when no *-id flags exist
	testCmd.Flags().Set("system", "")
	args = []string{"--id", "sys-2"}
	err = testCmd.Flags().Parse(args)
	assert.NoError(err)
	assert.Equal("sys-2", testCmd.Flags().Lookup("system").Value.String())
}

func TestAliasNormalize_ExhaustiveMappings(t *testing.T) {
	assert := assert.New(t)

	newCmdWithFlags := func(flags []string) cobra.Command {
		cmd := sampleCommand()
		for _, f := range flags {
			cmd.Flags().String(f, "", f+" flag")
		}
		RegisterViperFlags(&cmd, []string{})
		return cmd
	}

	// Back-compat alias mappings
	backCompat := []struct {
		name        string
		defined     []string
		args        []string
		targetFlag  string
		expectValue string
		expectErr   bool
	}{
		{"project-id -> project", []string{"project"}, []string{"--project-id", "p"}, "project", "p", false},
		{"project-name -> project", []string{"project"}, []string{"--project-name", "pn"}, "project", "pn", false},
		{"branch-name -> branch", []string{"branch"}, []string{"--branch-name", "b"}, "branch", "b", false},
		{"job-id -> test-id", []string{"test-id"}, []string{"--job-id", "j"}, "test-id", "j", false},
		{"locations -> location", []string{"location"}, []string{"--locations", "loc"}, "location", "loc", false},
	}
	for _, tc := range backCompat {
		cmd := newCmdWithFlags(tc.defined)
		err := cmd.Flags().Parse(tc.args)
		if tc.expectErr {
			assert.Error(err, tc.name)
			continue
		}
		assert.NoError(err, tc.name)
		assert.Equal(tc.expectValue, cmd.Flags().Lookup(tc.targetFlag).Value.String(), tc.name)
	}

	// Known components should accept --<component> and --<component>-id; --id routes when unambiguous
	components := []string{"batch", "report", "sweep", "build", "test", "branch"}
	for _, comp := range components {
		idFlag := comp + "-id"
		// Case: only id flag exists; --<component> maps to --<component>-id
		cmd := newCmdWithFlags([]string{idFlag})
		val := comp + "-val1"
		err := cmd.Flags().Parse([]string{"--" + comp, val})
		assert.NoError(err, comp+": --component -> --component-id")
		assert.Equal(val, cmd.Flags().Lookup(idFlag).Value.String(), comp+": value mapped")

		// Case: --id maps to the single id flag
		cmd = newCmdWithFlags([]string{idFlag})
		val = comp + "-val2"
		err = cmd.Flags().Parse([]string{"--id", val})
		assert.NoError(err, comp+": --id -> --component-id")
		assert.Equal(val, cmd.Flags().Lookup(idFlag).Value.String(), comp+": value mapped via --id")

		// Case: both base and id flags exist; --<component> maps to --<component>-id
		cmd = newCmdWithFlags([]string{comp, idFlag})
		val = comp + "-val3"
		err = cmd.Flags().Parse([]string{"--" + comp, val})
		assert.NoError(err, comp+": --component with both flags present")
		assert.Equal(val, cmd.Flags().Lookup(idFlag).Value.String(), comp+": value mapped to id flag")
	}

	// Ambiguous --id when multiple *-id flags exist
	cmd := newCmdWithFlags([]string{"build-id", "branch-id"})
	err := cmd.Flags().Parse([]string{"--id", "x"})
	assert.Error(err, "--id should be ambiguous when multiple id flags exist")

	// Systems: base only, no id flag
	cmd = newCmdWithFlags([]string{"system"})
	err = cmd.Flags().Parse([]string{"--system-id", "sys-1"})
	assert.NoError(err, "system-id -> system when id flag missing")
	assert.Equal("sys-1", cmd.Flags().Lookup("system").Value.String(), "system-id mapped to system")
	cmd = newCmdWithFlags([]string{"system"})
	err = cmd.Flags().Parse([]string{"--id", "sys-2"})
	assert.NoError(err, "--id -> system when only base exists")
	assert.Equal("sys-2", cmd.Flags().Lookup("system").Value.String(), "id mapped to system")

	// Systems: both base and id flags exist; --system maps to --system-id
	cmd = newCmdWithFlags([]string{"system", "system-id"})
	err = cmd.Flags().Parse([]string{"--system", "sys-3"})
	assert.NoError(err, "--system -> --system-id when both exist")
	assert.Equal("sys-3", cmd.Flags().Lookup("system-id").Value.String(), "system mapped to system-id")
}
