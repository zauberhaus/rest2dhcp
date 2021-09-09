package mock

import (
	context "context"
	"fmt"
	"net"

	"github.com/zauberhaus/rest2dhcp/dhcp"
	logger "github.com/zauberhaus/rest2dhcp/logger"
)

type MockIPResolver struct {
	relay  net.IP
	local  net.IP
	server net.IP
	web    net.IP
}

func NewMockIPResolver(local net.IP, server net.IP, relay net.IP, web net.IP) dhcp.IPResolver {
	return &MockIPResolver{
		local:  local,
		server: server,
		relay:  relay,
		web:    web,
	}
}

func (r *MockIPResolver) GetRelayIP(ctx context.Context) (net.IP, error) {
	if r.relay != nil {
		return r.relay, nil
	} else {
		return nil, fmt.Errorf("error")
	}
}

func (r *MockIPResolver) GetLocalIP(remote net.IP) (net.IP, error) {
	if r.local != nil {
		return r.local, nil
	} else {
		return nil, fmt.Errorf("error")
	}
}

func (r *MockIPResolver) GetServerIP() (net.IP, error) {
	if r.server != nil {
		return r.server, nil
	} else {
		return nil, fmt.Errorf("error")
	}
}

func (r *MockIPResolver) GetWebServiceIP(ctx context.Context) (net.IP, error) {
	if r.web != nil {
		return r.web, nil
	} else {
		return nil, fmt.Errorf("error")
	}
}

type MockConnectionResolver struct {
	conn dhcp.Connection
}

func NewMockConnectionResolver(conn dhcp.Connection) dhcp.ConnectionResolver {
	return &MockConnectionResolver{
		conn: conn,
	}
}

func (r *MockConnectionResolver) GetConnection(local net.IP, remote net.IP, t dhcp.ConnectionType, logger logger.Logger) dhcp.Connection {
	return r.conn
}
