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

package cmd_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/zauberhaus/rest2dhcp/client"
	"github.com/zauberhaus/rest2dhcp/cmd"
	"github.com/zauberhaus/rest2dhcp/dhcp"
	"github.com/zauberhaus/rest2dhcp/service"
	helper_test "github.com/zauberhaus/rest2dhcp/test"
	"golang.org/x/net/context"
	"gopkg.in/yaml.v2"
)

func TestMain(m *testing.M) {
	fl := helper_test.NewServerLock()
	defer fl.Unlock()

	code := m.Run()
	os.Exit(code)
}

func TestRunVersion(t *testing.T) {

	service.Version = helper_test.NewTestVersion()

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

	service.Version = helper_test.NewTestVersion()

	c := cmd.GetRootCmd()
	c.SetArgs([]string{"-v"})

	r := runner{}
	started, done := r.start(t, c)

	if assert.True(t, <-started) {
		resp, err := http.Get(helper_test.VersionURL)
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

		assert.Equal(t, service.Version, &info, "Invalid version info")

		r.stop(t)

		<-done
	}

	fmt.Println("Done.")

}

func TestRunServerArgs(t *testing.T) {
	testCases := []struct {
		Name  string
		Args  []string
		Check func(t *testing.T, s *service.Server)
	}{
		{
			Name: "Mode",
			Args: []string{"-m", "udp"},
			Check: func(t *testing.T, s *service.Server) {
				assert.Equal(t, s.Config.Mode, dhcp.UDP)
			},
		},
		{
			Name: "IP",
			Args: []string{"-s", "3.3.3.3", "-c", "127.0.0.1", "-r", "4.4.4.4", "-l", "127.0.0.1:8080"},
			Check: func(t *testing.T, s *service.Server) {
				assert.Equal(t, s.Config.Remote.To4(), net.IP{3, 3, 3, 3})
				assert.Equal(t, s.Config.Relay.To4(), net.IP{4, 4, 4, 4})
				assert.Equal(t, s.Config.Local.To4(), net.IP{127, 0, 0, 1})
				assert.Equal(t, s.Config.Listen, "127.0.0.1:8080")
			},
		},
		{
			Name: "Timeout",
			Args: []string{"-t", "2m", "-x", "13s", "-d", "5368ms"},
			Check: func(t *testing.T, s *service.Server) {
				assert.Equal(t, s.Config.Timeout, 2*time.Minute)
				assert.Equal(t, s.Config.Retry, 13*time.Second)
				assert.Equal(t, s.Config.DHCPTimeout, 5368*time.Millisecond)
				assert.Equal(t, false, s.Config.Verbose, "Verbose should be false")
			},
		},
		{
			Name: "LogLevel",
			Args: []string{"-v"},
			Check: func(t *testing.T, s *service.Server) {
				assert.Equal(t, true, s.Config.Verbose)
				assert.Equal(t, true, s.Config.Quiet)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			c := cmd.GetRootCmd()
			c.SetArgs(append(tc.Args, "-q"))

			r := runner{}
			started, done := r.start(t, c)

			if assert.True(t, <-started) {
				server := c.GetServer()
				if assert.NotNil(t, server) {
					if assert.NotNil(t, tc.Check) {
						tc.Check(t, server)
					}
				}

				r.stop(t)

				<-done
			}

			fmt.Println("Done.")
		})
	}
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
				assert.Equal(t, s.Config.Mode, dhcp.Broken)
			},
		},
		{
			Name: "LocalRemote",
			Env: map[string]string{
				"LISTEN": "127.0.0.1:8080",
				"CLIENT": "127.0.0.1",
				"SERVER": "10.199.178.131",
				"MODE":   "dual",
			},
			Check: func(t *testing.T, s *service.Server) {
				assert.Equal(t, s.Config.Local.To4(), net.IP{127, 0, 0, 1})
				assert.Equal(t, s.Config.Local.To4(), net.IP{127, 0, 0, 1})
				assert.Equal(t, s.Config.Listen, "127.0.0.1:8080")
				assert.Equal(t, s.Config.Mode, dhcp.Dual)
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
			for k, v := range tc.Env {
				os.Setenv(k, v)
			}

			c := cmd.GetRootCmd()
			c.SetArgs([]string{"-q"})

			r := runner{}
			started, done := r.start(t, c)

			if assert.True(t, <-started) {
				server := c.GetServer()
				if assert.NotNil(t, server) {
					if assert.NotNil(t, tc.Check) {
						tc.Check(t, server)
					}
				}

				r.stop(t)

				<-done
			}

			fmt.Println("Done.")

			for k := range tc.Env {
				os.Unsetenv(k)
			}
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
				assert.Equal(t, dhcp.Broken, s.Config.Mode)
				assert.Equal(t, 13*time.Minute, s.Config.Timeout)
				assert.Equal(t, 27*time.Second, s.Config.DHCPTimeout)
				assert.Equal(t, 38*time.Second, s.Config.Retry)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {

			c := cmd.GetRootCmd()
			c.SetArgs([]string{"-q", "--config", "test_config.yaml"})

			r := runner{}
			started, done := r.start(t, c)

			if assert.True(t, <-started) {
				server := c.GetServer()
				if assert.NotNil(t, server) {
					if assert.NotNil(t, tc.Check) {
						tc.Check(t, server)
					}
				}

				r.stop(t)

				<-done
			}

			fmt.Println("Done.")
		})

	}
}

type runner struct {
	pid int
}

func (r *runner) start(t *testing.T, c *cmd.RootCommand) (chan bool, chan bool) {
	started := make(chan bool)
	done := make(chan bool)

	go func() {

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

		go func() {
			fmt.Printf("Start server %v", t.Name())
			r.pid = syscall.Getpid()
			err := c.Execute()
			assert.NoError(t, err)
			cancel()
			close(done)
		}()

		client := http.Client{}

		for {
			req, _ := http.NewRequestWithContext(ctx, "GET", helper_test.VersionURL, nil)
			_, err := client.Do(req)
			if err == nil {
				started <- true
				break
			}

			if urlError, ok := err.(*url.Error); ok {
				if urlError.Timeout() || urlError.Err == context.Canceled {
					fmt.Printf(": %v\n", err)
					r.stop(t)
					started <- false
					break
				}
			}

			time.Sleep(100 * time.Millisecond)
			fmt.Print(".")
		}

	}()

	return started, done
}

func (r *runner) stop(t *testing.T) {
	if r.pid > 0 {
		syscall.Kill(r.pid, syscall.SIGINT)
	}
}
