package commands

import (
	"io"
	"time"

	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"
)

type Spinner struct {
	spinner *spinner.Spinner
	ccmd *cobra.Command
}

func NewSpinner(ccmd *cobra.Command) *Spinner {
	return &Spinner{
		spinner: spinner.New(spinner.CharSets[14], 100 * time.Millisecond),
		ccmd: ccmd,
	}
}

func (s *Spinner) Start(msg string) {
	s.spinner.HideCursor = false
	s.spinner.Suffix = " " + msg
	if s.ccmd.Flag(logGithubKey) != nil && s.ccmd.Flag(logGithubKey).Value.String() == "true" {
		s.spinner.Writer = io.Discard
	}
	s.spinner.Start()
}

func (s *Spinner) Stop(finalMsg string) {
	s.spinner.FinalMSG = finalMsg
	s.spinner.Stop()
}

func (s *Spinner) Update(msg string) {
	s.spinner.Suffix = " " + msg
}
