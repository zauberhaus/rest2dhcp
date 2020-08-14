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

package test_test

import "github.com/zauberhaus/rest2dhcp/client"

// Test constants
const (
	VersionURL = "http://localhost:8080/version"

	BuildDate    = "2020-08-11T10:06:44NZST"
	GitCommit    = "03fd9a8658c81c088fb548cc43b56703e6ee145b"
	GitVersion   = "v0.0.1"
	GitTreeState = "dirty"
)

// NewTestVersion creates version with tests values
func NewTestVersion() *client.Version {
	return client.NewVersion(BuildDate, GitCommit, GitVersion, GitTreeState)
}
