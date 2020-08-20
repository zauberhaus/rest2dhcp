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
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/zauberhaus/rest2dhcp/dhcp"
	"github.com/zauberhaus/rest2dhcp/service"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

var cfgFile string

const (
	defaultConfigFilename = "rest2dhcp"
)

// RootCommand is a root cobra.Command with extra fields
type RootCommand struct {
	cobra.Command

	config service.ServerConfig
	server *service.Server
}

// GetRootCmd returns the cobra root command
func GetRootCmd() *RootCommand {
	var rootCmd *RootCommand

	rootCmd = &RootCommand{
		Command: cobra.Command{
			Use:   "rest2dhcp",
			Short: "A REST web service gateway to a DHCP server",
			PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
				return rootCmd.initializeConfig(cmd)
			},
			Run: func(cmd *cobra.Command, args []string) {

				rootCmd.config.Mode = dhcp.AutoDetect

				modetxt := viper.GetString("mode")
				err := rootCmd.config.Mode.Parse(modetxt)
				if err != nil {
					fmt.Printf("Error: %v\n\n", err)
					cmd.Usage()
					os.Exit(1)
				}

				done := make(chan os.Signal, 1)
				signal.Notify(done, os.Interrupt, syscall.SIGINT)

				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				server := service.NewServer(&rootCmd.config, service.Version)
				server.Start(ctx)

				rootCmd.server = server

				s := <-done
				log.Printf("Got %v", s.String())

				signal.Reset(syscall.SIGINT)

				<-server.Done
				log.Println("Done.")

			},
		},
	}

	addVersionCmd(&rootCmd.Command)

	rootCmd.init()

	return rootCmd
}

// GetServer returns the server for tests
func (r *RootCommand) GetServer() *service.Server {
	return r.server
}

func (r *RootCommand) init() {
	r.PersistentFlags().StringVar(&cfgFile, "config", "", "Config file (default is $HOME/"+defaultConfigFilename+".yaml)")

	r.Flags().IPVarP(&r.config.Remote, "server", "s", nil, "DHCP server ip")
	r.Flags().IPVarP(&r.config.Local, "client", "c", nil, "Local IP for DHCP relay client")
	r.Flags().IPVarP(&r.config.Relay, "relay", "r", nil, "Relay IP for DHCP relay client")

	r.Flags().StringVarP(&r.config.Listen, "listen", "l", ":8080", "Address of the web service")
	r.Flags().StringP("mode", "m", "auto", "DHCP connection mode: "+strings.Join(dhcp.AllConnectionTypes, ", "))
	viper.BindPFlag("mode", r.Flags().Lookup("mode"))

	r.Flags().DurationVarP(&r.config.Timeout, "timeout", "t", 30*time.Second, "Service query timeout")
	r.Flags().DurationVarP(&r.config.Retry, "retry", "x", 15*time.Second, "DHCP retry time")
	r.Flags().DurationVarP(&r.config.DHCPTimeout, "dhcp-timeout", "d", 5*time.Second, "DHCP query timeout")

	r.Flags().BoolVarP(&r.config.Verbose, "verbose", "v", false, "Verbose messages")
	r.Flags().BoolVarP(&r.config.Quiet, "quiet", "q", false, "Only access log messages")

}

func (r *RootCommand) initializeConfig(cmd *cobra.Command) error {
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

		viper.AddConfigPath(home)
		viper.SetConfigName(".rest2dhcp")
	}

	viper.AutomaticEnv()

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())

		viper.WatchConfig()
		viper.OnConfigChange(func(e fsnotify.Event) {
			fmt.Println("Config file changed:", e.Name)
		})
	} else {
		// file not found isn't an error
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return err
		}
	}

	r.bindAllFlags(cmd)

	return nil
}

func (r *RootCommand) bindAllFlags(cmd *cobra.Command) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {

		// bind extra Env names
		if strings.Contains(f.Name, "-") {
			envVar := strings.ToUpper(strings.ReplaceAll(f.Name, "-", "_"))
			viper.BindEnv(f.Name, envVar)
		}

		// push values from config file to cobra
		if !f.Changed && viper.IsSet(f.Name) {
			val := viper.Get(f.Name)
			cmd.Flags().Set(f.Name, fmt.Sprintf("%v", val))
		}
	})
}
