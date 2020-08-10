/*
Copyright © 2020 Dirk Lembke <dirk@lembke.nz>

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

package main

import (
	"github.com/zauberhaus/rest2dhcp/client"
	"github.com/zauberhaus/rest2dhcp/cmd"
	"github.com/zauberhaus/rest2dhcp/service"
)

var (
	tag       string // git tag used to build the program
	gitCommit string // sha1 revision used to build the program
	buildTime string // when the executable was built
	treeState string // git tree state
)

func main() {
	version := client.NewVersion(buildTime, gitCommit, tag, treeState)
	service.Version = version

	cmd.Execute()
}
