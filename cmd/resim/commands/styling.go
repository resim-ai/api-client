// Enables applying custom ReSim styling to help and usage text.
//
// Cobra provides two methods for doing this:
// 1. Using cobra.Command.SetUsageFunc() allows an arbitrary function to be
// provided to produce the command usage help text. This is powerful, but
// also requires the author of the function to implement all functionality.
// 2. Using cobra.Command.SetUsageTemplate() allows the existing text template
// for usage to be modified or replaced. This gives the author control over the
// contents of usage message and takes advantage of all the built in cobra 
// functionality for generating the data in the text template. Further, text
// templates are quite powerful because they can be used to execute arbitrary 
// functions, including user defined functions. That may be added using:
// cobra.AddTemplateFunc("key", UserFunction). Therefore we follow this approach
// here in defining ReSim's custom styling.
//
// In this file, first the ReSimUsageTemplate is defined. Below this any
// custom template functions are defined. Finally a styling helper:
// ApplyReSimStyle is defined. Style templates are inherited by child commands
// so this function need only be applied once to the root command (rootCmd).

package commands

import (
	"text/template"

	"github.com/spf13/cobra"
	"github.com/fatih/color"
)

var ReSimUsageTemplate string = 
`{{StyleHeading "USAGE"}}{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

{{StyleHeading "ALIASES"}}
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

{{StyleHeading "EXAMPLES"}}
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

{{StyleHeading "AVAILABLE COMMANDS"}}{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{.Title}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}

{{StyleHeading "ADDITIONAL COMMANDS"}}{{range $cmds}}{{if (and (eq .GroupID "") (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

{{StyleHeading "FLAGS"}}
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

{{StyleHeading "GLOBAL FLAGS"}}
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

{{StyleHeading "ADDITIONAL HELP TOPICS"}}
{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`

var templateFuncs = template.FuncMap{
  "StyleHeading": color.New(color.Bold).SprintFunc(),
}

func styleHeading(s string) string {
    return color.New(color.Bold).SprintFunc()(s)
}

func ApplyReSimStyle(cmd *cobra.Command) {
  cobra.AddTemplateFuncs(templateFuncs) 
  cmd.SetUsageTemplate(ReSimUsageTemplate)
}
