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
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"reflect"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"unsafe"

	"github.com/golang/mock/gomock"
	"github.com/google/gopacket/layers"
	"github.com/stretchr/testify/assert"
	"github.com/zauberhaus/rest2dhcp/client"
	"github.com/zauberhaus/rest2dhcp/dhcp"
	"github.com/zauberhaus/rest2dhcp/kubernetes"
	"github.com/zauberhaus/rest2dhcp/logger"
	"github.com/zauberhaus/rest2dhcp/mock"
	"github.com/zauberhaus/rest2dhcp/service"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kube "k8s.io/client-go/kubernetes"
	testclient "k8s.io/client-go/kubernetes/fake"
)

//go:generate mockgen -source ../dhcp/client.go  -package mock -destination ../mock/client.go

var _port int32 = 50000 + int32(rand.Intn(10000))

func getPort() uint16 {
	atomic.AddInt32(&_port, 1)
	_port++
	return uint16(_port)
}

func TestNewServer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	dhcp := mock.NewMockDHCPClient(ctrl)

	dhcp.EXPECT().Start().DoAndReturn(func() chan error {
		rc := make(chan error)
		close(rc)
		return rc
	})

	dhcp.EXPECT().Stop()

	port := getPort()

	config := &service.ServerConfig{
		Hostname: "localhost",
		Port:     port,
	}
	version := client.NewVersion("now", "123456", "0001", "dirty")

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 4, 1, 0, 0, 1)

	server := service.NewServer(logger)
	assert.NotNil(t, server)

	setDHCPClient(server, dhcp)

	ctx, cancel := context.WithCancel(context.Background())
	server.Init(ctx, config, version)
	<-server.Start(ctx)

	response := request(t, "GET", getRequestUrl(server, "/version"), map[string]string{
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
			msgs: []int64{0, 0, 0, 5, 1, 0, 0, 1},
			svc: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "svc001",
				},
				Status: v1.ServiceStatus{
					LoadBalancer: v1.LoadBalancerStatus{
						Ingress: []v1.LoadBalancerIngress{
							{IP: "78.78.78.78"},
						},
					},
				},
			},
		},
		{
			name: "missing exernal ip",
			msgs: []int64{1, 1, 0, 4, 1, 0, 0, 1},
			svc: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "svc001",
				},
				Status: v1.ServiceStatus{
					LoadBalancer: v1.LoadBalancerStatus{
						Ingress: []v1.LoadBalancerIngress{},
					},
				},
			},
		},
		{
			name: "not existing config file",
			msgs: []int64{0, 0, 0, 4, 1, 0, 0, 1},
			svc: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "svc001",
				},
				Status: v1.ServiceStatus{
					LoadBalancer: v1.LoadBalancerStatus{
						Ingress: []v1.LoadBalancerIngress{
							{IP: "78.78.78.78"},
						},
					},
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
					Port: port,
					KubeConfig: &dhcp.KubeServiceConfig{
						Config:    "./testdata/config.yaml",
						Namespace: "ns001",
						Service:   "svc001",
					},
				}
			} else {
				config.Port = port
			}

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			dhcpClient := mock.NewMockDHCPClient(ctrl)

			dhcpClient.EXPECT().Start().DoAndReturn(func() chan error {
				rc := make(chan error)
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

			client, err := getMockKubeClient(clientset, logger)
			assert.NoError(t, err)

			setKubeClient(server, client)
			setDHCPClient(server, dhcpClient)

			ctx, cancel := context.WithCancel(context.Background())
			server.Init(ctx, config, version)
			<-server.Start(ctx)

			response := request(t, "GET", fmt.Sprintf("http://localhost:%v/version", +server.Port()), map[string]string{
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

	dhcpClient.EXPECT().Start().DoAndReturn(func() chan error {
		rc := make(chan error)
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
		Port: port,
		KubeConfig: &dhcp.KubeServiceConfig{
			Config:    "./testdata/config.yaml",
			Namespace: "ns001",
			Service:   "svc001",
		},
	}

	version := client.NewVersion("now", "123456", "0001", "dirty")

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 1, 1, 0, 4, 1, 0, 0, 1)

	server := service.NewServer(logger)
	assert.NotNil(t, server)

	client, err := getMockKubeClient(clientset, logger)
	assert.NoError(t, err)

	setKubeClient(server, client)
	setDHCPClient(server, dhcpClient)

	ctx, cancel := context.WithCancel(context.Background())
	server.Init(ctx, config, version)
	<-server.Start(ctx)

	response := request(t, "GET", fmt.Sprintf("http://localhost:%v/version", +server.Port()), map[string]string{
		"Accept": "text/dummy",
	})

	assert.Equal(t, 415, response.StatusCode)

	cancel()
	<-server.Done()
}

func TestServer_Requests(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 5, 1, 0, 0, 9)
	server, _, cancel := start(t, ctrl, logger)

	tests := []struct {
		name       string
		method     string
		url        string
		header     map[string]string
		statuscode int
		content    client.ContentType
		result     string
		body       string
		follow     bool
		do         interface{}
	}{
		{
			name:   "Invalid Accept Content Type",
			method: "GET",
			url:    getRequestUrl(server, "/version"),
			header: map[string]string{
				"Accept": "text/dummy",
			},
			statuscode: 415,
		},
		{
			name:       "Call api",
			method:     "GET",
			url:        getRequestUrl(server, "/api"),
			statuscode: http.StatusFound,
		},
		{
			name:       "Call metrics",
			method:     "GET",
			url:        getRequestUrl(server, "/metrics"),
			statuscode: http.StatusOK,
		},
		{
			name:       "Call api",
			method:     "GET",
			url:        getRequestUrl(server, "/"),
			statuscode: http.StatusFound,
		},
		{
			name:       "Call swagger file",
			method:     "GET",
			url:        getRequestUrl(server, "/api/swagger.yaml"),
			statuscode: http.StatusOK,
		},
		{
			name:       "Call doc",
			method:     "GET",
			url:        getRequestUrl(server, "/doc/"),
			statuscode: http.StatusOK,
		},
		{
			name:       "GetVersion_json",
			method:     "GET",
			url:        getRequestUrl(server, "/version"),
			statuscode: 200,
			content:    client.JSON,
			result:     "version.json",
		},
		{
			name:       "GetVersion_yaml",
			method:     "GET",
			url:        getRequestUrl(server, "/version"),
			statuscode: 200,
			content:    client.YAML,
			result:     "version.yaml",
		},
		{
			name:       "GetVersion_xml",
			method:     "GET",
			url:        getRequestUrl(server, "/version"),
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

				tt.header["Accept"] = string(tt.content)
			}

			fmt.Printf("Request %s: %v %v\n", tt.name, tt.method, tt.url)
			response := request(t, tt.method, tt.url, tt.header, tt.follow)
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

				tmp := string(body)

				assert.Equal(t, result, tmp)
			} else if tt.body != "" {
				assert.Equal(t, tt.body, string(body))
			} else if tt.statuscode == 200 {
				assert.Greater(t, len(body), 0)
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
	defer logger.Assert(t, 0, 0, 0, 4, 1, 0, 0, 5)
	server, dhcpClient, cancel := start(t, ctrl, logger)

	tests := []struct {
		name       string
		method     string
		url        string
		header     map[string]string
		statuscode int
		content    client.ContentType
		result     string
		body       string
		do         interface{}
	}{
		{
			name:       "GetLease hostname",
			method:     "GET",
			url:        getRequestUrl(server, "/ip/hostname"),
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
			url:        getRequestUrl(server, "/ip/hostname/09:08:07:06:05:04"),
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
			url:        getRequestUrl(server, "/ip/hostname"),
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
			url:        getRequestUrl(server, "/ip/hostname"),
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
		{
			name:       "GetLeaseTimeput",
			method:     "GET",
			url:        getRequestUrl(server, "/ip/hostname"),
			statuscode: 408,
			content:    client.JSON,
			do: func(ctx context.Context, hostname string, chaddr net.HardwareAddr) chan *dhcp.Lease {
				rc := make(chan *dhcp.Lease, 1)
				rc <- nil
				return rc
			},
			body: "Request Timeout\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			dhcpClient.EXPECT().GetLease(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(tt.do)

			if tt.content != "" {
				if tt.header == nil {
					tt.header = make(map[string]string, 1)
				}

				tt.header["Accept"] = string(tt.content)
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
	defer logger.Assert(t, 0, 0, 0, 4, 1, 0, 0, 2)
	server, _, cancel := start(t, ctrl, logger)

	tests := []struct {
		name       string
		method     string
		url        string
		header     map[string]string
		statuscode int
		content    client.ContentType
		result     string
		body       string
		do         interface{}
	}{
		{
			name:       "InvalidMacAddress",
			method:     "GET",
			url:        getRequestUrl(server, "/ip/hostname/01:02:03:04:05_1"),
			statuscode: 400,
			content:    client.JSON,
			body:       "address 01:02:03:04:05_1: invalid MAC address\n",
		},
		{
			name:       "InvalidHostname",
			method:     "GET",
			url:        getRequestUrl(server, "/ip/host_name/01:02:03:04:05:06"),
			statuscode: 400,
			content:    client.JSON,
			body:       "invalid hostname\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			if tt.content != "" {
				if tt.header == nil {
					tt.header = make(map[string]string, 1)
				}

				tt.header["Accept"] = string(tt.content)
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
	defer logger.Assert(t, 0, 0, 0, 4, 1, 0, 0, 3)
	server, _, cancel := start(t, ctrl, logger)

	tests := []struct {
		name       string
		method     string
		url        string
		header     map[string]string
		statuscode int
		content    client.ContentType
		result     string
		body       string
		do         interface{}
	}{
		{
			name:       "InvalidMacAddress",
			method:     "GET",
			url:        getRequestUrl(server, "/ip/hostname/01:02:03:04:05_1/192.168.1.1"),
			statuscode: 400,
			content:    client.JSON,
			body:       "address 01:02:03:04:05_1: invalid MAC address\n",
		},
		{
			name:       "InvalidIP",
			method:     "GET",
			url:        getRequestUrl(server, "/ip/hostname/01:02:03:04:05:06/192.168.1.456"),
			statuscode: 400,
			content:    client.JSON,
			body:       "invalid IP format\n",
		},
		{
			name:       "InvalidHostname",
			method:     "GET",
			url:        getRequestUrl(server, "/ip/host_name/01:02:03:04:05:06/192.168.1.4"),
			statuscode: 400,
			content:    client.JSON,
			body:       "invalid hostname\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			if tt.content != "" {
				if tt.header == nil {
					tt.header = make(map[string]string, 1)
				}

				tt.header["Accept"] = string(tt.content)
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
	defer logger.Assert(t, 0, 0, 0, 4, 1, 0, 0, 3)
	server, _, cancel := start(t, ctrl, logger)

	tests := []struct {
		name       string
		method     string
		url        string
		header     map[string]string
		statuscode int
		content    client.ContentType
		result     string
		body       string
		do         interface{}
	}{
		{
			name:       "InvalidMacAddress",
			method:     "DELETE",
			url:        getRequestUrl(server, "/ip/hostname/01:02:03:04:05_1/192.168.1.1"),
			statuscode: 400,
			content:    client.JSON,
			body:       "address 01:02:03:04:05_1: invalid MAC address\n",
		},
		{
			name:       "InvalidIP",
			method:     "DELETE",
			url:        getRequestUrl(server, "/ip/hostname/01:02:03:04:05:06/192.168.1.456"),
			statuscode: 400,
			content:    client.JSON,
			body:       "invalid IP format\n",
		},
		{
			name:       "InvalidHostname",
			method:     "GET",
			url:        getRequestUrl(server, "/ip/host_name/01:02:03:04:05:06/192.168.1.4"),
			statuscode: 400,
			content:    client.JSON,
			body:       "invalid hostname\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			if tt.content != "" {
				if tt.header == nil {
					tt.header = make(map[string]string, 1)
				}

				tt.header["Accept"] = string(tt.content)
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

func TestServer_Renew(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 4, 1, 0, 0, 4)
	server, dhcpClient, cancel := start(t, ctrl, logger)

	tests := []struct {
		name       string
		method     string
		url        string
		header     map[string]string
		statuscode int
		content    client.ContentType
		result     string
		body       string
		do         interface{}
	}{{
		name:       "Renew",
		method:     "GET",
		url:        getRequestUrl(server, "/ip/hostname/01:02:03:04:05:06/192.168.1.99"),
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
			url:        getRequestUrl(server, "/ip/hostname/01:02:03:04:05:06/192.168.1.99"),
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
			url:        getRequestUrl(server, "/ip/hostname/01:02:03:04:05:06/192.168.1.99"),
			statuscode: 400,
			do: func(ctx context.Context, hostname string, chaddr net.HardwareAddr, ip net.IP) chan *dhcp.Lease {
				rc := make(chan *dhcp.Lease, 1)
				lease := dhcp.NewLeaseError(fmt.Errorf("TestError"))
				rc <- lease
				return rc
			},
			body: "TestError\n",
		},
		{
			name:       "Renew Timeout",
			method:     "GET",
			url:        getRequestUrl(server, "/ip/hostname/01:02:03:04:05:06/192.168.1.99"),
			statuscode: 408,
			do: func(ctx context.Context, hostname string, chaddr net.HardwareAddr, ip net.IP) chan *dhcp.Lease {
				rc := make(chan *dhcp.Lease, 1)
				rc <- nil
				return rc
			},
			body: "Request Timeout\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			dhcpClient.EXPECT().Renew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(tt.do)

			if tt.content != "" {
				if tt.header == nil {
					tt.header = make(map[string]string, 1)
				}

				tt.header["Accept"] = string(tt.content)
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
	defer logger.Assert(t, 0, 0, 0, 4, 1, 0, 0, 2)
	server, dhcpClient, cancel := start(t, ctrl, logger)

	tests := []struct {
		name       string
		method     string
		url        string
		header     map[string]string
		statuscode int
		content    client.ContentType
		result     string
		body       string
		do         interface{}
	}{
		{
			name:       "Release",
			method:     "DELETE",
			url:        getRequestUrl(server, "/ip/hostname/01:02:03:04:05:06/192.168.1.99"),
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
			url:        getRequestUrl(server, "/ip/hostname/01:02:03:04:05:06/192.168.1.99"),
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

				tt.header["Accept"] = string(tt.content)
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

func start(t *testing.T, ctrl *gomock.Controller, logger logger.Logger) (service.Server, *mock.MockDHCPClient, context.CancelFunc) {
	dhcpClient := mock.NewMockDHCPClient(ctrl)

	port := getPort()

	dhcpClient.EXPECT().Start().DoAndReturn(func() chan error {
		rc := make(chan error)
		close(rc)
		return rc
	})

	dhcpClient.EXPECT().Stop()

	config := &service.ServerConfig{
		Port: port,
	}
	version := client.NewVersion("now", "123456", "0001", "dirty")

	server := service.NewServer(logger)
	assert.NotNil(t, server)

	setDHCPClient(server, dhcpClient)

	ctx, cancel := context.WithCancel(context.Background())
	server.Init(ctx, config, version)
	<-server.Start(ctx)

	return server, dhcpClient, cancel
}

func request(t *testing.T, method string, url string, header map[string]string, follow ...bool) *http.Response {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		t.Fatalf("%v", err)
	}

	for key, value := range header {
		req.Header.Set(key, value)
	}

	client := &http.Client{}

	if len(follow) > 0 && !follow[0] {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%v", err)
	}

	return resp
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

func getRequestUrl(server service.Server, url string) string {
	return fmt.Sprintf("http://localhost:%v%v", server.Port(), url)
}

func getMockKubeClient(c kube.Interface, logger logger.Logger) (kubernetes.KubeClient, error) {
	client, err := kubernetes.NewKubeClient("./testdata/kube.config", logger)
	if err == nil && client != nil {
		setClientSet(client, c)
		return client, nil
	} else {
		return nil, err
	}
}

func setKubeClient(server service.Server, c kubernetes.KubeClient) {
	pointerVal := reflect.ValueOf(server)
	val := reflect.Indirect(pointerVal)
	member := val.FieldByName("kube")
	ptrToY := unsafe.Pointer(member.UnsafeAddr())
	realPtrToY := (*kubernetes.KubeClient)(ptrToY)
	*realPtrToY = c
}

func setClientSet(client kubernetes.KubeClient, c kube.Interface) {
	pointerVal := reflect.ValueOf(client)
	val := reflect.Indirect(pointerVal)
	member := val.FieldByName("client")
	ptrToY := unsafe.Pointer(member.UnsafeAddr())
	realPtrToY := (*kube.Interface)(ptrToY)
	*realPtrToY = c
}

func setDHCPClient(server service.Server, c dhcp.DHCPClient) {
	pointerVal := reflect.ValueOf(server)
	val := reflect.Indirect(pointerVal)
	member := val.FieldByName("client")
	ptrToY := unsafe.Pointer(member.UnsafeAddr())
	realPtrToY := (*dhcp.DHCPClient)(ptrToY)
	*realPtrToY = c
}
