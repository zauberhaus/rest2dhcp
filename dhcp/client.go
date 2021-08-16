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

package dhcp

import (
	"context"
	"fmt"
	"hash/crc64"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/gopacket/layers"
	"github.com/zauberhaus/rest2dhcp/background"
	"github.com/zauberhaus/rest2dhcp/logger"
)

const (
	sendMessage    = "Send DHCP %s (%v)"
	timeoutMessage = "Timeout, wait %v (%v)"
	retryMessage   = "Retry..."
)

type DHCPClient interface {
	Start() chan bool
	Stop()

	GetLease(ctx context.Context, hostname string, chaddr net.HardwareAddr) chan *Lease
	Renew(ctx context.Context, hostname string, chaddr net.HardwareAddr, ip net.IP) chan *Lease
	Release(ctx context.Context, hostname string, chaddr net.HardwareAddr, ip net.IP) chan error
}

// dhcpClientImpl is a simple DHCP relay client
type dhcpClientImpl struct {
	background.Process
	conn  Connection
	store *LeaseStore

	timeout time.Duration
	retry   time.Duration

	resolver IPResolver

	remote net.IP
	local  net.IP
	relay  net.IP

	mode ConnectionType

	logger logger.Logger
}

var (
	hashtable = crc64.MakeTable(crc64.ECMA)
)

// NewClient initialize a new client
func NewClient(resolver IPResolver, connResolver ConnectionResolver, connType ConnectionType, timeout time.Duration, retry time.Duration, logger logger.Logger) DHCPClient {

	if connResolver == nil {
		connResolver = &DefaultConnectioneResolver{}
	}

	client := dhcpClientImpl{
		store:    NewStore(60*time.Second, logger),
		timeout:  timeout,
		retry:    retry,
		logger:   logger,
		resolver: resolver,
	}

	client.Init("DHCP client", func(ctx context.Context) error {
		remote, err := resolver.GetServerIP()
		if err != nil {
			return err
		}

		client.remote = remote

		local, err := resolver.GetLocalIP(remote)
		if err != nil {
			return err
		}

		client.local = local

		relay, err := resolver.GetRelayIP(ctx)
		if err != nil {
			return err
		}

		client.relay = relay

		if connType == AutoDetect {
			connType = client.getAutoConnectionType(remote)
		}

		logger.Infof("DHCP server: %v", client.remote)

		client.mode = connType
		client.conn = connResolver.GetConnection(local, remote, connType, logger)

		if client.conn == nil {
			logger.Fatalf("unknown connection type: %v", connType)
		} else {
			logger.Infof("Use %v connection", connType)
		}

		return nil

	}, func(ctx context.Context) error {
		return client.conn.Close()
	}, logger)

	return &client
}

func (c *dhcpClientImpl) Start() chan bool {
	return c.Run(func(ctx context.Context) bool {
		c.store.Run(ctx)

		for {
			c3, c2 := c.conn.Receive(ctx)
			select {
			case err := <-c2:
				c.logger.Errorf("Receive error: %v", err)
			case <-ctx.Done():
				return false
			case dhcp := <-c3:
				if dhcp != nil {
					c.processDHCPReponse(dhcp)
				} else {
					c.logger.Error("Got empty packet")
				}
			}
		}
	})
}

func (c *dhcpClientImpl) processDHCPReponse(dhcp *DHCP4) {
	lease, ok, cancel := c.store.Get(dhcp.Xid)
	defer cancel()

	if !ok {
		c.logger.Errorf("Unexpected DHCP response (%v)", dhcp.Xid)
		return
	}

	msgType := dhcp.GetMsgType()
	c.logger.Debugf("Got DHCP %s (%v)", msgType, lease.Xid)
	if !lease.CheckResponseType(dhcp) {
		c.logger.Errorf("Unexpected response %s -> %s (%v)", lease.GetMsgType(), dhcp.GetMsgType(), dhcp.Xid)
		return
	}

	if msgType == layers.DHCPMsgTypeNak {
		msg := dhcp.GetOption(layers.DHCPOptMessage)
		if msg != nil {
			lease.SetError(fmt.Errorf("NAK: %v", string(msg.Data)))
		} else {
			lease.SetError(fmt.Errorf("NAK"))
		}
	}

	c.logger.Debugf("Change status %s -> %s (%v)", lease.GetMsgType(), dhcp.GetMsgType(), lease.Xid)
	lease.DHCP4 = dhcp
	lease.Touch()
	close(lease.Done)
	c.logger.Debugf("Lease done %v", lease.Xid)
}

func (c *dhcpClientImpl) error(ctx context.Context, xid uint32, types ...layers.DHCPMsgType) (*Lease, bool) {
	c.logger.Infof(timeoutMessage, c.retry, xid)
	if !c.sleep(ctx, c.retry) {
		return nil, false
	}

	l, ok, cancel := c.store.Get(xid)
	defer cancel()

	msgType := l.GetMsgType()
	finished := false
	for _, i := range types {
		if i == msgType {
			finished = true
			break
		}
	}

	if ok && finished {
		c.store.Remove(l.Xid)
		return l, false
	}

	c.logger.Info(retryMessage)
	return nil, true
}

// GetLease requests a new lease with given hostname and mac address
// @param ctx context.Context - for authentication, logging, cancellation, deadlines, tracing, etc. Passed from http.Request or context.Background().
// @param hostname Hostname
// @param mac Mac address
// @param ip IP address
func (c *dhcpClientImpl) GetLease(ctx context.Context, hostname string, chaddr net.HardwareAddr) chan *Lease {
	chan1 := make(chan *Lease, 1)

	if chaddr == nil {
		chaddr = c.getHardwareAddr(hostname)
	}

	go func() {
		var lease *Lease
		var lease2 *Lease

		for {
			timeout, cancel := context.WithTimeout(ctx, c.timeout)
			defer cancel()

			xid := GenerateXID()

			ch := c.discover(timeout, xid, c.conn, hostname, chaddr, nil)
			lease = c.wait(timeout, ch)

			if lease != nil {
				c.logger.Debugf("DHCP discover finished (%v)", lease.YourClientIP)
				c.store.Remove(lease.Xid)
				break
			} else {
				tmp, ok := c.error(ctx, xid, layers.DHCPMsgTypeOffer)
				if !ok {
					lease = tmp
					break
				}
			}
		}

		if lease == nil || !lease.Ok() {
			chan1 <- lease
			return
		}

		for {
			timeout, cancel := context.WithTimeout(ctx, c.timeout)
			defer cancel()

			ch := c.request(timeout, layers.DHCPMsgTypeRequest, lease, c.conn, nil)
			lease2 = c.wait(timeout, ch)

			if lease2 != nil {
				c.logger.Debugf("DHCP request finished (%v/%v - %v)", lease2.GetMsgType(), lease2.YourClientIP, lease2.Xid)
				c.store.Remove(lease2.Xid)
				break
			} else {
				tmp, ok := c.error(ctx, lease.Xid, layers.DHCPMsgTypeAck, layers.DHCPMsgTypeNak)
				if !ok {
					lease2 = tmp
					break
				}
			}
		}

		if lease2 != nil {
			c.logger.Debugf("Got lease: %v (%v / %v)", lease2.YourClientIP, lease2.GetMsgType(), lease2.Xid)
		} else {
			c.logger.Errorf("Get lease failed: %v", ctx.Err())
		}

		chan1 <- lease2
	}()

	return chan1
}

// Renew a lease
// @param ctx context.Context - for authentication, logging, cancellation, deadlines, tracing, etc. Passed from http.Request or context.Background().
// @param hostname Hostname
// @param mac Mac address
// @param ip IP address
func (c *dhcpClientImpl) Renew(ctx context.Context, hostname string, chaddr net.HardwareAddr, ip net.IP) chan *Lease {
	chan1 := make(chan *Lease, 1)

	if chaddr == nil {
		chaddr = c.getHardwareAddr(hostname)
	}

	go func() {
		var lease2 *Lease

		for {
			xid := GenerateXID()

			lease := NewLease(layers.DHCPMsgTypeDiscover, xid, chaddr, nil)
			relayIP, err := c.resolver.GetRelayIP(ctx)
			if err != nil {
				c.logger.Errorf("%v (%v)", err, xid)
			}

			lease.RelayAgentIP = relayIP
			lease.YourClientIP = ip
			if hostname != "" {
				lease.SetHostname(hostname)
			}

			timeout, cancel := context.WithTimeout(ctx, c.timeout)
			defer cancel()

			ch := c.request(ctx, layers.DHCPMsgTypeRequest, lease, c.conn, nil)
			lease2 = c.wait(timeout, ch)

			if lease2 != nil {
				c.logger.Debugf("DHCP request (renew) finished (%v - %v)", lease2.YourClientIP, lease2.Xid)
				c.store.Remove(lease2.Xid)
				break
			} else {
				c.logger.Infof(timeoutMessage, c.retry, lease.Xid)
				if c.sleep(ctx, c.retry) {
					l, ok, cancel := c.store.Get(lease.Xid)
					defer cancel()

					if ok && (l.GetMsgType() == layers.DHCPMsgTypeAck || l.GetMsgType() == layers.DHCPMsgTypeNak) {
						c.store.Remove(l.Xid)
						lease2 = l
						break
					} else {
						c.logger.Info(retryMessage)
					}
				} else {
					break
				}
			}
		}

		chan1 <- lease2
	}()

	return chan1
}

// Release a lease
// @param ctx context.Context - for authentication, logging, cancellation, deadlines, tracing, etc. Passed from http.Request or context.Background().
// @param hostname Hostname
// @param mac Mac address
// @param ip IP address
func (c *dhcpClientImpl) Release(ctx context.Context, hostname string, chaddr net.HardwareAddr, ip net.IP) chan error {
	chan1 := make(chan error, 1)

	if chaddr == nil {
		chaddr = c.getHardwareAddr(hostname)
	}

	go func() {

		xid := GenerateXID()

		request := NewPackage(layers.DHCPMsgTypeRelease, xid, chaddr, nil)
		relayIP, err := c.resolver.GetRelayIP(ctx)
		if err != nil {
			c.logger.Error("%v (%v)", err, xid)
		}

		request.RelayAgentIP = relayIP
		request.ClientIP = ip

		c.logger.Debugf(sendMessage, strings.ToUpper(layers.DHCPMsgTypeRelease.String()), xid)

		c1, c2 := c.conn.Send(ctx, request)
		select {
		case err := <-c2:
			chan1 <- err
		case <-c1:
			chan1 <- nil
		case <-ctx.Done():
			chan1 <- fmt.Errorf("Release error: %v", ctx.Err())
		}

	}()

	return chan1
}

func (c *dhcpClientImpl) discover(ctx context.Context, xid uint32, conn Connection, hostname string, chaddr net.HardwareAddr, options layers.DHCPOptions) chan *Lease {
	chan1 := make(chan *Lease)

	go func() {

		dhcp := NewLease(layers.DHCPMsgTypeDiscover, xid, chaddr, options)
		dhcp.SetHostname(hostname)
		relayIP, err := c.resolver.GetRelayIP(ctx)
		if err != nil {
			c.logger.Errorf("Get relay id: %v", err)
		}

		dhcp.RelayAgentIP = relayIP

		c.logger.Debugf(sendMessage, strings.ToUpper(layers.DHCPMsgTypeDiscover.String()), xid)

		c.store.Set(dhcp)
		c1, c2 := conn.Send(ctx, dhcp.DHCP4)
		select {
		case err := <-c2:
			c.store.Remove(dhcp.Xid)
			chan1 <- NewLeaseError(err)
		case <-c1:
			select {
			case <-dhcp.Done:
				chan1 <- dhcp
			case <-ctx.Done():
				chan1 <- nil
			}
		case <-ctx.Done():
			chan1 <- nil
		}

	}()

	return chan1
}

func (c *dhcpClientImpl) request(ctx context.Context, msgType layers.DHCPMsgType, lease *Lease, conn Connection, options layers.DHCPOptions) chan *Lease {
	rc := make(chan *Lease, 1)

	go func() {

		request := lease.GetRequest(msgType, options)

		c.logger.Debugf(sendMessage, strings.ToUpper(msgType.String()), request.Xid)
		lease.SetMsgType(layers.DHCPMsgTypeRequest)

		c.store.Set(request)
		c1, c2 := conn.Send(ctx, request.DHCP4)
		select {
		case err := <-c2:
			rc <- NewLeaseError(err)
		case <-c1:
			select {
			case <-request.Done:
				rc <- request
			case <-ctx.Done():
				rc <- nil
			}
		case <-ctx.Done():
			rc <- nil
		}

	}()

	return rc

}

func (c *dhcpClientImpl) getAutoConnectionType(remote net.IP) ConnectionType {
	client := http.Client{
		Timeout: 1 * time.Second,
	}

	resp, err := client.Get("http://" + remote.String())
	if err == nil {
		body, err := ioutil.ReadAll(resp.Body)
		if err == nil {
			if strings.Contains(string(body), "AVM") {
				return Fritzbox
			}
		}
	} else if remote.Equal(net.IP{192, 168, 43, 1}) {
		return Broken
	}

	return UDP
}

func (c *dhcpClientImpl) getHardwareAddr(name string) net.HardwareAddr {

	addr, err := net.ParseMAC(name)
	if err == nil {
		return addr
	}

	h := crc64.Checksum([]byte(name), hashtable)
	return []byte{
		byte(0xff & h),
		byte(0xff & (h >> 8)),
		byte(0xff & (h >> 16)),
		byte(0xff & (h >> 24)),
		byte(0xff & (h >> 32)),
		byte(0xff & (h >> 40))}
}

func (c *dhcpClientImpl) wait(ctx context.Context, ch chan *Lease) *Lease {
	select {
	case lease := <-ch:
		return lease
	case <-ctx.Done():
		return nil
	}
}

func (c *dhcpClientImpl) sleep(ctx context.Context, timeout time.Duration) bool {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	c.conn.Block(ctx)

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}
