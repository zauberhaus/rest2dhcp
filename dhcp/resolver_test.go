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

package dhcp_test

import (
	"context"
	"fmt"
	"net"
	"runtime"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/zauberhaus/rest2dhcp/dhcp"
	"github.com/zauberhaus/rest2dhcp/mock"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestLocalIPResolver_GetLocalIP(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 0, 0, 0)

	type args struct {
		remote net.IP
	}
	tests := []struct {
		name   string
		local  net.IP
		remote net.IP
		relay  net.IP
		args   args
		want   net.IP
	}{
		{
			name:   "test1",
			local:  net.IP{1, 1, 1, 1},
			remote: net.IP{2, 2, 2, 2},
			relay:  net.IP{3, 3, 3, 3},
			args: args{
				remote: net.IP{2, 2, 2, 2},
			},
			want: net.IP{1, 1, 1, 1},
		},
		{
			name:   "test2",
			local:  net.IP{1, 1, 1, 1},
			remote: net.IP{2, 2, 2, 2},
			relay:  net.IP{3, 3, 3, 3},
			args: args{
				remote: net.IP{1, 1, 1, 1},
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := dhcp.NewStaticIPResolver(tt.local, tt.remote, tt.relay, logger)
			got, err := l.GetLocalIP(tt.args.remote)
			assert.NoError(t, err)
			if tt.want != nil {
				assert.Equal(t, tt.want, got)
			} else {
				if runtime.GOOS == "linux" {
					assert.NotEqual(t, tt.local, got)
				} else {
					assert.Equal(t, tt.local, got)
				}
				assert.NotEqual(t, tt.remote, got)
				assert.NotEqual(t, tt.relay, got)
			}
		})
	}
}

func TestLocalIPResolver_GetServerIP(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 0, 0, 0)

	tests := []struct {
		name   string
		local  net.IP
		remote net.IP
		relay  net.IP
	}{
		{
			name:   "GetServerIP (variable)",
			local:  net.IP{1, 1, 1, 1},
			remote: net.IP{2, 2, 2, 2},
			relay:  net.IP{3, 3, 3, 3},
		},
		{
			name:  "GetServerIP (route)",
			local: net.IP{1, 1, 1, 1},
			relay: net.IP{3, 3, 3, 3},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := dhcp.NewStaticIPResolver(tt.local, tt.remote, tt.relay, logger)
			got, err := l.GetServerIP()
			if tt.remote != nil {
				assert.NoError(t, err)
				assert.Equal(t, tt.remote, got)
			} else {
				if runtime.GOOS == "linux" {
					assert.NoError(t, err)
					assert.NotEqual(t, tt.remote, got)
				} else {
					assert.Error(t, err)
				}
				assert.NotEqual(t, tt.local, got)
				assert.NotEqual(t, tt.relay, got)
			}
		})
	}
}

func TestStaticIPResolver_GetRelayIP(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 1, 0, 0)

	tests := []struct {
		name   string
		local  net.IP
		remote net.IP
		relay  net.IP
	}{
		{
			name:   "GetServerIP (variable)",
			local:  net.IP{1, 1, 1, 1},
			remote: net.IP{2, 2, 2, 2},
			relay:  net.IP{3, 3, 3, 3},
		},
		{
			name:   "GetServerIP (route)",
			local:  net.IP{1, 1, 1, 1},
			remote: net.IP{2, 2, 2, 2},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			l := dhcp.NewStaticIPResolver(tt.local, tt.remote, tt.relay, logger)
			got, err := l.GetRelayIP(ctx)
			assert.NoError(t, err)
			if tt.relay != nil {
				assert.Equal(t, tt.relay, got)
			} else {
				assert.Equal(t, tt.local, got)
			}
		})
	}
}

func TestNewDefaultConnectioneResolver(t *testing.T) {
	resolver := dhcp.NewDefaultConnectioneResolver()
	assert.NotNil(t, resolver)
}

func TestKubernetesExternalIPResolver_GetRelayIP(t *testing.T) {
	local := net.IP{1, 1, 1, 1}
	remote := net.IP{2, 2, 2, 2}

	tests := []struct {
		name    string
		svc     *v1.Service
		svc_err error
		want    net.IP
		logs    []int64
		err     error
	}{
		{
			name: "LoadBalancerIP",
			svc: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "svc001",
					Namespace: "ns001",
				},
				Spec: v1.ServiceSpec{
					LoadBalancerIP: local.String(),
				},
			},
			want: local,
			logs: []int64{0, 0, 0, 1, 0, 0},
		},
		{
			name: "ExternalIPs",
			svc: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "svc001",
					Namespace: "ns001",
				},
				Spec: v1.ServiceSpec{
					ExternalIPs: []string{local.String()},
				},
			},
			want: local,
			logs: []int64{0, 0, 0, 1, 0, 0},
		},
		{
			name: "No ExternalIPs",
			svc: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "svc001",
					Namespace: "ns001",
				},
				Spec: v1.ServiceSpec{
					ExternalIPs: []string{},
				},
			},
			err:  fmt.Errorf("service ns001/svc001 has no external IP"),
			logs: []int64{0, 0, 0, 0, 0, 0},
		},
		{
			name: "Multiple ExternalIPs",
			svc: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "svc001",
					Namespace: "ns001",
				},
				Spec: v1.ServiceSpec{
					ExternalIPs: []string{local.String(), remote.String()},
				},
			},
			err:  fmt.Errorf("service ns001/svc001 has multiple external IPs"),
			logs: []int64{0, 0, 0, 0, 0, 0},
		},
		{
			name: "Invalid ExternalIPs",
			svc: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "svc001",
					Namespace: "ns001",
				},
				Spec: v1.ServiceSpec{
					ExternalIPs: []string{"abc"},
				},
			},
			err:  fmt.Errorf("invalid external IP format 'abc' for service ns001/svc001"),
			logs: []int64{0, 0, 0, 0, 0, 0},
		},
		{
			name:    "service not found",
			svc_err: fmt.Errorf("service not found"),
			err:     fmt.Errorf("resolve external IP from ns001/svc001: service not found"),
			logs:    []int64{0, 0, 0, 0, 0, 0},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			logger := mock.NewTestLogger()
			defer logger.Assert(t, tt.logs...)

			config := &dhcp.KubeServiceConfig{
				Config:    "./testdata/config.yaml",
				Namespace: "ns001",
				Service:   "svc001",
			}

			client := mock.NewMockKubeClient(ctrl)

			client.EXPECT().
				GetService(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(tt.svc, tt.svc_err)

			r := dhcp.NewKubernetesExternalIPResolver(local, remote, config, client, logger)
			ctx := context.Background()

			ip, err := r.GetRelayIP(ctx)
			if tt.err == nil {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, ip.To4())
			} else {
				assert.EqualError(t, err, tt.err.Error())
			}
		})
	}
}

func TestKubernetesExternalIPResolver_GetLocalIP(t *testing.T) {
	local := net.IP{1, 1, 1, 1}
	remote := net.IP{2, 2, 2, 2}
	extern := net.IP{8, 8, 8, 8}

	tests := []struct {
		name    string
		local   net.IP
		remote  net.IP
		target  net.IP
		svc_err error
		want    net.IP
		logs    []int64
		err     error
	}{
		{
			name:   "RemoteIP",
			local:  local,
			remote: remote,
			target: remote,
			want:   local,
			logs:   []int64{0, 0, 0, 0, 0, 0},
		},
		{
			name:   "NonLinux 1",
			local:  local,
			remote: remote,
			target: extern,
			want:   local,
			logs:   []int64{0, 0, 0, 0, 0, 0},
		},
		{
			name:   "NonLinux 2",
			local:  nil,
			remote: remote,
			target: extern,
			err:    fmt.Errorf("empty local ip"),
			logs:   []int64{0, 0, 0, 0, 0, 0},
		},
		{
			name:   "Linux 2",
			local:  nil,
			remote: remote,
			target: extern,
			err:    fmt.Errorf("empty local ip"),
			logs:   []int64{0, 0, 0, 0, 0, 0},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if runtime.GOARCH == "linux" && strings.HasPrefix(tt.name, "NonLinux ") {
				return
			}

			if runtime.GOARCH != "linux" && strings.HasPrefix(tt.name, "Linux ") {
				return
			}

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			logger := mock.NewTestLogger()
			defer logger.Assert(t, tt.logs...)

			config := &dhcp.KubeServiceConfig{
				Config:    "./testdata/config.yaml",
				Namespace: "ns001",
				Service:   "svc001",
			}

			client := mock.NewMockKubeClient(ctrl)

			r := dhcp.NewKubernetesExternalIPResolver(tt.local, tt.remote, config, client, logger)

			ip, err := r.GetLocalIP(tt.target)
			if tt.err == nil {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, ip.To4())
			} else {
				assert.EqualError(t, err, tt.err.Error())
			}

			ip, err = r.GetServerIP()
			assert.NoError(t, err)
			assert.Equal(t, remote, ip.To4())

		})
	}
}
