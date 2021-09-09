package dhcp

import (
	"context"
	"fmt"
	"net"

	"github.com/zauberhaus/rest2dhcp/kubernetes"
	"github.com/zauberhaus/rest2dhcp/logger"
	"github.com/zauberhaus/rest2dhcp/routing"
)

type IPResolver interface {
	GetRelayIP(ctx context.Context) (net.IP, error)
	GetLocalIP(remote net.IP) (net.IP, error)
	GetServerIP() (net.IP, error)
	GetWebServiceIP(ctx context.Context) (net.IP, error)
}

type ConnectionResolver interface {
	GetConnection(local net.IP, remote net.IP, t ConnectionType, logger logger.Logger) Connection
}

type LocalIPResolver struct {
	logger logger.Logger

	local  net.IP
	remote net.IP
}

func (l *LocalIPResolver) GetLocalIP(remote net.IP) (net.IP, error) {
	if l.local != nil && l.remote.To4().String() == remote.To4().String() {
		return l.local, nil
	}

	r, err := routing.New()
	if err != nil {
		l.logger.Fatal(err)
	}

	if r != nil {
		_, gateway, src, err := r.Route(remote)

		if err == nil {
			l.local = src

			if l.remote == nil {
				l.remote = gateway
			}
		}

		return src, err
	} else {
		if l.local != nil {
			return l.local, nil
		} else {
			return nil, fmt.Errorf("empty local ip")
		}
	}
}

func (l *LocalIPResolver) GetServerIP() (net.IP, error) {
	if l.remote != nil {
		return l.remote, nil
	}

	r, err := routing.New()
	if err != nil {
		l.logger.Fatal(err)
	}

	if r != nil {
		_, gateway, src, err := r.Route(net.IP{1, 1, 1, 1})

		if l.local == nil {
			l.local = src
		}

		l.remote = gateway

		return gateway, err
	} else {
		return nil, fmt.Errorf("no DHCP server ip")
	}
}

type StaticIPResolver struct {
	LocalIPResolver
	relay net.IP
	web   net.IP
}

func NewStaticIPResolver(local net.IP, remote net.IP, relay net.IP, web net.IP, logger logger.Logger) *StaticIPResolver {
	resolver := &StaticIPResolver{
		LocalIPResolver: LocalIPResolver{
			logger: logger,
			local:  local,
			remote: remote,
		},
		relay: relay,
		web:   web,
	}

	return resolver
}

func (r *StaticIPResolver) GetRelayIP(ctx context.Context) (net.IP, error) {
	if r.relay == nil {
		r.relay = r.local
	}

	return r.relay, nil
}

func (r *StaticIPResolver) GetWebServiceIP(ctx context.Context) (net.IP, error) {
	if r.web == nil {
		r.web = r.local
	}

	return r.web, nil
}

type KubernetesExternalIPResolver struct {
	LocalIPResolver

	client    kubernetes.KubeClient
	service   string
	web       string
	namespace string
}

func NewKubernetesExternalIPResolver(local net.IP, remote net.IP, config *KubeServiceConfig, client kubernetes.KubeClient, logger logger.Logger) *KubernetesExternalIPResolver {

	return &KubernetesExternalIPResolver{
		LocalIPResolver: LocalIPResolver{
			logger: logger,
			local:  local,
			remote: remote,
		},
		client:    client,
		service:   config.Service,
		web:       config.WebService,
		namespace: config.Namespace,
	}

}

func (r *KubernetesExternalIPResolver) GetRelayIP(ctx context.Context) (net.IP, error) {
	if r.service == "" {
		return nil, nil
	}

	return r.client.GetExternalIP(ctx, r.namespace, r.service)
}

func (r *KubernetesExternalIPResolver) GetWebServiceIP(ctx context.Context) (net.IP, error) {
	if r.web == "" {
		return nil, nil
	}

	return r.client.GetExternalIP(ctx, r.namespace, r.web)
}

type DefaultConnectioneResolver struct {
}

func NewDefaultConnectioneResolver() *DefaultConnectioneResolver {
	return &DefaultConnectioneResolver{}
}

func (r *DefaultConnectioneResolver) GetConnection(local net.IP, remote net.IP, t ConnectionType, logger logger.Logger) Connection {
	switch t {
	case UDP:
		return NewUDPConn(&net.UDPAddr{
			IP:   local,
			Port: 67,
		}, &net.UDPAddr{
			IP:   remote,
			Port: 67,
		}, nil, logger)
	case Dual:
		return NewDualConn(&net.UDPAddr{
			IP:   local,
			Port: 67,
		}, &net.UDPAddr{
			IP:   remote,
			Port: 67,
		}, true, nil, nil, logger)
	case Fritzbox:
		return NewDualConn(&net.UDPAddr{
			IP:   local,
			Port: 67,
		}, &net.UDPAddr{
			IP:   remote,
			Port: 67,
		}, false, nil, nil, logger)
	case Broken:
		return NewRawConn(&net.UDPAddr{
			IP:   local,
			Port: 68,
		}, &net.UDPAddr{
			IP:   remote,
			Port: 67,
		}, nil, logger)
	case Packet:
		return NewRawConn(&net.UDPAddr{
			IP:   local,
			Port: 67,
		}, &net.UDPAddr{
			IP:   remote,
			Port: 67,
		}, nil, logger)
	default:
		return nil
	}
}
