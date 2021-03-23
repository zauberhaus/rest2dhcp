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

package client_test

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"reflect"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/golang/mock/gomock"
	"github.com/google/gopacket/layers"
	"github.com/stretchr/testify/assert"
	"github.com/zauberhaus/rest2dhcp/background"
	"github.com/zauberhaus/rest2dhcp/client"
	"github.com/zauberhaus/rest2dhcp/dhcp"
	"github.com/zauberhaus/rest2dhcp/logger"
	"github.com/zauberhaus/rest2dhcp/mock"
	"github.com/zauberhaus/rest2dhcp/service"
)

var (
	_port   int32 = 51231
	host          = "http://localhost"
	ip            = net.IP{192, 168, 99, 173}
	netmask       = net.IP{255, 255, 255, 0}
	dns           = net.IP{192, 168, 99, 1}
)

func getPort() uint16 {
	atomic.AddInt32(&_port, 1)
	_port++
	return uint16(_port)
}

func TestClientVersion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 3, 2, 0, 0, 4)

	server, _, cancel := start(t, ctrl, logger)

	testCases := []struct {
		Mime client.ContentType
	}{
		{
			Mime: client.XML,
		},
		{
			Mime: client.YAML,
		},
		{
			Mime: client.JSON,
		},
		{
			Mime: client.Unknown,
		},
	}

	for _, tc := range testCases {
		tc := tc // We run our tests twice one with this line & one without
		t.Run(tc.Mime.String(), func(t *testing.T) {

			cl := client.NewClient(fmt.Sprintf("%s:%v", host, server.Port()))
			cl.ContentType = tc.Mime
			ctx := context.Background()

			version, err := cl.Version(ctx)
			if cl.ContentType == client.Unknown {
				clientError, ok := err.(*client.Error)
				if !ok {
					t.Fatalf("Unexpected error type")
				}

				assert.Equal(t, 415, clientError.Code(), "Unexpected status code")

			} else if assert.NoError(t, err, "client.Version failed") {
				if assert.NotNil(t, version, "Empty version info") {
					assert.Equal(t, getVersion(), version, "Invalid Version info")
				}
			}
		})
	}

	cancel()
	<-server.Done()
}

func TestClient(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 3, 2, 0, 3, 10)

	server, dhcpClient, cancel := start(t, ctrl, logger)

	testCases := []struct {
		Name     string
		Mime     client.ContentType
		Hostname string
		Mac      client.MAC
	}{
		{
			Name:     "Run DHCP workflow via HTTP request with content type XML",
			Mime:     client.XML,
			Hostname: "test-xml",
			Mac:      client.MAC{1, 2, 3, 4, 5, 6},
		},
		{
			Name:     "Run DHCP workflow via HTTP request with content type YAML",
			Mime:     client.YAML,
			Hostname: "test-yaml",
			Mac:      client.MAC{1, 2, 3, 4, 5, 7},
		},
		{
			Name:     "Run DHCP workflow via HTTP request with content type JSON",
			Mime:     client.JSON,
			Hostname: "test-json",
			Mac:      client.MAC{1, 2, 3, 4, 5, 8},
		},
		{
			Name:     "Run DHCP workflow via HTTP request without a content type",
			Mime:     client.Unknown,
			Hostname: "test-unknown",
			Mac:      client.MAC{1, 2, 3, 4, 5, 9},
		},
	}

	for _, tc := range testCases {
		tc := tc // We run our tests twice one with this line & one without
		t.Run(tc.Name, func(t *testing.T) {

			if tc.Mime != client.Unknown {
				dhcpClient.EXPECT().GetLease(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, hostname string, chaddr net.HardwareAddr) chan *dhcp.Lease {
					rc := make(chan *dhcp.Lease, 1)
					var mac net.HardwareAddr = net.HardwareAddr(tc.Mac)
					lease := dhcp.NewLease(layers.DHCPMsgTypeAck, 12345, mac, nil)
					lease.Touch()
					lease.YourClientIP = ip
					lease.Hostname = tc.Hostname
					lease.SetOption(layers.DHCPOptSubnetMask, netmask)
					lease.SetOption(layers.DHCPOptDNS, dns)
					lease.SetOption(layers.DHCPOptRouter, dns)

					buffer := []byte{0, 0, 0, 0}
					binary.BigEndian.PutUint32(buffer, uint32((24 * time.Hour).Seconds()))
					lease.SetOption(layers.DHCPOptLeaseTime, buffer)

					binary.BigEndian.PutUint32(buffer, uint32((12 * time.Hour).Seconds()))
					lease.SetOption(layers.DHCPOptT1, buffer)

					binary.BigEndian.PutUint32(buffer, uint32((6 * time.Hour).Seconds()))
					lease.SetOption(layers.DHCPOptT2, buffer)

					rc <- lease
					return rc
				})

				dhcpClient.EXPECT().Renew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, hostname string, chaddr net.HardwareAddr, ip net.IP) chan *dhcp.Lease {
					rc := make(chan *dhcp.Lease, 1)
					var mac net.HardwareAddr = net.HardwareAddr(tc.Mac)
					lease := dhcp.NewLease(layers.DHCPMsgTypeAck, 12345, mac, nil)
					lease.Touch()
					lease.YourClientIP = ip
					lease.Hostname = tc.Hostname
					lease.SetOption(layers.DHCPOptSubnetMask, netmask)
					lease.SetOption(layers.DHCPOptDNS, dns)
					lease.SetOption(layers.DHCPOptRouter, dns)

					buffer := []byte{0, 0, 0, 0}
					binary.BigEndian.PutUint32(buffer, uint32((24 * time.Hour).Seconds()))
					lease.SetOption(layers.DHCPOptLeaseTime, buffer)

					binary.BigEndian.PutUint32(buffer, uint32((12 * time.Hour).Seconds()))
					lease.SetOption(layers.DHCPOptT1, buffer)

					binary.BigEndian.PutUint32(buffer, uint32((6 * time.Hour).Seconds()))
					lease.SetOption(layers.DHCPOptT2, buffer)

					rc <- lease
					return rc
				})

				dhcpClient.EXPECT().Release(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, hostname string, chaddr net.HardwareAddr, ip net.IP) chan error {
					rc := make(chan error, 1)
					rc <- nil
					return rc
				})
			}

			cl := client.NewClient(fmt.Sprintf("%s:%v", host, server.Port()))
			cl.ContentType = tc.Mime

			ctx := context.Background()

			lease, err := cl.Lease(ctx, tc.Hostname, nil)

			if tc.Mime == client.Unknown {
				clientError, ok := err.(*client.Error)
				if !ok {
					assert.Fail(t, "Unexpected error type")
				}

				assert.Equal(t, 415, clientError.Code(), "Unexpected status code")

				return
			}

			if assert.NoError(t, err) {
				if assert.NotNil(t, lease) {
					assert.Equal(t, ip, lease.IP.To4())
					assert.Equal(t, tc.Mac, lease.Mac)
					assert.Equal(t, tc.Hostname, lease.Hostname)
					assert.Equal(t, netmask, lease.Mask.To4())
					assert.Equal(t, dns, lease.DNS.To4())
					assert.Equal(t, dns, lease.Router.To4())

					assert.Greater(t, 1*time.Second, lease.Expire.Sub(time.Now().Add(24*time.Hour)))
					assert.Greater(t, 1*time.Second, lease.Rebind.Sub(time.Now().Add(12*time.Hour)))
					assert.Greater(t, 1*time.Second, lease.Renew.Sub(time.Now().Add(6*time.Hour)))

					lease, err = cl.Renew(ctx, lease.Hostname, lease.Mac, lease.IP)
					if assert.NoError(t, err) {
						if assert.NotNil(t, lease) {
							assert.Equal(t, ip, lease.IP.To4())
							assert.Equal(t, tc.Mac, lease.Mac)
							assert.Equal(t, tc.Hostname, lease.Hostname)
							assert.Equal(t, netmask, lease.Mask.To4())
							assert.Equal(t, dns, lease.DNS.To4())
							assert.Equal(t, dns, lease.Router.To4())

							assert.Greater(t, 1*time.Second, lease.Expire.Sub(time.Now().Add(24*time.Hour)))
							assert.Greater(t, 1*time.Second, lease.Rebind.Sub(time.Now().Add(12*time.Hour)))
							assert.Greater(t, 1*time.Second, lease.Renew.Sub(time.Now().Add(6*time.Hour)))

							err = cl.Release(ctx, lease.Hostname, lease.Mac, lease.IP)
							assert.NoError(t, err)
						}
					}
				}
			}

			logger.Testf("DONE %v", tc.Name)
		})
	}

	cancel()
	<-server.Done()
}

func TestClient_GetLease_Fail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 3, 2, 0, 0, 1)

	server, dhcpClient, cancel := start(t, ctrl, logger)

	testCases := []struct {
		Name     string
		Server   string
		Mime     client.ContentType
		Hostname string
		Mac      client.MAC
		Code     int
	}{
		{
			Name:     "Response with status code 400",
			Server:   fmt.Sprintf("%s:%v", host, server.Port()),
			Mime:     client.YAML,
			Hostname: "fail-yaml",
			Mac:      client.MAC{1, 2, 3, 4, 5, 6},
			Code:     400,
		},
		{
			Name:     "Unknown host",
			Mime:     client.YAML,
			Hostname: "fail-yaml",
			Mac:      client.MAC{1, 2, 3, 4, 5, 6},
			Code:     -1,
		},
	}

	for _, tc := range testCases {
		tc := tc // We run our tests twice one with this line & one without
		t.Run(tc.Name, func(t *testing.T) {

			if tc.Server != "" {
				dhcpClient.EXPECT().GetLease(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, hostname string, chaddr net.HardwareAddr) chan *dhcp.Lease {
					rc := make(chan *dhcp.Lease, 1)
					lease := dhcp.NewLeaseError(fmt.Errorf("failed"))
					rc <- lease
					return rc
				})
			}

			cl := client.NewClient(tc.Server)
			cl.ContentType = tc.Mime

			ctx := context.Background()

			lease, err := cl.Lease(ctx, tc.Hostname, nil)

			clientError, ok := err.(*client.Error)
			if !ok {
				assert.Fail(t, "Expect an error")
			}

			assert.Equal(t, tc.Code, clientError.Code(), "Unexpected status code")

			assert.Nil(t, lease)
		})
	}

	cancel()
	<-server.Done()
}

func TestClient_Lease_InvalidParameter(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		mac      client.MAC
	}{
		{
			name:     "EmptyHostname",
			hostname: "",
			mac:      client.MAC{1, 2, 3, 4, 5, 6},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := client.NewClient(fmt.Sprintf("%s:%v", host, 12345))
			ctx := context.Background()

			lease, err := c.Lease(ctx, tt.hostname, tt.mac)
			assert.Nil(t, lease)
			assert.Error(t, err)
		})
	}
}

func TestClient_Renew_InvalidParameter(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		mac      client.MAC
		ip       net.IP
	}{
		{
			name: "Empty Hostname",
			mac:  client.MAC{1, 2, 3, 4, 5, 6},
			ip:   net.IP{1, 2, 3, 4},
		},
		{
			name:     "Empty Mac",
			hostname: "hostname",
			ip:       net.IP{1, 2, 3, 4},
		},
		{
			name:     "Empty IP",
			hostname: "hostname",
			mac:      client.MAC{1, 2, 3, 4, 5, 6},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := client.NewClient(fmt.Sprintf("%s:%v", host, 12345))
			ctx := context.Background()

			lease, err := c.Renew(ctx, tt.hostname, tt.mac, tt.ip)
			assert.Nil(t, lease)
			assert.Error(t, err)
		})
	}
}

func TestClient_Release_InvalidParameter(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		mac      client.MAC
		ip       net.IP
	}{
		{
			name: "Empty Hostname",
			mac:  client.MAC{1, 2, 3, 4, 5, 6},
			ip:   net.IP{1, 2, 3, 4},
		},
		{
			name:     "Empty Mac",
			hostname: "hostname",
			ip:       net.IP{1, 2, 3, 4},
		},
		{
			name:     "Empty IP",
			hostname: "hostname",
			mac:      client.MAC{1, 2, 3, 4, 5, 6},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := client.NewClient(fmt.Sprintf("%s:%v", host, 12345))
			ctx := context.Background()

			err := c.Release(ctx, tt.hostname, tt.mac, tt.ip)
			assert.Error(t, err)
		})
	}
}

func TestClientInvalidLease(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 3, 2, 0, 0, 2)

	server, _, cancel := start(t, ctrl, logger)

	testCases := []struct {
		Name     string
		Hostname string
		Mac      client.MAC
		Code     int
	}{
		{
			Name:     "Invalid hostname",
			Hostname: "test_123",
			Mac:      nil,
			Code:     400,
		},
		{
			Name:     "Invalid mac",
			Hostname: "test",
			Mac:      client.MAC{1, 2, 3, 4, 5, 6, 7},
			Code:     400,
		},
		{
			Name:     "Empty hostname",
			Hostname: "",
			Mac:      nil,
			Code:     400,
		},
	}

	for _, tc := range testCases {
		tc := tc // We run our tests twice one with this line & one without
		t.Run(tc.Name, func(t *testing.T) {
			ctx := context.Background()
			cl := client.NewClient(fmt.Sprintf("%s:%v", host, server.Port()))
			_, err := cl.Lease(ctx, tc.Hostname, tc.Mac)

			if assert.Error(t, err) {
				clientError, ok := err.(*client.Error)
				if assert.True(t, ok) {
					assert.Equal(t, tc.Code, clientError.Code())
				}
			}
		})
	}

	cancel()
	<-server.Done()
}

func TestClientInvalidRenew(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 3, 2, 0, 0, 2)

	server, _, cancel := start(t, ctrl, logger)
	port := server.Port()

	testCases := []struct {
		Name     string
		Hostname string
		Mac      client.MAC
		IP       net.IP
		Code     int
	}{
		{
			Name:     "Invalid hostname",
			Hostname: "test_123",
			Mac:      client.MAC{1, 2, 3, 4, 5, 6},
			IP:       net.IP{1, 2, 3, 4},
			Code:     400,
		},
		{
			Name:     "Empty hostname",
			Hostname: "",
			Mac:      client.MAC{1, 2, 3, 4, 5, 6},
			IP:       net.IP{1, 2, 3, 4},
			Code:     400,
		},
		{
			Name:     "Empty mac",
			Hostname: "test",
			Mac:      nil,
			IP:       net.IP{1, 2, 3, 4},
			Code:     400,
		},
		{
			Name:     "Invalid mac",
			Hostname: "test",
			Mac:      client.MAC{1, 2, 3, 4, 5, 6, 7},
			IP:       net.IP{1, 2, 3, 4},
			Code:     400,
		},
		{
			Name:     "Empty IP",
			Hostname: "test",
			Mac:      client.MAC{1, 2, 3, 4, 5, 6},
			IP:       nil,
			Code:     400,
		},
		{
			Name:     "Invalid ip",
			Hostname: "test",
			Mac:      client.MAC{1, 2, 3, 4, 5, 6},
			IP:       net.IP{1, 2, 3, 4, 5},
			Code:     400,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			ctx := context.Background()
			cl := client.NewClient(fmt.Sprintf("%s:%v", host, port))
			_, err := cl.Renew(ctx, tc.Hostname, tc.Mac, tc.IP)

			if assert.Error(t, err) {
				clientError, ok := err.(*client.Error)
				if assert.True(t, ok) {
					assert.Equal(t, tc.Code, clientError.Code())
				}
			}
		})
	}

	cancel()
	<-server.Done()
}

func TestNewVersion(t *testing.T) {
	buildDate := "date"
	gitCommit := "commit"
	tag := "tag"
	treeState := "state"

	version := client.NewVersion(buildDate, gitCommit, tag, treeState)

	version2 := &client.Version{
		BuildDate:    buildDate,
		Compiler:     runtime.Compiler,
		GitCommit:    gitCommit,
		GitTreeState: treeState,
		GitVersion:   tag,
		GoVersion:    runtime.Version(),
		Platform:     fmt.Sprintf("%v/%v", runtime.GOOS, runtime.GOARCH),
	}

	assert.Equal(t, version2, version)
}

func TestLeaseToString(t *testing.T) {
	lease := &client.Lease{
		Hostname: "server",
	}

	txt := lease.String()

	assert.Equal(t, "hostname: server\nmac: \"\"\nip: \"\"\nrenew: 0001-01-01T00:00:00Z\nrebind: 0001-01-01T00:00:00Z\nexpire: 0001-01-01T00:00:00Z\n", txt)
}

func TestClientError(t *testing.T) {
	err := client.NewError(404, "not found")
	assert.Equal(t, 404, err.Code())
	assert.Equal(t, "not found", err.Msg())
	assert.Equal(t, "(404 Not Found) not found", err.Error())
}

func start(t *testing.T, ctrl *gomock.Controller, logger logger.Logger) (background.Server, *mock.MockDHCPClient, context.CancelFunc) {
	dhcpClient := mock.NewMockDHCPClient(ctrl)

	dhcpClient.EXPECT().Start().DoAndReturn(func() chan bool {
		rc := make(chan bool)
		close(rc)
		return rc
	})

	dhcpClient.EXPECT().Stop()

	config := &service.ServerConfig{
		Verbose: true,
		Port:    getPort(),
	}

	server := service.NewServer(logger)
	assert.NotNil(t, server)

	setDHCPClient(server, dhcpClient)

	ctx, cancel := context.WithCancel(context.Background())
	server.Init(ctx, config, getVersion())
	<-server.Start(ctx)

	time.Sleep(100 * time.Microsecond)

	return server, dhcpClient, cancel
}

func getVersion() *client.Version {
	return &client.Version{
		BuildDate:    "01-01-2001",
		Compiler:     "myCompiler",
		GitCommit:    "abcdefg",
		GitTreeState: "very dirty",
		GitVersion:   "nix",
		GoVersion:    runtime.Version(),
		Platform:     fmt.Sprintf("%v/%v", runtime.GOOS, runtime.GOARCH),
	}
}

func setDHCPClient(server background.Server, c dhcp.DHCPClient) {
	restServer, ok := server.(*service.RestServer)
	if ok {
		pointerVal := reflect.ValueOf(restServer)
		val := reflect.Indirect(pointerVal)
		member := val.FieldByName("client")
		ptrToY := unsafe.Pointer(member.UnsafeAddr())
		realPtrToY := (*dhcp.DHCPClient)(ptrToY)
		*realPtrToY = c
	}
}
