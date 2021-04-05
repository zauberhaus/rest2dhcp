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
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/zauberhaus/rest2dhcp/background"
	"github.com/zauberhaus/rest2dhcp/dhcp"
	"github.com/zauberhaus/rest2dhcp/logger"
	"github.com/zauberhaus/rest2dhcp/service"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

const (
	defaultConfigFilename = "rest2dhcp"
)

// RootCommand is a root cobra.Command with extra fields
type RootCommand struct {
	cobra.Command

	cfgFile string
	kube    dhcp.KubeServiceConfig
	config  service.ServerConfig
	server  background.Server
	logger  logger.Logger
	viper   *viper.Viper
}

// GetRootCmd returns the cobra root command
func GetRootCmd() *RootCommand {
	var rootCmd *RootCommand

	logger := logger.New()

	rootCmd = &RootCommand{
		logger: logger,
		viper:  viper.New(),
		Command: cobra.Command{
			Use:   "rest2dhcp",
			Short: "A REST web service gateway to a DHCP server",
			PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
				return rootCmd.initializeConfig(cmd)
			},
			Run: func(cmd *cobra.Command, args []string) {

				if rootCmd.config.Remote == nil {
					if rootCmd.config.DHCPServer != "" {
						ips, err := net.LookupIP(rootCmd.config.DHCPServer)
						if err != nil {
							logger.Errorf("Name or service '%s' not known: %v", rootCmd.config.DHCPServer, err)
						}

						for _, ip := range ips {
							if ip.To4() != nil {
								rootCmd.config.Remote = ip
								break
							}
						}
					}
				} else {
					rootCmd.config.DHCPServer = ""
				}

				rootCmd.config.Mode = dhcp.AutoDetect
				modetxt, err := cmd.Flags().GetString("mode")
				if err == nil {
					err = rootCmd.config.Mode.Parse(modetxt)
					if err != nil {
						rootCmd.logger.Errorf("Error: %v\n\n", err)
						cmd.Usage()
						os.Exit(1)
					}
				}

				term := make(chan os.Signal, 1)
				signal.Notify(term, os.Interrupt, syscall.SIGINT)

				ctx, cancel := context.WithCancel(cmd.Context())
				defer cancel()

				rootCmd.config.KubeConfig = &rootCmd.kube

				if rootCmd.server == nil {
					server := service.NewServer(rootCmd.logger)
					rootCmd.server = server
				}

				rootCmd.server.Init(ctx, &rootCmd.config, service.Version)
				<-rootCmd.server.Start(ctx)

				select {
				case s := <-term:
					rootCmd.logger.Infof("Got %v", s.String())
					signal.Reset(syscall.SIGINT)
					cancel()
					<-rootCmd.server.Done()
				case <-rootCmd.server.Done():
				}

				rootCmd.logger.Info("Done.")
			},
		},
	}

	addVersionCmd(&rootCmd.Command, logger)

	rootCmd.init()

	return rootCmd
}

func (r *RootCommand) init() {
	r.PersistentFlags().StringVar(&r.cfgFile, "config", "", "Config file (default is $HOME/"+defaultConfigFilename+".yaml)")

	r.Flags().IPVarP(&r.config.Remote, "server", "s", nil, "DHCP server ip")
	r.Flags().StringVarP(&r.config.DHCPServer, "dhcp-server", "S", "", "DHCP server name")
	r.Flags().IPVarP(&r.config.Local, "client", "c", nil, "Local IP for DHCP relay client")
	r.Flags().IPVarP(&r.config.Relay, "relay", "r", nil, "Relay IP for DHCP relay client")

	r.Flags().StringVarP(&r.config.BaseURL, "base-url", "b", "", "Base URL for the swagger ui")
	r.Flags().StringVarP(&r.config.Hostname, "hostname", "H", "", "Hostname to listen on")
	r.Flags().Uint16VarP(&r.config.Port, "port", "p", 8080, "Port to listen on")
	r.Flags().StringP("mode", "m", "auto", "DHCP connection mode: "+strings.Join(dhcp.AllConnectionTypes, ", "))

	r.Flags().DurationVarP(&r.config.Timeout, "timeout", "t", 30*time.Second, "Service query timeout")
	r.Flags().DurationVarP(&r.config.Retry, "retry", "x", 15*time.Second, "DHCP retry time")
	r.Flags().DurationVarP(&r.config.DHCPTimeout, "dhcp-timeout", "d", 5*time.Second, "DHCP query timeout")

	r.Flags().BoolVarP(&r.config.Verbose, "verbose", "v", false, "Verbose messages")
	r.Flags().BoolVarP(&r.config.Quiet, "quiet", "q", false, "Only access log messages")
	r.Flags().BoolVarP(&r.config.AccessLog, "access-log", "a", true, "Print access log messages")

	if home := r.homeDir(); home != "" {
		r.Flags().StringVarP(&r.kube.Config, "kubeconfig", "k", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		r.Flags().StringVarP(&r.kube.Config, "kubeconfig", "k", "", "(optional) absolute path to the kubeconfig file")
	}

	r.Flags().StringVarP(&r.kube.Namespace, "namespace", "n", "default", "Kubernetes namespace")
	r.Flags().StringVarP(&r.kube.Service, "service", "K", "", "Kubernetes service name")
}

func (r *RootCommand) initializeConfig(cmd *cobra.Command) error {
	if r.cfgFile != "" {
		// Use config file from the flag.
		r.viper.SetConfigFile(r.cfgFile)
	} else {
		tmp := os.Getenv("CONFIG")
		if tmp != "" {
			// Use config file from env variables.
			r.cfgFile = tmp
			r.viper.SetConfigFile(r.cfgFile)
		} else {

			// Find home directory.
			home, err := homedir.Dir()
			if err != nil {
				r.logger.Errorf("Get homedir: %v", err)
				os.Exit(1)
			}

			r.viper.AddConfigPath(home)
			r.viper.SetConfigName(".rest2dhcp")
		}
	}

	r.viper.AutomaticEnv()

	// If a config file is found, read it in.
	if err := r.viper.ReadInConfig(); err == nil {
		r.logger.Infof("Using config file: %v", r.viper.ConfigFileUsed())

		r.viper.WatchConfig()
		r.viper.OnConfigChange(func(e fsnotify.Event) {
			r.logger.Infof("Config file changed: %v", e.Name)
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
			r.viper.BindEnv(f.Name, envVar)
		}

		// push values from config file to cobra
		if !f.Changed && r.viper.IsSet(f.Name) {
			val := r.viper.Get(f.Name)
			cmd.Flags().Set(f.Name, fmt.Sprintf("%v", val))
		}
	})
}

func (r *RootCommand) homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

func (r *RootCommand) GetConfigFile() string {
	return r.cfgFile
}
