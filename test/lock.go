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

import (
	"fmt"
	"os/user"

	"github.com/zbiljic/go-filelock"
)

// NewServerLock creates and locks a file lock
func NewServerLock() filelock.TryLockerSafe {
	tmp := "default"
	u, err := user.Current()
	if err == nil {
		tmp = fmt.Sprint(u.Uid)
	}

	fl, err := filelock.New("/tmp/rest2dhcp-test-" + tmp)
	if err != nil {
		panic(err)
	}

	err = fl.Lock()
	if err != nil {
		panic(err)
	}

	return fl
}
