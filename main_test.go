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
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/zauberhaus/rest2dhcp/client"
	"github.com/zauberhaus/rest2dhcp/cmd"
	"github.com/zauberhaus/rest2dhcp/dhcp"
	"github.com/zauberhaus/rest2dhcp/service"
	test_test "github.com/zauberhaus/rest2dhcp/test"
	"gopkg.in/yaml.v2"
)

func TestRunVersion(t *testing.T) {

	service.Version = test_test.NewTestVersion()

	c := cmd.GetRootCmd()
	c.SetArgs([]string{"version"})
	output := &bytes.Buffer{}

	c.SetOut(output)
	err := c.Execute()
	if assert.NoError(t, err) {
		var info client.Version
		err := yaml.Unmarshal(output.Bytes(), &info)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, service.Version, &info, "Invalid value")
	}
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
		c.SetArgs([]string{})
		err := c.Execute()
		assert.NoError(t, err)
		close(done)
	}()

	time.Sleep(1 * time.Second)

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

func TestRunServerEnv(t *testing.T) {
	testCases := []struct {
		Name  string
		Env   map[string]string
		Check func(t *testing.T, s *service.Server)
	}{
		{
			Name: "Relay",
			Env: map[string]string{
				"RELAY": "1.2.3.4",
			},
			Check: func(t *testing.T, s *service.Server) {
				assert.Equal(t, s.Config.Relay.To4(), net.IP{1, 2, 3, 4})
			},
		},
		{
			Name: "Mode",
			Env: map[string]string{
				"MODE": "broken",
			},
			Check: func(t *testing.T, s *service.Server) {
				assert.Equal(t, s.Config.Mode, dhcp.BrokenRelay)
			},
		},
		{
			Name: "LocalRemote",
			Env: map[string]string{
				"LISTEN": "127.0.0.1:8888",
				"CLIENT": "127.0.0.1",
				"SERVER": "1.1.1.1",
				"MODE":   "packet",
			},
			Check: func(t *testing.T, s *service.Server) {
				assert.Equal(t, s.Config.Local.To4(), net.IP{127, 0, 0, 1})
				assert.Equal(t, s.Config.Local.To4(), net.IP{127, 0, 0, 1})
				assert.Equal(t, s.Config.Listen, "127.0.0.1:8888")
				assert.Equal(t, s.Config.Mode, dhcp.Relay)
			},
		},
		{
			Name: "Timeouts",
			Env: map[string]string{
				"TIMEOUT":      "1m",
				"RETRY":        "20s",
				"DHCP_TIMEOUT": "10s",
			},
			Check: func(t *testing.T, s *service.Server) {
				assert.Equal(t, s.Config.Timeout, 1*time.Minute)
				assert.Equal(t, s.Config.Retry, 20*time.Second)
				assert.Equal(t, s.Config.DHCPTimeout, 10*time.Second)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			pid := syscall.Getpid()
			done := make(chan bool)

			fl := test_test.NewServerLock()
			defer fl.Unlock()

			for k, v := range tc.Env {
				os.Setenv(k, v)
			}

			c := cmd.GetRootCmd()
			c.SetArgs([]string{})

			go func() {
				fmt.Println("Start server...")
				err := c.Execute()
				assert.NoError(t, err)
				close(done)
			}()

			time.Sleep(1 * time.Second)

			server := c.GetServer()
			if assert.NotNil(t, server) {
				if assert.NotNil(t, tc.Check) {
					tc.Check(t, server)
				}
			}

			fmt.Printf("Send SIGINT to %v\n", pid)
			syscall.Kill(pid, syscall.SIGINT)

			<-done
			fmt.Println("Done.")

			for k, _ := range tc.Env {
				os.Unsetenv(k)
			}

			fmt.Println("RELAY: " + os.Getenv("RELAY"))
		})

	}
}

func TestRunServerConfigFile(t *testing.T) {
	testCases := []struct {
		Name     string
		Filename string
		Check    func(t *testing.T, s *service.Server)
	}{
		{
			Name:     "test_config",
			Filename: "test_config.yaml",
			Check: func(t *testing.T, s *service.Server) {
				assert.Equal(t, net.IP{127, 0, 0, 1}, s.Config.Local.To4())
				assert.Equal(t, net.IP{2, 2, 2, 2}, s.Config.Remote.To4())
				assert.Equal(t, net.IP{3, 3, 3, 3}, s.Config.Relay.To4())
				assert.Equal(t, dhcp.BrokenRelay, s.Config.Mode)
				assert.Equal(t, 13*time.Minute, s.Config.Timeout)
				assert.Equal(t, 27*time.Second, s.Config.DHCPTimeout)
				assert.Equal(t, 38*time.Second, s.Config.Retry)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			pid := syscall.Getpid()
			done := make(chan bool)

			fl := test_test.NewServerLock()
			defer fl.Unlock()

			c := cmd.GetRootCmd()
			c.SetArgs([]string{"--config", tc.Filename})

			go func() {
				fmt.Println("Start server...")
				err := c.Execute()
				assert.NoError(t, err)
				close(done)
			}()

			time.Sleep(1 * time.Second)

			server := c.GetServer()
			if assert.NotNil(t, server) {
				if assert.NotNil(t, tc.Check) {
					tc.Check(t, server)
				}
			}

			fmt.Printf("Send SIGINT to %v\n", pid)
			syscall.Kill(pid, syscall.SIGINT)

			<-done
			fmt.Println("Done.")
		})

	}
}
