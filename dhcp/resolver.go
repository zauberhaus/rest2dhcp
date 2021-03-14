package dhcp

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/zauberhaus/rest2dhcp/kubernetes"
	"github.com/zauberhaus/rest2dhcp/logger"
	"github.com/zauberhaus/rest2dhcp/routing"
)

type IPResolver interface {
	GetRelayIP(ctx context.Context) (net.IP, error)
	GetLocalIP(remote net.IP) (net.IP, error)
	GetServerIP() (net.IP, error)
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

	_, gateway, src, err := r.Route(remote)

	if err == nil {
		l.local = src

		if l.remote == nil {
			l.remote = gateway
		}
	}

	return src, err
}

func (l *LocalIPResolver) GetServerIP() (net.IP, error) {
	if l.remote != nil {
		return l.remote, nil
	}

	r, err := routing.New()
	if err != nil {
		l.logger.Fatal(err)
	}

	_, gateway, src, err := r.Route(net.IP{1, 1, 1, 1})

	if l.local == nil {
		l.local = src
	}

	l.remote = gateway

	return gateway, err
}

type StaticIPResolver struct {
	LocalIPResolver
	relay net.IP
}

func NewStaticIPResolver(local net.IP, remote net.IP, relay net.IP, logger logger.Logger) *StaticIPResolver {
	resolver := &StaticIPResolver{
		LocalIPResolver: LocalIPResolver{
			logger: logger,
			local:  local,
			remote: remote,
		},
		relay: relay,
	}

	return resolver
}

func (r *StaticIPResolver) GetRelayIP(ctx context.Context) (net.IP, error) {
	if r.relay == nil {
		r.relay = r.local
		r.logger.Infof("Relay agent IP: %v", r.relay)
	}

	return r.relay, nil
}

type KubernetesExternalIPResolver struct {
	LocalIPResolver

	client    kubernetes.KubeClient
	service   string
	namespace string

	last  net.IP
	mtime time.Time
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
		namespace: config.Namespace,
	}

}

func (r *KubernetesExternalIPResolver) GetRelayIP(ctx context.Context) (net.IP, error) {

	result, err := r.client.GetService(ctx, r.namespace, r.service)
	if err != nil {
		return r.last, fmt.Errorf("Resolve external IP from %s/%s: %v", r.namespace, r.service, err)
	}

	lbip := net.ParseIP(result.Spec.LoadBalancerIP)
	if lbip != nil {
		if r.last == nil || r.last.To4().String() != lbip.To4().String() {
			r.logger.Infof("Use Kubernetes loadbalancer IP %v (%s/%s) as relay address", lbip, result.ObjectMeta.Namespace, result.ObjectMeta.Name)
			r.last = lbip
		}

		return lbip, nil
	} else {
		ips := result.Spec.ExternalIPs
		if len(ips) == 0 {
			return r.last, fmt.Errorf("Service %s/%s has no external IP", result.ObjectMeta.Namespace, result.ObjectMeta.Name)
		}

		if len(ips) > 1 {
			return r.last, fmt.Errorf("Service %s/%s has multiple external IPs", result.ObjectMeta.Namespace, result.ObjectMeta.Name)
		}

		ip := net.ParseIP(ips[0])
		if ip != nil {
			if r.last == nil || r.last.To4().String() != lbip.To4().String() {
				r.logger.Infof("Use external Kubernetes service ip %v (%s/%s", result.ObjectMeta.Namespace, result.ObjectMeta.Name, ip)
				r.last = ip
			}

			return ip, nil
		} else {
			return r.last, fmt.Errorf("Invalid IP format: %s", ips[0])
		}
	}
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
