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
	"log"

	"github.com/spf13/cobra"
	"github.com/zauberhaus/rest2dhcp/service"
	"gopkg.in/yaml.v3"
)

// VersionCmd represents the version command
var VersionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show the version info",
	Run: func(cmd *cobra.Command, args []string) {

		data, err := yaml.Marshal(service.Version)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println(string(data))
	},
}

func init() {
	rootCmd.AddCommand(VersionCmd)
}
