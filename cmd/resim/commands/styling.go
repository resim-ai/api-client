package commands

import (
	"fmt"
	"strings"
	"text/template"
	"unicode"

	"github.com/spf13/cobra"
)

func trimRightSpace(s string) string {
	return strings.TrimRightFunc(s, unicode.IsSpace)
}

var templateFuncs = template.FuncMap{
	"trimTrailingWhitespaces": trimRightSpace,
	"gt":                      cobra.Gt,
	"eq":                      cobra.Eq,
}

func ResimUsage(c *cobra.Command) error {
  fmt.Println("testing 123")
//  t := template.New("top")
//  t.Funcs(templateFuncs)
//  template.Must(t.Parse(c.UsageTemplate()))
//  err := t.Execute(c.OutOrStderr(), c)
//  c.PrintErrln(err);
//  return err
  return nil
}
