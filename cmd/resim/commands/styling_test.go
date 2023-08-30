package commands

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

const headingRequired string = "REQUIRED"
const headingOptional string = "OPTIONAL"

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
	identifierO := "Easy to find description for optional"
	identifierR := "Easy to find description for required"
	testCmd := helperTestCommand()
	testCmd.Flags().String("optionalflag", "o", identifierO)
	testCmd.Flags().String("requiredflag", "r", identifierR)
	testCmd.MarkFlagRequired("requiredflag")
	usageMsg := testCmd.UsageString()
	// VERIFICATION
	if !strings.Contains(usageMsg, identifierO) {
		t.Fail()
	}
	if !strings.Contains(usageMsg, identifierR) {
		t.Fail()
	}
	if !strings.Contains(usageMsg, headingRequired) {
		t.Fail()
	}
	if !strings.Contains(usageMsg, headingOptional) {
		t.Fail()
	}
}
