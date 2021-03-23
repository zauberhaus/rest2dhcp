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

package cmd_test

import (
	"bytes"
	"context"
	"net"
	"os"
	"reflect"
	"testing"
	"time"
	"unsafe"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/zauberhaus/rest2dhcp/background"
	"github.com/zauberhaus/rest2dhcp/client"
	"github.com/zauberhaus/rest2dhcp/cmd"
	"github.com/zauberhaus/rest2dhcp/dhcp"
	"github.com/zauberhaus/rest2dhcp/logger"
	"github.com/zauberhaus/rest2dhcp/mock"
	"github.com/zauberhaus/rest2dhcp/service"
	"gopkg.in/yaml.v3"
)

//go:generate mockgen -source ../background/server.go  -package mock -destination ../mock/server.go

type Check func(cmd *cmd.RootCommand, config *service.ServerConfig)

func TestEnv(t *testing.T) {
	t.Run("EnvVariable", TestEnvVariable)
	t.Run("EnvConfigFile", TestEnvConfigFile)
	t.Run("EnvDHCPServer`", TestEnvDHCPServer)
}

func TestRunVersion(t *testing.T) {
	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 0, 0, 0)

	service.Version = client.NewVersion("123", "456", "789", "abc")

	c := cmd.GetRootCmd()
	setLogger(c, logger)

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

func TestEnvConfigFile(t *testing.T) {
	configfile := "./test_config.yaml"
	os.Setenv("CONFIG", configfile)
	run(t, func(cmd *cmd.RootCommand, config *service.ServerConfig) {
	})
	os.Unsetenv("CONFIG")
}

func TestEnvVariable(t *testing.T) {
	local := net.IP{1, 1, 1, 1}
	remote := net.IP{2, 2, 2, 2}
	relay := net.IP{3, 3, 3, 3}

	configfile := "./test_config.yaml"

	os.Setenv("CONFIG", configfile)
	os.Setenv("MODE", "fritzbox")
	os.Setenv("CLIENT", local.String())
	os.Setenv("SERVER", remote.String())
	os.Setenv("RELAY", relay.String())
	os.Setenv("QUIET", "1")
	os.Setenv("VERBOSE", "1")
	os.Setenv("HOSTNAME", "test")
	os.Setenv("PORT", "1234")
	os.Setenv("KUBECONFIG", "abcdefg")
	os.Setenv("NAMESPACE", "ns001")
	os.Setenv("SERVICE", "svr001")
	os.Setenv("TIMEOUT", "65s")
	os.Setenv("DHCP_SERVER", "MyDHCP")
	os.Setenv("DHCP_TIMEOUT", "31s")
	os.Setenv("RETRY", "43s")
	os.Setenv("ACCESS_LOG", "0")

	run(t, func(cmd *cmd.RootCommand, config *service.ServerConfig) {
		assert.Equal(t, configfile, cmd.GetConfigFile(), "Config file wrong")
		assert.Equal(t, dhcp.Fritzbox, config.Mode, "Mode wrong")
		assert.Equal(t, local, config.Local.To4(), "Local wrong")
		assert.Equal(t, remote, config.Remote.To4(), "Remote wrong")
		assert.Equal(t, relay, config.Relay.To4(), "Relay wrong")
		assert.Equal(t, true, config.Quiet, "Quiet wrong")
		assert.Equal(t, true, config.Verbose, "Verbose wrong")
		assert.Equal(t, "test", config.Hostname, "Hostname wrong")
		assert.Equal(t, uint16(1234), config.Port, "Port wrong")
		assert.Equal(t, "abcdefg", config.KubeConfig.Config, "Kube config wrong")
		assert.Equal(t, "ns001", config.KubeConfig.Namespace, "Namespace wrong")
		assert.Equal(t, "svr001", config.KubeConfig.Service, "Service wrong")
		assert.Equal(t, 65*time.Second, config.Timeout, "Timeout wrong")
		assert.Equal(t, "", config.DHCPServer, "DHCP server wrong")
		assert.Equal(t, 31*time.Second, config.DHCPTimeout, "DHCP timeout wrong")
		assert.Equal(t, 43*time.Second, config.Retry, "Retry wrong")
		assert.Equal(t, false, config.AccessLog, "Access log wrong")
	})

	os.Unsetenv("CONFIG")
	os.Unsetenv("MODE")
	os.Unsetenv("CLIENT")
	os.Unsetenv("SERVER")
	os.Unsetenv("RELAY")
	os.Unsetenv("QUIET")
	os.Unsetenv("VERBOSE")
	os.Unsetenv("LISTEN")
	os.Unsetenv("KUBECONFIG")
	os.Unsetenv("NAMESPACE")
	os.Unsetenv("SERVICE")
	os.Unsetenv("TIMEOUT")
	os.Unsetenv("DHCP_SERVER")
	os.Unsetenv("DHCP_TIMEOUT")
	os.Unsetenv("RETRY")
	os.Unsetenv("ACCESS_LOG")
}

func TestEnvDHCPServer(t *testing.T) {
	os.Setenv("DHCP_SERVER", "localhost")

	run(t, func(cmd *cmd.RootCommand, config *service.ServerConfig) {
		assert.Equal(t, "localhost", config.DHCPServer, "DHCP server wrong")
		assert.Equal(t, false, config.Quiet, "Quiet wrong")
		assert.Equal(t, false, config.Verbose, "Verbose wrong")
		assert.Equal(t, true, config.AccessLog, "Access log wrong")
	})

	os.Unsetenv("DHCP_SERVER")
}

func TestConfigVariable(t *testing.T) {
	local := net.IP{1, 1, 1, 1}
	remote := net.IP{2, 2, 2, 2}
	relay := net.IP{3, 3, 3, 3}

	run(t, func(cmd *cmd.RootCommand, config *service.ServerConfig) {
		assert.Equal(t, local, config.Local.To4(), "Local wrong")
		assert.Equal(t, dhcp.Fritzbox, config.Mode, "Mode wrong")
		assert.Equal(t, remote, config.Remote.To4(), "Remote wrong")
		assert.Equal(t, relay, config.Relay.To4(), "Relay wrong")
		assert.Equal(t, true, config.Quiet, "Quiet wrong")
		assert.Equal(t, true, config.Verbose, "Verbose wrong")
		assert.Equal(t, false, config.AccessLog, "Access log wrong")
		assert.Equal(t, "test", config.Hostname, "Hostname wrong")
		assert.Equal(t, uint16(1234), config.Port, "Port wrong")
		assert.Equal(t, "abcdefg", config.KubeConfig.Config, "Kube config wrong")
		assert.Equal(t, "ns001", config.KubeConfig.Namespace, "Namespace wrong")
		assert.Equal(t, "svr001", config.KubeConfig.Service, "Service wrong")
		assert.Equal(t, 65*time.Second, config.Timeout, "Timeout wrong")
		assert.Equal(t, "", config.DHCPServer, "DHCP server wrong")
		assert.Equal(t, 31*time.Second, config.DHCPTimeout, "DHCP timeout wrong")
		assert.Equal(t, 43*time.Second, config.Retry, "Retry wrong")
	},
		"--mode", "fritzbox",
		"--client", local.String(),
		"--server", remote.String(),
		"--relay", relay.String(),
		"-q",
		"-v",
		"--access-log=false",
		"-H", "test",
		"-p", "1234",
		"--kubeconfig", "abcdefg",
		"--namespace", "ns001",
		"--service", "svr001",
		"-t", "65s",
		"--dhcp-timeout", "31s",
		"--retry", "43s",
	)
}

func TestConfigDHCPServer(t *testing.T) {
	run(t, func(cmd *cmd.RootCommand, config *service.ServerConfig) {
		assert.Equal(t, "localhost", config.DHCPServer, "DHCP server wrong")
		assert.Equal(t, false, config.Quiet, "Quiet wrong")
		assert.Equal(t, false, config.Verbose, "Verbose wrong")
		assert.Equal(t, true, config.AccessLog, "Access log wrong")
	}, "-S", "localhost")
}

func TestConfigFile(t *testing.T) {
	configfile := "./test_config.yaml"

	local := net.IP{11, 11, 11, 11}
	remote := net.IP{22, 22, 22, 22}
	relay := net.IP{33, 33, 33, 33}

	run(t, func(cmd *cmd.RootCommand, config *service.ServerConfig) {
		assert.Equal(t, configfile, cmd.GetConfigFile(), "Config file wrong")
		assert.Equal(t, local, config.Local.To4(), "Local wrong")
		assert.Equal(t, dhcp.Broken, config.Mode, "Mode wrong")
		assert.Equal(t, remote, config.Remote.To4(), "Remote wrong")
		assert.Equal(t, relay, config.Relay.To4(), "Relay wrong")
		assert.Equal(t, true, config.Quiet, "Quiet wrong")
		assert.Equal(t, true, config.Verbose, "Verbose wrong")
		assert.Equal(t, false, config.AccessLog, "Access log wrong")
		assert.Equal(t, "test", config.Hostname, "Hostname wrong")
		assert.Equal(t, uint16(1234), config.Port, "Port wrong")
		assert.Equal(t, "abcdefg", config.KubeConfig.Config, "Kube config wrong")
		assert.Equal(t, "ns001", config.KubeConfig.Namespace, "Namespace wrong")
		assert.Equal(t, "svr001", config.KubeConfig.Service, "Service wrong")
		assert.Equal(t, 13*time.Minute, config.Timeout, "Timeout wrong")
		assert.Equal(t, "", config.DHCPServer, "DHCP server wrong")
		assert.Equal(t, 27*time.Second, config.DHCPTimeout, "DHCP timeout wrong")
		assert.Equal(t, 38*time.Second, config.Retry, "Retry wrong")

	}, "--config", configfile)
}

func run(t *testing.T, check Check, args ...string) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	logger := mock.NewTestLogger()
	//defer logger.Assert(t, 0, 0, 0, 1, 0, 0, 0, 0)

	service.Version = client.NewVersion("123", "456", "789", "abc")
	server := mock.NewMockServer(ctrl)

	c := cmd.GetRootCmd()
	setLogger(c, logger)
	setServer(c, server)

	server.EXPECT().Init(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, args ...interface{}) {
		assert.Len(t, args, 2)
		cfg, ok := args[0].(*service.ServerConfig)
		assert.True(t, ok)
		check(c, cfg)
	}).AnyTimes()

	server.EXPECT().Start(gomock.Any()).DoAndReturn(func(ctx context.Context) chan bool {
		rc := make(chan bool)
		close(rc)
		return rc
	}).AnyTimes()

	server.EXPECT().Done().DoAndReturn(func() chan bool {
		rc := make(chan bool)
		close(rc)
		return rc
	})

	if args == nil {
		args = []string{}
	}
	c.SetArgs(args)

	err := c.Execute()
	assert.NoError(t, err)
}

func setServer(f *cmd.RootCommand, s background.Server) {
	pointerVal := reflect.ValueOf(f)
	val := reflect.Indirect(pointerVal)
	member := val.FieldByName("server")
	ptrToY := unsafe.Pointer(member.UnsafeAddr())
	realPtrToY := (*background.Server)(ptrToY)
	*realPtrToY = s
}

func setLogger(f *cmd.RootCommand, l logger.Logger) {
	pointerVal := reflect.ValueOf(f)
	val := reflect.Indirect(pointerVal)
	member := val.FieldByName("logger")
	ptrToY := unsafe.Pointer(member.UnsafeAddr())
	realPtrToY := (*logger.Logger)(ptrToY)
	*realPtrToY = l
}
