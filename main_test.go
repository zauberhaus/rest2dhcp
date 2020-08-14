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

package main_test

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/zauberhaus/rest2dhcp/client"
	"github.com/zauberhaus/rest2dhcp/cmd"
	"github.com/zauberhaus/rest2dhcp/service"
	test_test "github.com/zauberhaus/rest2dhcp/test"
	"gopkg.in/yaml.v2"
)

func TestRunVersion(t *testing.T) {
	old := os.Stdout // keep backup of the real stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	service.Version = test_test.NewTestVersion()
	cmd.VersionCmd.Run(nil, nil)

	outC := make(chan []byte)
	// copy the output in a separate goroutine so printing can't block indefinitely
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		outC <- buf.Bytes()
	}()

	// back to normal state
	w.Close()
	os.Stdout = old // restoring the real stdout
	out := <-outC

	var info client.Version
	err := yaml.Unmarshal(out, &info)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, &info, service.Version, "Invalid value")
}

func TestRunServer(t *testing.T) {
	pid := syscall.Getpid()
	done := make(chan bool)

	service.Version = test_test.NewTestVersion()

	fl := test_test.NewServerLock()
	defer fl.Unlock()

	go func() {
		fmt.Println("Start server...")
		c := cmd.GetRootCmd()
		service.RunServer(c, nil)
		close(done)
	}()

	time.Sleep(5 * time.Second)

	resp, err := http.Get("http://localhost:8080/version")
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Can't read version: %s (%v)", resp.Status, resp.StatusCode)
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Can't read version: %s", err)
	}

	var info client.Version
	err = yaml.Unmarshal(data, &info)
	if err != nil {
		fmt.Println(string(data))
		t.Fatalf("Invalid version info: %s", err)
	}

	assert.Equal(t, &info, service.Version, "Invalid version info")

	fmt.Printf("Send SIGINT to %v", pid)
	syscall.Kill(pid, syscall.SIGINT)

	<-done
	fmt.Println("Done.", pid)
}
