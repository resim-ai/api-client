package commands

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

var headingRequired string = "REQUIRED"
var headingOptional string = "OPTIONAL"

func helperTestCommand() cobra.Command {
	var testCmd = cobra.Command{
		Use:           "resim",
		Short:         "resim - Command Line Interface for ReSim",
		Long:          ``,
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	ApplyReSimStyle(&testCmd)
	return testCmd
}

func TestNoFlags(t *testing.T) {
	// SETUP
	testCmd := helperTestCommand()
	usageMsg := testCmd.UsageString()
	// VERIFICATION
	// Contains "Usage"
	if !strings.Contains(usageMsg, "USAGE") {
		t.Fail()
	}
	// Contains no flag headings
	if strings.Contains(usageMsg, headingRequired) {
		t.Fail()
	}
	if strings.Contains(usageMsg, headingOptional) {
		t.Fail()
	}
}

func TestOptionalFlags(t *testing.T) {
	// SETUP
	identifier := "Easy to find description for optional"
	testCmd := helperTestCommand()
	testCmd.Flags().String("optionalflag", "o", identifier)
	usageMsg := testCmd.UsageString()
	// VERIFICATION
	if !strings.Contains(usageMsg, identifier) {
		t.Fail()
	}
	if strings.Contains(usageMsg, headingRequired) {
		t.Fail()
	}
	if !strings.Contains(usageMsg, headingOptional) {
		t.Fail()
	}
}

func TestRequiredFlags(t *testing.T) {
	// SETUP
	identifier := "Easy to find description for required"
	testCmd := helperTestCommand()
	testCmd.Flags().String("requiredflag", "r", identifier)
	testCmd.MarkFlagRequired("requiredflag")
	usageMsg := testCmd.UsageString()
	// VERIFICATION
	if !strings.Contains(usageMsg, identifier) {
		t.Fail()
	}
	if !strings.Contains(usageMsg, headingRequired) {
		t.Fail()
	}
	if strings.Contains(usageMsg, headingOptional) {
		t.Fail()
	}
}

func TestBothFlags(t *testing.T) {
	// SETUP
	identifier_o := "Easy to find description for optional"
	identifier_r := "Easy to find description for required"
	testCmd := helperTestCommand()
	testCmd.Flags().String("optionalflag", "o", identifier_o)
	testCmd.Flags().String("requiredflag", "r", identifier_r)
	testCmd.MarkFlagRequired("requiredflag")
	usageMsg := testCmd.UsageString()
	// VERIFICATION
	if !strings.Contains(usageMsg, identifier_o) {
		t.Fail()
	}
	if !strings.Contains(usageMsg, identifier_r) {
		t.Fail()
	}
	if !strings.Contains(usageMsg, headingRequired) {
		t.Fail()
	}
	if !strings.Contains(usageMsg, headingOptional) {
		t.Fail()
	}
}
