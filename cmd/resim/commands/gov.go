package commands

import (
	"fmt"
	"io/fs"
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	govcloudCmd = &cobra.Command{
		Use:   "govcloud",
		Short: "govcloud - enable or disable govcloud mode",
		Long:  ``,
	}
	enableCmd = &cobra.Command{
		Use:   "enable",
		Short: "enable - enables govcloud mode",
		Long:  ``,
		Run:   enableGovcloud,
	}
	disableCmd = &cobra.Command{
		Use:   "disable",
		Short: "disable - disables govcloud mode",
		Long:  ``,
		Run:   disableGovcloud,
	}
)

func init() {
	govcloudCmd.AddCommand(enableCmd)
	govcloudCmd.AddCommand(disableCmd)
	rootCmd.AddCommand(govcloudCmd)
}

func enableGovcloud(ccmd *cobra.Command, args []string) {
	v := readConfigFile()
	v.Set("govcloud", true)
	v.WriteConfigAs(os.ExpandEnv(ConfigPath) + "/resim.yaml")
	fmt.Println("GovCloud mode enabled")
}

func disableGovcloud(ccmd *cobra.Command, args []string) {
	v := readConfigFile()
	v.Set("govcloud", false)
	v.WriteConfigAs(os.ExpandEnv(ConfigPath) + "/resim.yaml")
	fmt.Println("GovCloud mode disabled")
}

func readConfigFile() *viper.Viper {
	// Open the config file as an independent Viper instance. This instance does not have all the flags set.
	// Therefore we can safely save it again without adding any additional flags.
	v := viper.New()
	v.SetConfigName("resim")
	v.SetConfigType("yaml")
	v.AddConfigPath(os.ExpandEnv(ConfigPath))
	if err := v.ReadInConfig(); err != nil {
		switch err.(type) {
		case viper.ConfigFileNotFoundError, *fs.PathError:
		default:
			log.Fatal(fmt.Errorf("error reading config file: %v %T", err, err))
		}
	}
	return v
}
