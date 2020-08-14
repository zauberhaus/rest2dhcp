/*
Copyright © 2020 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/zauberhaus/rest2dhcp/dhcp"
	"github.com/zauberhaus/rest2dhcp/service"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "rest2dhcp",
	Short: "A REST webservice gateway to a DHCP server",
	Run:   service.RunServer,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// GetRootCmd returns the cobra root command
func GetRootCmd() *cobra.Command {
	return rootCmd
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.rest2dhcp.yaml)")

	rootCmd.Flags().IPP("server", "s", nil, "DHCP server ip (autodetect)")
	rootCmd.Flags().IPP("client", "c", nil, "Local IP for DHCP relay client (autodetect)")
	rootCmd.Flags().IPP("relay", "r", nil, "Relay IP for DHCP relay client (client IP)")
	rootCmd.Flags().StringP("listen", "l", ":8080", "Address of the web service")
	rootCmd.Flags().StringP("mode", "m", "auto", "DHCP connection mode: "+strings.Join(dhcp.AllConnectionTypes, "|"))
	rootCmd.Flags().DurationP("timeout", "t", 30*time.Second, "Service query timeout")
	rootCmd.Flags().DurationP("retry", "x", 15*time.Second, "DHCP retry time")
	rootCmd.Flags().DurationP("dhcp-timeout", "d", 5*time.Second, "DHCP query timeout")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".test1" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".rest2dhcp")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
