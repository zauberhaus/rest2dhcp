package dhcp_test

import (
	"context"
	"net"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/zauberhaus/rest2dhcp/dhcp"
	"github.com/zauberhaus/rest2dhcp/mock"
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
				assert.NotEqual(t, tt.local, got)
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
			assert.NoError(t, err)
			if tt.remote != nil {
				assert.Equal(t, tt.remote, got)
			} else {
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
