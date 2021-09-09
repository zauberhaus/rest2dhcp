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
)

func TestLocalIPResolverGetLocalIP(t *testing.T) {
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
		web    net.IP
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
			l := dhcp.NewStaticIPResolver(tt.local, tt.remote, tt.relay, tt.web, logger)
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

func TestLocalIPResolverGetServerIP(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 0, 0, 0)

	tests := []struct {
		name   string
		local  net.IP
		remote net.IP
		relay  net.IP
		web    net.IP
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
			l := dhcp.NewStaticIPResolver(tt.local, tt.remote, tt.relay, tt.web, logger)
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

func TestStaticIPResolverGetRelayIP(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 0, 0, 0)

	tests := []struct {
		name   string
		local  net.IP
		remote net.IP
		relay  net.IP
		web    net.IP
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
			l := dhcp.NewStaticIPResolver(tt.local, tt.remote, tt.relay, tt.web, logger)
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

func TestStaticIPResolverGetWebServiceIP(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 0, 0, 0)

	tests := []struct {
		name   string
		local  net.IP
		remote net.IP
		relay  net.IP
		web    net.IP
	}{
		{
			name:   "GetServerIP (variable)",
			local:  net.IP{1, 1, 1, 1},
			remote: net.IP{2, 2, 2, 2},
			relay:  net.IP{3, 3, 3, 3},
			web:    net.IP{4, 4, 4, 4},
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
			l := dhcp.NewStaticIPResolver(tt.local, tt.remote, tt.relay, tt.web, logger)
			got, err := l.GetWebServiceIP(ctx)
			assert.NoError(t, err)
			if tt.web != nil {
				assert.Equal(t, tt.web, got)
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

func TestKubernetesExternalIPResolverGetRelayIP(t *testing.T) {
	local := net.IP{1, 1, 1, 1}
	remote := net.IP{2, 2, 2, 2}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 0, 0, 0)

	config := &dhcp.KubeServiceConfig{
		Config:    "./testdata/config.yaml",
		Namespace: "ns001",
		Service:   "svc001",
	}

	client := mock.NewMockKubeClient(ctrl)

	client.EXPECT().
		GetExternalIP(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(local, nil)

	r := dhcp.NewKubernetesExternalIPResolver(local, remote, config, client, logger)
	ctx := context.Background()

	ip, err := r.GetRelayIP(ctx)
	assert.NoError(t, err)
	assert.Equal(t, local, ip.To4())
}

func TestKubernetesExternalIPResolverGetWebIP(t *testing.T) {
	local := net.IP{1, 1, 1, 1}
	remote := net.IP{2, 2, 2, 2}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 0, 0, 0)

	config := &dhcp.KubeServiceConfig{
		Config:     "./testdata/config.yaml",
		Namespace:  "ns001",
		Service:    "svc001",
		WebService: "svc002",
	}

	client := mock.NewMockKubeClient(ctrl)

	client.EXPECT().
		GetExternalIP(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(local, nil)

	r := dhcp.NewKubernetesExternalIPResolver(local, remote, config, client, logger)
	ctx := context.Background()

	ip, err := r.GetWebServiceIP(ctx)
	assert.NoError(t, err)
	assert.Equal(t, local, ip.To4())
}

func TestKubernetesExternalIPResolverGetLocalIP(t *testing.T) {
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
		wantNot []net.IP
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
		/*{
			name:   "Linux 1",
			local:  nil,
			remote: remote,
			target: extern,
			err:    fmt.Errorf("empty local ip"),
			logs:   []int64{0, 0, 0, 0, 0, 0},
		},
		*/
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if runtime.GOOS == "linux" && strings.HasPrefix(tt.name, "NonLinux ") {
				return
			}

			if runtime.GOOS != "linux" && strings.HasPrefix(tt.name, "Linux ") {
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
				if tt.wantNot != nil && len(tt.wantNot) > 0 {
					for _, ips := range tt.wantNot {
						assert.NotEqual(t, tt.want, ips.To4())
					}
				} else {
					assert.Equal(t, tt.want, ip.To4())
				}
			} else {
				assert.EqualError(t, err, tt.err.Error())
			}

			ip, err = r.GetServerIP()
			assert.NoError(t, err)
			assert.Equal(t, remote, ip.To4())

		})
	}
}
