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

//go:generate esc -o doc.go -ignore /doc/.*map -pkg service ../doc ../api

package service_test

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"reflect"
	"runtime"
	"strings"
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
	"gopkg.in/yaml.v3"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kube "k8s.io/client-go/kubernetes"
	testclient "k8s.io/client-go/kubernetes/fake"
)

//go:generate mockgen -source ../dhcp/client.go  -package mock -destination ../mock/client.go

var _port int32 = 57011

func getPort() int32 {
	atomic.AddInt32(&_port, 1)
	_port++
	return _port
}

func TestNewServer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	dhcp := mock.NewMockDHCPClient(ctrl)

	dhcp.EXPECT().Start().DoAndReturn(func() chan bool {
		rc := make(chan bool)
		close(rc)
		return rc
	})

	dhcp.EXPECT().Stop()

	port := getPort()

	config := &service.ServerConfig{
		Listen: fmt.Sprintf(":%v", port),
	}
	version := client.NewVersion("now", "123456", "0001", "dirty")

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 3, 1, 0, 0, 1)

	server := service.NewServer(logger)
	assert.NotNil(t, server)

	setDHCPClient(server, dhcp)

	ctx, cancel := context.WithCancel(context.Background())
	server.Init(ctx, config, version)
	<-server.Start(ctx)

	response := request(t, "GET", "http://localhost:"+server.Port()+"/version", map[string]string{
		"Accept": "text/dummy",
	})

	assert.Equal(t, 415, response.StatusCode)

	cancel()
	<-server.Done()
}

func TestNewServerWithKubernetes(t *testing.T) {
	tests := []struct {
		name   string
		svc    *v1.Service
		msgs   []int64
		config *service.ServerConfig
	}{
		{
			name: "ok",
			msgs: []int64{0, 0, 0, 4, 1, 0, 0, 1},
			svc: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "svc001",
				},
				Spec: v1.ServiceSpec{
					ExternalIPs: []string{
						"78.78.78.78",
					},
				},
			},
		},
		{
			name: "missing exernal ip",
			msgs: []int64{0, 1, 0, 3, 1, 0, 0, 1},
			svc: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "svc001",
				},
				Spec: v1.ServiceSpec{
					ExternalIPs: []string{},
				},
			},
		},
		{
			name: "not existing config file",
			msgs: []int64{0, 0, 0, 3, 1, 0, 0, 1},
			svc: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "svc001",
				},
				Spec: v1.ServiceSpec{
					ExternalIPs: []string{},
				},
			},
			config: &service.ServerConfig{
				KubeConfig: &dhcp.KubeServiceConfig{
					Config: "./testdata/dummy.yaml",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			port := getPort()

			config := tt.config
			if config == nil {
				config = &service.ServerConfig{
					Listen: fmt.Sprintf(":%v", port),
					KubeConfig: &dhcp.KubeServiceConfig{
						Config:    "./testdata/config.yaml",
						Namespace: "ns001",
						Service:   "svc001",
					},
				}
			} else {
				config.Listen = fmt.Sprintf(":%v", port)
			}

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			dhcpClient := mock.NewMockDHCPClient(ctrl)

			dhcpClient.EXPECT().Start().DoAndReturn(func() chan bool {
				rc := make(chan bool)
				close(rc)
				return rc
			})

			clientset := testclient.NewSimpleClientset()

			ctx := context.Background()

			ns, err := clientset.CoreV1().Namespaces().Create(ctx, &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns001",
				},
			}, metav1.CreateOptions{})

			assert.NoError(t, err)
			assert.NotNil(t, ns)

			svc, err := clientset.CoreV1().Services(ns.ObjectMeta.Name).Create(ctx, tt.svc, metav1.CreateOptions{})
			assert.NoError(t, err)
			assert.NotNil(t, svc)

			dhcpClient.EXPECT().Stop()

			version := client.NewVersion("now", "123456", "0001", "dirty")

			logger := mock.NewTestLogger()
			defer logger.Assert(t, tt.msgs...)

			server := service.NewServer(logger)
			assert.NotNil(t, server)

			setClientSet(server, clientset)
			setDHCPClient(server, dhcpClient)

			ctx, cancel := context.WithCancel(context.Background())
			server.Init(ctx, config, version)
			<-server.Start(ctx)

			response := request(t, "GET", "http://localhost:"+server.Port()+"/version", map[string]string{
				"Accept": "text/dummy",
			})

			assert.Equal(t, 415, response.StatusCode)

			cancel()
			<-server.Done()
		})
	}
}

func TestNewServerWithKubernetesFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	dhcpClient := mock.NewMockDHCPClient(ctrl)

	dhcpClient.EXPECT().Start().DoAndReturn(func() chan bool {
		rc := make(chan bool)
		close(rc)
		return rc
	})

	clientset := testclient.NewSimpleClientset()

	ctx := context.Background()

	ns, err := clientset.CoreV1().Namespaces().Create(ctx, &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ns001",
		},
	}, metav1.CreateOptions{})

	assert.NoError(t, err)
	assert.NotNil(t, ns)

	svc, err := clientset.CoreV1().Services(ns.ObjectMeta.Name).Create(ctx, &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "svc001",
		},
		Spec: v1.ServiceSpec{
			ExternalIPs: []string{
				//		"78.78.78.78",
			},
		},
	}, metav1.CreateOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, svc)

	dhcpClient.EXPECT().Stop()

	port := getPort()

	config := &service.ServerConfig{
		Listen: fmt.Sprintf(":%v", port),
		KubeConfig: &dhcp.KubeServiceConfig{
			Config:    "./testdata/config.yaml",
			Namespace: "ns001",
			Service:   "svc001",
		},
	}

	version := client.NewVersion("now", "123456", "0001", "dirty")

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 1, 0, 3, 1, 0, 0, 1)

	server := service.NewServer(logger)
	assert.NotNil(t, server)

	setClientSet(server, clientset)
	setDHCPClient(server, dhcpClient)

	ctx, cancel := context.WithCancel(context.Background())
	server.Init(ctx, config, version)
	<-server.Start(ctx)

	response := request(t, "GET", "http://localhost:"+server.Port()+"/version", map[string]string{
		"Accept": "text/dummy",
	})

	assert.Equal(t, 415, response.StatusCode)

	cancel()
	<-server.Done()
}

func TestServer_Version(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 3, 1, 0, 0, 6)
	server, _, cancel := start(t, ctrl, logger)
	port := server.Port()

	tests := []struct {
		name       string
		method     string
		url        string
		header     map[string]string
		statuscode int
		content    string
		result     string
		body       string
		do         interface{}
	}{
		{
			name:   "Invalid Accept Content Type",
			method: "GET",
			url:    "http://localhost:" + port + "/version",
			header: map[string]string{
				"Accept": "text/dummy",
			},
			statuscode: 415,
		},
		{
			name:       "Call API doc",
			method:     "GET",
			url:        "http://localhost:" + port + "/api",
			statuscode: 200,
		},
		{
			name:       "GetVersion_json",
			method:     "GET",
			url:        "http://localhost:" + port + "/version",
			statuscode: 200,
			content:    client.JSON,
			result:     "version.json",
		},
		{
			name:       "GetVersion_yaml",
			method:     "GET",
			url:        "http://localhost:" + port + "/version",
			statuscode: 200,
			content:    client.YAML,
			result:     "version.yaml",
		},
		{
			name:       "GetVersion_xml",
			method:     "GET",
			url:        "http://localhost:" + port + "/version",
			statuscode: 200,
			content:    client.XML,
			result:     "version.xml",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.content != "" {
				if tt.header == nil {
					tt.header = make(map[string]string, 1)
				}

				tt.header["Accept"] = tt.content
			}

			response := request(t, tt.method, tt.url, tt.header)
			if !assert.Equal(t, tt.statuscode, response.StatusCode) {
				t.Fail()
			}

			body, err := ioutil.ReadAll(response.Body)
			if err != nil {
				t.Fatalf("%v", err)
			}

			if tt.statuscode == 200 && tt.result != "" {
				data2, err := readTestData(tt.result)
				if err != nil {
					t.Fatalf("%v: %v", tt.name, err)
				}

				arch := runtime.GOOS + "/" + runtime.GOARCH
				result := strings.Replace(data2, "go1.16", runtime.Version(), 1)
				result = strings.Replace(result, "linux/amd64", arch, 1)

				assert.Equal(t, result, string(body))
			} else if tt.body != "" {
				assert.Equal(t, tt.body, string(body))
			}

		})
	}

	cancel()
	<-server.Done()

}

func TestServer_GetLease(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 3, 1, 0, 0, 4)
	server, dhcpClient, cancel := start(t, ctrl, logger)
	port := server.Port()

	tests := []struct {
		name       string
		method     string
		url        string
		header     map[string]string
		statuscode int
		content    string
		result     string
		body       string
		do         interface{}
	}{
		{
			name:       "GetLease hostname",
			method:     "GET",
			url:        "http://localhost:" + port + "/ip/hostname",
			statuscode: 200,
			content:    client.JSON,
			result:     "lease.json",
			do: func(ctx context.Context, hostname string, chaddr net.HardwareAddr) chan *dhcp.Lease {
				rc := make(chan *dhcp.Lease, 1)
				lease := dhcp.NewLease(layers.DHCPMsgTypeAck, 99, net.HardwareAddr{1, 2, 3, 4, 5, 6}, nil)

				lease.YourClientIP = net.IP{192, 168, 1, 99}

				rc <- lease
				return rc
			},
		},
		{
			name:       "GetLease hostname/mac",
			method:     "GET",
			url:        "http://localhost:" + port + "/ip/hostname/09:08:07:06:05:04",
			statuscode: 200,
			content:    client.JSON,
			result:     "lease2.json",
			do: func(ctx context.Context, hostname string, chaddr net.HardwareAddr) chan *dhcp.Lease {
				rc := make(chan *dhcp.Lease, 1)
				lease := dhcp.NewLease(layers.DHCPMsgTypeAck, 99, chaddr, nil)

				lease.YourClientIP = net.IP{192, 168, 1, 99}

				rc <- lease
				return rc
			},
		},
		{
			name:       "GetLeaseNAK",
			method:     "GET",
			url:        "http://localhost:" + port + "/ip/hostname",
			statuscode: 406,
			content:    client.JSON,
			do: func(ctx context.Context, hostname string, chaddr net.HardwareAddr) chan *dhcp.Lease {
				rc := make(chan *dhcp.Lease, 1)
				lease := dhcp.NewLease(layers.DHCPMsgTypeNak, 99, chaddr, nil)
				lease.SetError(fmt.Errorf("NAK"))

				rc <- lease
				return rc
			},
			body: "NAK\n",
		},
		{
			name:       "GetLeaseFailed",
			method:     "GET",
			url:        "http://localhost:" + port + "/ip/hostname",
			statuscode: 400,
			content:    client.JSON,
			do: func(ctx context.Context, hostname string, chaddr net.HardwareAddr) chan *dhcp.Lease {
				rc := make(chan *dhcp.Lease, 1)
				lease := dhcp.NewLeaseError(fmt.Errorf("TestError"))
				rc <- lease
				return rc
			},
			body: "TestError\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			dhcpClient.EXPECT().GetLease(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(tt.do)

			if tt.content != "" {
				if tt.header == nil {
					tt.header = make(map[string]string, 1)
				}

				tt.header["Accept"] = tt.content
			}

			response := request(t, tt.method, tt.url, tt.header)
			if !assert.Equal(t, tt.statuscode, response.StatusCode) {
				t.Fail()
			}

			body, err := ioutil.ReadAll(response.Body)
			if err != nil {
				t.Fatalf("%v", err)
			}

			if tt.statuscode == 200 && tt.result != "" {

				data2, err := readTestData(tt.result)
				if err != nil {
					t.Fatalf("%v: %v", tt.name, err)
				}

				assert.Equal(t, data2, string(body))
			} else if tt.body != "" {
				assert.Equal(t, tt.body, string(body))
			}

		})
	}

	cancel()
	<-server.Done()

}

func TestServer_GetLease_Invalid(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 3, 1, 0, 0, 2)
	server, _, cancel := start(t, ctrl, logger)
	port := server.Port()

	tests := []struct {
		name       string
		method     string
		url        string
		header     map[string]string
		statuscode int
		content    string
		result     string
		body       string
		do         interface{}
	}{
		{
			name:       "InvalidMacAddress",
			method:     "GET",
			url:        "http://localhost:" + port + "/ip/hostname/01:02:03:04:05_1",
			statuscode: 400,
			content:    client.JSON,
			body:       "address 01:02:03:04:05_1: invalid MAC address\n",
		},
		{
			name:       "InvalidHostname",
			method:     "GET",
			url:        "http://localhost:" + port + "/ip/host_name/01:02:03:04:05:06",
			statuscode: 400,
			content:    client.JSON,
			body:       "Invalid hostname\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			if tt.content != "" {
				if tt.header == nil {
					tt.header = make(map[string]string, 1)
				}

				tt.header["Accept"] = tt.content
			}

			response := request(t, tt.method, tt.url, tt.header)
			if !assert.Equal(t, tt.statuscode, response.StatusCode) {
				t.Fail()
			}

			body, err := ioutil.ReadAll(response.Body)
			if err != nil {
				t.Fatalf("%v", err)
			}

			if tt.statuscode == 200 && tt.result != "" {

				data2, err := readTestData(tt.result)
				if err != nil {
					t.Fatalf("%v: %v", tt.name, err)
				}

				assert.Equal(t, data2, string(body))
			} else if tt.body != "" {
				assert.Equal(t, tt.body, string(body))
			}

		})
	}

	cancel()
	<-server.Done()

}

func TestServer_Renew_Invalid(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 3, 1, 0, 0, 3)
	server, _, cancel := start(t, ctrl, logger)
	port := server.Port()

	tests := []struct {
		name       string
		method     string
		url        string
		header     map[string]string
		statuscode int
		content    string
		result     string
		body       string
		do         interface{}
	}{
		{
			name:       "InvalidMacAddress",
			method:     "GET",
			url:        "http://localhost:" + port + "/ip/hostname/01:02:03:04:05_1/192.168.1.1",
			statuscode: 400,
			content:    client.JSON,
			body:       "address 01:02:03:04:05_1: invalid MAC address\n",
		},
		{
			name:       "InvalidIP",
			method:     "GET",
			url:        "http://localhost:" + port + "/ip/hostname/01:02:03:04:05:06/192.168.1.456",
			statuscode: 400,
			content:    client.JSON,
			body:       "Invalid IP format\n",
		},
		{
			name:       "InvalidHostname",
			method:     "GET",
			url:        "http://localhost:" + port + "/ip/host_name/01:02:03:04:05:06/192.168.1.4",
			statuscode: 400,
			content:    client.JSON,
			body:       "Invalid hostname\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			if tt.content != "" {
				if tt.header == nil {
					tt.header = make(map[string]string, 1)
				}

				tt.header["Accept"] = tt.content
			}

			response := request(t, tt.method, tt.url, tt.header)
			if !assert.Equal(t, tt.statuscode, response.StatusCode) {
				t.Fail()
			}

			body, err := ioutil.ReadAll(response.Body)
			if err != nil {
				t.Fatalf("%v", err)
			}

			if tt.statuscode == 200 && tt.result != "" {

				data2, err := readTestData(tt.result)
				if err != nil {
					t.Fatalf("%v: %v", tt.name, err)
				}

				assert.Equal(t, data2, string(body))
			} else if tt.body != "" {
				assert.Equal(t, tt.body, string(body))
			}

		})
	}

	cancel()
	<-server.Done()

}

func TestServer_Release_Invalid(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 3, 1, 0, 0, 3)
	server, _, cancel := start(t, ctrl, logger)
	port := server.Port()

	tests := []struct {
		name       string
		method     string
		url        string
		header     map[string]string
		statuscode int
		content    string
		result     string
		body       string
		do         interface{}
	}{
		{
			name:       "InvalidMacAddress",
			method:     "DELETE",
			url:        "http://localhost:" + port + "/ip/hostname/01:02:03:04:05_1/192.168.1.1",
			statuscode: 400,
			content:    client.JSON,
			body:       "address 01:02:03:04:05_1: invalid MAC address\n",
		},
		{
			name:       "InvalidIP",
			method:     "DELETE",
			url:        "http://localhost:" + port + "/ip/hostname/01:02:03:04:05:06/192.168.1.456",
			statuscode: 400,
			content:    client.JSON,
			body:       "Invalid IP format\n",
		},
		{
			name:       "InvalidHostname",
			method:     "GET",
			url:        "http://localhost:" + port + "/ip/host_name/01:02:03:04:05:06/192.168.1.4",
			statuscode: 400,
			content:    client.JSON,
			body:       "Invalid hostname\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			if tt.content != "" {
				if tt.header == nil {
					tt.header = make(map[string]string, 1)
				}

				tt.header["Accept"] = tt.content
			}

			response := request(t, tt.method, tt.url, tt.header)
			if !assert.Equal(t, tt.statuscode, response.StatusCode) {
				t.Fail()
			}

			body, err := ioutil.ReadAll(response.Body)
			if err != nil {
				t.Fatalf("%v", err)
			}

			if tt.statuscode == 200 && tt.result != "" {

				data2, err := readTestData(tt.result)
				if err != nil {
					t.Fatalf("%v: %v", tt.name, err)
				}

				assert.Equal(t, data2, string(body))
			} else if tt.body != "" {
				assert.Equal(t, tt.body, string(body))
			}

		})
	}

	cancel()
	<-server.Done()

}

func TestServer_Renew_(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 3, 1, 0, 0, 3)
	server, dhcpClient, cancel := start(t, ctrl, logger)

	tests := []struct {
		name       string
		method     string
		url        string
		header     map[string]string
		statuscode int
		content    string
		result     string
		body       string
		do         interface{}
	}{{
		name:       "Renew",
		method:     "GET",
		url:        "http://localhost:" + server.Port() + "/ip/hostname/01:02:03:04:05:06/192.168.1.99",
		statuscode: 200,
		content:    client.JSON,
		result:     "lease.json",
		do: func(ctx context.Context, hostname string, chaddr net.HardwareAddr, ip net.IP) chan *dhcp.Lease {
			rc := make(chan *dhcp.Lease, 1)
			lease := dhcp.NewLease(layers.DHCPMsgTypeAck, 99, chaddr, nil)

			lease.YourClientIP = ip
			lease.Hostname = hostname

			rc <- lease
			return rc
		},
	},
		{
			name:       "Renew NAK",
			method:     "GET",
			url:        "http://localhost:" + server.Port() + "/ip/hostname/01:02:03:04:05:06/192.168.1.99",
			statuscode: 406,
			do: func(ctx context.Context, hostname string, chaddr net.HardwareAddr, ip net.IP) chan *dhcp.Lease {
				rc := make(chan *dhcp.Lease, 1)
				lease := dhcp.NewLease(layers.DHCPMsgTypeNak, 99, chaddr, nil)
				lease.SetError(fmt.Errorf("NAK"))

				rc <- lease
				return rc
			},
			body: "NAK\n",
		},
		{
			name:       "Renew Fail",
			method:     "GET",
			url:        "http://localhost:" + server.Port() + "/ip/hostname/01:02:03:04:05:06/192.168.1.99",
			statuscode: 400,
			do: func(ctx context.Context, hostname string, chaddr net.HardwareAddr, ip net.IP) chan *dhcp.Lease {
				rc := make(chan *dhcp.Lease, 1)
				lease := dhcp.NewLeaseError(fmt.Errorf("TestError"))
				rc <- lease
				return rc
			},
			body: "TestError\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			dhcpClient.EXPECT().Renew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(tt.do)

			if tt.content != "" {
				if tt.header == nil {
					tt.header = make(map[string]string, 1)
				}

				tt.header["Accept"] = tt.content
			}

			response := request(t, tt.method, tt.url, tt.header)
			if !assert.Equal(t, tt.statuscode, response.StatusCode) {
				t.Fail()
			}

			body, err := ioutil.ReadAll(response.Body)
			if err != nil {
				t.Fatalf("%v", err)
			}

			if tt.statuscode == 200 && tt.result != "" {

				data2, err := readTestData(tt.result)
				if err != nil {
					t.Fatalf("%v: %v", tt.name, err)
				}

				assert.Equal(t, data2, string(body))
			} else if tt.body != "" {
				assert.Equal(t, tt.body, string(body))
			}

		})
	}

	cancel()
	<-server.Done()

}

func TestServer_Release(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 3, 1, 0, 0, 2)
	server, dhcpClient, cancel := start(t, ctrl, logger)
	port := server.Port()

	tests := []struct {
		name       string
		method     string
		url        string
		header     map[string]string
		statuscode int
		content    string
		result     string
		body       string
		do         interface{}
	}{
		{
			name:       "Release",
			method:     "DELETE",
			url:        "http://localhost:" + port + "/ip/hostname/01:02:03:04:05:06/192.168.1.99",
			statuscode: 200,
			content:    client.JSON,
			do: func(ctx context.Context, hostname string, chaddr net.HardwareAddr, ip net.IP) chan error {
				rc := make(chan error, 1)
				rc <- nil
				return rc
			},
			body: "Ok.\n",
		},
		{
			name:       "ReleaseFail",
			method:     "DELETE",
			url:        "http://localhost:" + port + "/ip/hostname/01:02:03:04:05:06/192.168.1.99",
			statuscode: 400,
			content:    client.JSON,
			do: func(ctx context.Context, hostname string, chaddr net.HardwareAddr, ip net.IP) chan error {
				rc := make(chan error, 1)
				rc <- fmt.Errorf("ReleaseFail")
				return rc
			},
			body: "ReleaseFail\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			dhcpClient.EXPECT().Release(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(tt.do)

			if tt.content != "" {
				if tt.header == nil {
					tt.header = make(map[string]string, 1)
				}

				tt.header["Accept"] = tt.content
			}

			response := request(t, tt.method, tt.url, tt.header)
			if !assert.Equal(t, tt.statuscode, response.StatusCode) {
				t.Fail()
			}

			body, err := ioutil.ReadAll(response.Body)
			if err != nil {
				t.Fatalf("%v", err)
			}

			if tt.statuscode == 200 && tt.result != "" {

				data2, err := readTestData(tt.result)
				if err != nil {
					t.Fatalf("%v: %v", tt.name, err)
				}

				assert.Equal(t, data2, string(body))
			} else if tt.body != "" {
				assert.Equal(t, tt.body, string(body))
			}

		})
	}

	cancel()
	<-server.Done()

}

func start(t *testing.T, ctrl *gomock.Controller, logger logger.Logger) (background.Server, *mock.MockDHCPClient, context.CancelFunc) {
	dhcpClient := mock.NewMockDHCPClient(ctrl)

	port := getPort()

	dhcpClient.EXPECT().Start().DoAndReturn(func() chan bool {
		rc := make(chan bool)
		close(rc)
		return rc
	})

	dhcpClient.EXPECT().Stop()

	config := &service.ServerConfig{
		Listen: fmt.Sprintf(":%v", port),
	}
	version := client.NewVersion("now", "123456", "0001", "dirty")

	server := service.NewServer(logger)
	assert.NotNil(t, server)

	setDHCPClient(server, dhcpClient)

	ctx, cancel := context.WithCancel(context.Background())
	server.Init(ctx, config, version)
	<-server.Start(ctx)

	time.Sleep(100 * time.Microsecond)

	return server, dhcpClient, cancel
}

func request(t *testing.T, method string, url string, header map[string]string) *http.Response {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		t.Fatalf("%v", err)
	}

	if header != nil {
		for key, value := range header {
			req.Header.Set(key, value)
		}
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%v", err)
	}

	return resp
}

func checkResult(t *testing.T, resp *http.Response, hostname string, mime client.ContentType) *client.Lease {
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Wrong http status: %v", resp.Status)
	}

	if mime == client.Unknown {
		mime = client.YAML
	}

	value := resp.Header.Get("Content-Type")
	if client.ContentType(value) != mime {
		t.Fatalf("Wrong content type: %v != %v", value, mime)
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("%v", err)
	}

	var result client.Lease
	unmarshal(t, data, &result, mime)

	if result.Hostname != hostname {
		t.Fatalf("Wrong hostname: '%v' != '%v'", result.Hostname, hostname)
	}

	if result.IP == nil {
		t.Fatalf("Invalid return ip")
	}

	if result.Mac == nil {
		t.Fatalf("Empty mac")
	}

	resp.Body.Close()
	return &result
}

func unmarshal(t *testing.T, data []byte, result interface{}, mime client.ContentType) {
	switch mime {
	case client.YAML:
		err := yaml.Unmarshal(data, result)
		if err != nil {
			t.Fatalf("%v", err)
			return
		}
	case client.JSON:
		err := json.Unmarshal(data, result)
		if err != nil {
			t.Fatalf("%v", err)
			return
		}
	case client.XML:
		err := xml.Unmarshal(data, result)
		if err != nil {
			t.Fatalf("%v", err)
			return
		}
	}
}

func readTestData(file string) (string, error) {
	data, err := ioutil.ReadFile("./testdata/" + file)
	if err != nil {
		return "", err
	}

	result := string(data)

	if runtime.GOOS == "windows" {
		result = strings.ReplaceAll(result, "\n", "\r\n")
	}

	return result, nil
}

func setClientSet(server background.Server, c kube.Interface) {
	restServer, ok := server.(*service.RestServer)
	if ok {
		pointerVal := reflect.ValueOf(restServer)
		val := reflect.Indirect(pointerVal)
		member := val.FieldByName("clientset")
		ptrToY := unsafe.Pointer(member.UnsafeAddr())
		realPtrToY := (*kube.Interface)(ptrToY)
		*realPtrToY = c
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
