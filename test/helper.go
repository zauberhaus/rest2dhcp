package test

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/zauberhaus/rest2dhcp/client"
	"github.com/zauberhaus/rest2dhcp/dhcp"
	"github.com/zauberhaus/rest2dhcp/service"
	"github.com/zbiljic/go-filelock"
	"gopkg.in/yaml.v2"
)

const (
	VersionURL = "http://localhost:8080/version"

	BuildDate    = "2020-08-11T10:06:44NZST"
	GitCommit    = "03fd9a8658c81c088fb548cc43b56703e6ee145b"
	GitVersion   = "v0.0.1"
	GitTreeState = "dirty"
)

type TestServer struct {
	mode    dhcp.ConnectionType
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

	var info client.VersionInfo
	err = yaml.Unmarshal(data, &info)
	if err != nil {
		fmt.Println(string(data))
		fmt.Printf("Invalid version info: %s", err)
		os.Exit(1)
	}

	s.mode = info.ServiceVersion.Mode

	return false
}

func (s *TestServer) setup() (*service.Server, context.CancelFunc) {
	server := service.NewServer(nil, nil, nil, dhcp.AutoDetect, ":8080", 30*time.Second, 3*time.Second, 15*time.Second, client.NewVersion(BuildDate, GitCommit, GitVersion, GitTreeState))
	ctx, cancel := context.WithCancel(context.Background())
	server.Start(ctx)
	time.Sleep(100 * time.Millisecond)
	return server, cancel
}

func (s *TestServer) Run(m *testing.M) {
	s.started = s.check()

	if s.started {
		fl, err := filelock.New("/tmp/rest2dhcp-lock")
		if err != nil {
			panic(err)
		}

		err = fl.Lock()
		if err != nil {
			panic(err)
		}

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

func (s *TestServer) IsStarted() bool {
	return s.started
}

func (s *TestServer) GetMode() dhcp.ConnectionType {
	return s.mode
}
