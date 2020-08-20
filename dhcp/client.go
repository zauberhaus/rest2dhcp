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

	log "github.com/sirupsen/logrus"

	"github.com/google/gopacket/layers"
	"github.com/zauberhaus/rest2dhcp/routing"
)

// Client is a simple DHCP relay client
type Client struct {
	conn  Connection
	store *LeaseStore

	timeout time.Duration
	retry   time.Duration

	relay net.IP

	ctx    context.Context
	cancel context.CancelFunc

	remote net.IP
	mode   ConnectionType

	Done chan bool
}

var (
	hashtable = crc64.MakeTable(crc64.ECMA)
)

// NewClient initialize a new client
func NewClient(local net.IP, remote net.IP, relay net.IP, connType ConnectionType, timeout time.Duration, retry time.Duration) *Client {
	ctx, cancel := context.WithCancel(context.Background())

	client := Client{
		store:   NewStore(60 * time.Second),
		timeout: timeout,
		retry:   retry,
		ctx:     ctx,
		cancel:  cancel,

		Done: make(chan bool),
	}

	if remote == nil {
		gateway, src, err := client.getDefaultGateway()
		if err != nil {
			log.Fatal(err)
		}

		if gateway == nil {
			log.Fatal("Can't detect gatway ip")
		}

		if local == nil {
			local = src
		}

		remote = gateway
	} else {
		remote = remote.To4()
	}

	if local == nil {
		ip, err := client.getLocalIP(remote)
		if err == nil {
			local = ip
		}
	} else {
		local = local.To4()
	}

	if connType == AutoDetect {
		connType = client.getAutoConnectionType(remote)
	}

	log.Infof("DHCP server: %v", remote)
	client.remote = remote

	if relay != nil {
		log.Infof("Relay agent IP: %v", relay)
		client.relay = relay.To4()
	} else {
		client.relay = local
	}

	client.mode = connType

	switch connType {
	case UDP:
		client.conn = NewUDPConn(&net.UDPAddr{
			IP:   local,
			Port: 67,
		}, &net.UDPAddr{
			IP:   remote,
			Port: 67,
		})
	case Dual:
		client.conn = NewDualConn(&net.UDPAddr{
			IP:   local,
			Port: 67,
		}, &net.UDPAddr{
			IP:   remote,
			Port: 67,
		}, true)
	case Fritzbox:
		client.conn = NewDualConn(&net.UDPAddr{
			IP:   local,
			Port: 67,
		}, &net.UDPAddr{
			IP:   remote,
			Port: 67,
		}, false)
	case Broken:
		client.conn = NewRawConn(&net.UDPAddr{
			IP:   local,
			Port: 68,
		}, &net.UDPAddr{
			IP:   remote,
			Port: 67,
		})
	case Packet:
		client.conn = NewRawConn(&net.UDPAddr{
			IP:   local,
			Port: 67,
		}, &net.UDPAddr{
			IP:   remote,
			Port: 67,
		})
	default:
		log.Fatalf("Unknown connection type: %v", connType)
	}

	log.Infof("Use %v connection", connType)

	return &client
}

// Start the client (response listener)
func (c *Client) Start() {
	go func() {
		c.store.Run(c.ctx)

		for {
			c3, c2 := c.conn.Receive()
			select {
			case err := <-c2:
				log.Error(err)
			case <-c.ctx.Done():
				log.Info("DHCP client stopped")
				close(c.Done)
				return
			case dhcp := <-c3:
				if dhcp != nil {
					go func() {
						lease, ok := c.store.Get(dhcp.Xid)
						if ok {
							logDebugf(dhcp.Xid, "Got DHCP %s", dhcp.GetMsgType())
							if lease.CheckResponseType(dhcp) {
								msgType := dhcp.GetMsgType()
								if msgType == layers.DHCPMsgTypeNak {
									msg := dhcp.GetOption(layers.DHCPOptMessage)
									if msg != nil {
										lease.SetError(fmt.Errorf("NAK: %v", string(msg.Data)))
									} else {
										lease.SetError(fmt.Errorf("NAK"))
									}
								}

								logDebugf(lease.Xid, "Change status %s -> %s", lease.GetMsgType(), dhcp.GetMsgType())
								lease.DHCP4 = dhcp
								lease.Touch()
								lease.Done <- true
							} else {
								logErrorf(dhcp.Xid, "Unexpected response %s -> %s", lease.GetMsgType(), dhcp.GetMsgType())
							}

						} else {
							logErrorf(dhcp.Xid, "Unknown DHCP response id=%x", dhcp.Xid)
						}
					}()
				} else {
					log.Error("empty packet")
				}
			}
		}
	}()
}

// Stop the client
func (c *Client) Stop() error {
	c.cancel()
	<-c.Done
	return c.conn.Close()
}

// GetDHCPServerIP returns the current server IP
func (c *Client) GetDHCPServerIP() net.IP {
	return c.remote.To16()
}

// GetDHCPRelayIP returns the current relay IP
func (c *Client) GetDHCPRelayIP() net.IP {
	return c.relay.To16()
}

// GetDHCPRelayMode returns the current connection mode
func (c *Client) GetDHCPRelayMode() ConnectionType {
	return c.mode
}

// GetLease requests a new lease with given hostname and mac address
// @param ctx context.Context - for authentication, logging, cancellation, deadlines, tracing, etc. Passed from http.Request or context.Background().
// @param hostname Hostname
// @param mac Mac address
// @param ip IP address
func (c *Client) GetLease(ctx context.Context, hostname string, chaddr net.HardwareAddr) chan *Lease {
	chan1 := make(chan *Lease)

	if chaddr == nil {
		chaddr = c.getHardwareAddr(hostname)
	}

	go func() {
		var lease *Lease
		var lease2 *Lease

		for {
			ctx2, cancel := context.WithTimeout(ctx, c.timeout)
			ch := c.discover(ctx2, c.conn, hostname, chaddr, nil)
			lease = c.wait(ctx2, ch, cancel)

			if lease != nil {
				logDebugf(lease.Xid, "DHCP discover finished (%v)", lease.YourClientIP)
				break
			} else {
				log.Infof("Timeout, wait %v", c.retry)
				if c.sleep(ctx, c.retry) {
					log.Info("Retry...")
				} else {
					break
				}
			}
		}

		if lease == nil || !lease.Ok() {
			chan1 <- lease
			return
		}

		for {
			ctx2, cancel := context.WithTimeout(ctx, c.timeout)
			ch := c.request(ctx2, layers.DHCPMsgTypeRequest, lease, c.conn, nil)
			lease2 = c.wait(ctx2, ch, cancel)

			if lease2 != nil {
				logDebugf(lease2.Xid, "DHCP request finished (%v)", lease2.YourClientIP)
				c.store.Remove(lease2.Xid)
				break
			} else {
				logInfof(lease.Xid, "Timeout, wait %v", c.retry)
				if c.sleep(ctx, c.retry) {
					log.Info("Retry...")
				} else {
					break
				}
			}
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
func (c *Client) Renew(ctx context.Context, hostname string, chaddr net.HardwareAddr, ip net.IP) chan *Lease {
	chan1 := make(chan *Lease)

	if chaddr == nil {
		chaddr = c.getHardwareAddr(hostname)
	}

	go func() {
		var lease2 *Lease

		for {
			xid := GenerateXID()

			lease := NewLease(layers.DHCPMsgTypeDiscover, xid, chaddr, nil)
			lease.RelayAgentIP = c.relay
			lease.YourClientIP = ip
			if hostname != "" {
				lease.SetHostname(hostname)
			}

			ctx2, cancel := context.WithTimeout(ctx, c.timeout)
			ch := c.request(ctx2, layers.DHCPMsgTypeRequest, lease, c.conn, nil)
			lease2 = c.wait(ctx2, ch, cancel)

			if lease2 != nil {
				logDebugf(lease2.Xid, "DHCP request finished (%v)", lease2.YourClientIP)
				c.store.Remove(lease2.Xid)
				break
			} else {
				log.Infof("Timeout, wait %v", c.retry)
				if c.sleep(ctx, c.retry) {
					log.Info("Retry...")
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
func (c *Client) Release(ctx context.Context, hostname string, chaddr net.HardwareAddr, ip net.IP) chan error {
	chan1 := make(chan error)

	if chaddr == nil {
		chaddr = c.getHardwareAddr(hostname)
	}

	go func() {

		xid := GenerateXID()

		request := NewPackage(layers.DHCPMsgTypeRelease, xid, chaddr, nil)
		request.RelayAgentIP = c.relay
		request.ClientIP = ip

		logDebugf(xid, "Send DHCP %s", strings.ToUpper(layers.DHCPMsgTypeRelease.String()))

		c1, c2 := c.conn.Send(request)
		select {
		case err := <-c2:
			chan1 <- err
		case <-c1:
			chan1 <- nil
		case <-ctx.Done():
			chan1 <- nil
		}

	}()

	return chan1
}

func (c *Client) discover(ctx context.Context, conn Connection, hostname string, chaddr net.HardwareAddr, options layers.DHCPOptions) chan *Lease {
	chan1 := make(chan *Lease)

	go func() {

		xid := GenerateXID()

		dhcp := NewLease(layers.DHCPMsgTypeDiscover, xid, chaddr, options)
		dhcp.SetHostname(hostname)
		dhcp.RelayAgentIP = c.relay

		logDebugf(xid, "Send DHCP %s", strings.ToUpper(layers.DHCPMsgTypeDiscover.String()))

		c1, c2 := conn.Send(dhcp.DHCP4)
		select {
		case err := <-c2:
			chan1 <- NewLeaseError(err)
		case <-c1:
			c.store.Set(dhcp)
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

func (c *Client) request(ctx context.Context, msgType layers.DHCPMsgType, lease *Lease, conn Connection, options layers.DHCPOptions) chan *Lease {
	chan1 := make(chan *Lease)

	go func() {

		request := lease.GetRequest(msgType, options)

		logDebugf(request.Xid, "Send DHCP %s", strings.ToUpper(msgType.String()))
		lease.SetMsgType(layers.DHCPMsgTypeRequest)

		c1, c2 := conn.Send(request)
		select {
		case err := <-c2:
			chan1 <- NewLeaseError(err)
		case <-c1:
			lease.SetMsgType(layers.DHCPMsgTypeRequest)
			c.store.Set(lease)
			select {
			case <-lease.Done:
				chan1 <- lease
			case <-ctx.Done():
				chan1 <- nil
			}
		case <-ctx.Done():
			chan1 <- nil
		}

	}()

	return chan1

}

func (c *Client) getLocalIP(remote net.IP) (net.IP, error) {
	r, err := routing.New()
	if err != nil {
		log.Fatal(err)
	}

	_, _, src, err := r.Route(remote)

	return src, err
}

func (c *Client) getDefaultGateway() (net.IP, net.IP, error) {
	r, err := routing.New()
	if err != nil {
		log.Fatal(err)
	}

	_, gateway, src, err := r.Route(net.IP{1, 1, 1, 1})

	return gateway, src, err
}

func (c *Client) getAutoConnectionType(remote net.IP) ConnectionType {
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

func (c *Client) getHardwareAddr(name string) net.HardwareAddr {

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

func (c *Client) wait(ctx context.Context, ch chan *Lease, cancel context.CancelFunc) *Lease {
	defer cancel()

	select {
	case lease := <-ch:
		return lease
	case <-ctx.Done():
		return nil
	}
}

func (c *Client) sleep(ctx context.Context, timeout time.Duration) bool {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	c.conn.Block(ctx)

	select {
	case <-time.After(timeout):
		return true
	case <-ctx.Done():
		return false
	}
}

func logDebug(xid uint32, args ...interface{}) {
	log.WithFields(log.Fields{
		"xid": fmt.Sprintf("%v", xid),
	}).Debug(args...)
}

func logDebugf(xid uint32, format string, args ...interface{}) {
	log.WithFields(log.Fields{
		"xid": fmt.Sprintf("%v", xid),
	}).Debugf(format, args...)
}

func logInfo(xid uint32, args ...interface{}) {
	log.WithFields(log.Fields{
		"xid": fmt.Sprintf("%v", xid),
	}).Info(args...)
}

func logInfof(xid uint32, format string, args ...interface{}) {
	log.WithFields(log.Fields{
		"xid": fmt.Sprintf("%v", xid),
	}).Infof(format, args...)
}

func logError(xid uint32, args ...interface{}) {
	log.WithFields(log.Fields{
		"xid": fmt.Sprintf("%v", xid),
	}).Error(args...)
}

func logErrorf(xid uint32, format string, args ...interface{}) {
	log.WithFields(log.Fields{
		"xid": fmt.Sprintf("%v", xid),
	}).Errorf(format, args...)
}
