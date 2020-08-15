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
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/zauberhaus/rest2dhcp/client"
	"github.com/zauberhaus/rest2dhcp/dhcp"
	"github.com/zauberhaus/rest2dhcp/service"
	"gopkg.in/yaml.v2"
)

// TestServer starts the server for tests
type TestServer struct {
	started bool
}

func (s *TestServer) check() bool {
	resp, err := http.Get(VersionURL)
	if err != nil {
		return true
	}

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Can't read version: %s (%v)", resp.Status, resp.StatusCode)
		os.Exit(1)
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Can't read version: %s", err)
		os.Exit(1)
	}

	var info client.Version
	err = yaml.Unmarshal(data, &info)
	if err != nil {
		fmt.Println(string(data))
		fmt.Printf("Invalid version info: %s", err)
		os.Exit(1)
	}

	service.Version = &info

	return false
}

func (s *TestServer) setup() (*service.Server, context.CancelFunc) {
	local := net.ParseIP(os.Getenv("LOCAL"))
	remote := net.ParseIP(os.Getenv("REMOTE"))
	relay := net.ParseIP(os.Getenv("RELAY"))

	mode := dhcp.AutoDetect
	mode.Parse(os.Getenv("MODE"))

	config := service.ServerConfig{
		Local:       local,
		Remote:      remote,
		Relay:       relay,
		Mode:        mode,
		Listen:      ":8080",
		Timeout:     30 * time.Second,
		DHCPTimeout: 3 * time.Second,
		Retry:       15 * time.Second,
	}

	server := service.NewServer(&config, NewTestVersion())
	service.Version = server.Info

	ctx, cancel := context.WithCancel(context.Background())
	server.Start(ctx)
	time.Sleep(100 * time.Millisecond)
	return server, cancel
}

// Run checks weather a server is running and starts a new server if needed
// For usage in MainTest
func (s *TestServer) Run(m *testing.M) {
	s.started = s.check()

	if s.started {
		fl := NewServerLock()
		defer fl.Unlock()

		server, cancel := s.setup()
		code := m.Run()
		cancel()
		<-server.Done
		fmt.Println("Done.")
		os.Exit(code)
	} else {
		code := m.Run()
		os.Exit(code)
	}
}

// IsStarted returns true if the TestServer has to start a new server
func (s *TestServer) IsStarted() bool {
	return s.started
}

// GetMode returns the dhcp.ConnectionType
func (s *TestServer) GetMode() dhcp.ConnectionType {
	if service.Version != nil {
		return service.Version.Mode
	}

	return dhcp.AutoDetect
}
