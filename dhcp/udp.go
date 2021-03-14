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
	"net"
	"sync"
	"time"

	"github.com/zauberhaus/rest2dhcp/logger"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

type UDPPacketConn interface {
	net.PacketConn
	net.Conn
	LocalAddr() net.Addr
	RemoteAddr() net.Addr
	ReadFromUDP(b []byte) (int, *net.UDPAddr, error)
}

// UDPConn is a simple upd connection using the same src and destination port
type UDPConn struct {
	conn   UDPPacketConn
	inmux  sync.Mutex
	outmux sync.Mutex

	logger logger.Logger
}

// NewUDPConn initializes a new udp connection
func NewUDPConn(local *net.UDPAddr, remote *net.UDPAddr, conn UDPPacketConn, logger logger.Logger) Connection {
	if conn == nil {
		sc, err := net.DialUDP("udp4", local, remote)
		if err != nil {
			logger.Fatal(err)
		} else {
			conn = sc
		}
	}

	logger.Infof("Connect %s -> %s", conn.LocalAddr().String(), conn.RemoteAddr().String())

	return &UDPConn{
		conn:   conn,
		logger: logger,
	}
}

// Close the connection
func (c *UDPConn) Close() error {
	c.logger.Infof("Close %s -> %s", c.conn.LocalAddr().String(), c.conn.RemoteAddr().String())
	return c.conn.Close()
}

// Local returns the local udp address
func (c *UDPConn) Local() *net.UDPAddr {
	return c.conn.LocalAddr().(*net.UDPAddr)
}

// Remote returns the local udp address
func (c *UDPConn) Remote() *net.UDPAddr {
	return c.conn.RemoteAddr().(*net.UDPAddr)
}

// Send a DHCP data packet
func (c *UDPConn) Send(ctx context.Context, dhcp *DHCP4) (chan int, chan error) {
	chan1 := make(chan int, 1)
	chan2 := make(chan error, 1)
	done := make(chan bool)

	go func() {
		select {
		case <-done:
			return
		case <-ctx.Done():
			c.logger.Debugf("Send context done: %v", ctx.Err())
			c.conn.SetWriteDeadline(time.Now())
			chan2 <- ctx.Err()
		}
	}()

	go func() {
		c.outmux.Lock()
		defer c.outmux.Unlock()

		if err := c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
			chan2 <- err
			return
		}

		buf, err := dhcp.serialize()
		if err != nil {
			chan2 <- err
			return
		}

		i, err := c.conn.Write(buf)
		close(done)

		if err != nil {
			chan2 <- err
		} else {
			chan1 <- i
		}
	}()

	return chan1, chan2
}

// Receive a DHCP data packet
func (c *UDPConn) Receive(ctx context.Context) (chan *DHCP4, chan error) {
	chan1 := make(chan *DHCP4, 1)
	chan2 := make(chan error, 1)
	done := make(chan bool)

	go func() {
		select {
		case <-done:
			return
		case <-ctx.Done():
			c.logger.Debugf("Receive context done: %v", ctx.Err())
			c.conn.SetReadDeadline(time.Now())
			chan2 <- ctx.Err()
		}
	}()

	go func() {
		c.inmux.Lock()
		defer c.inmux.Unlock()

		if err := c.conn.SetReadDeadline(time.Now().Add(24 * time.Hour)); err != nil {
			chan2 <- err
			return
		}

		buffer := make([]byte, 2048)
		n, _, err := c.conn.ReadFromUDP(buffer)
		close(done)

		if err != nil {
			chan2 <- err
			return
		}

		var dhcp2 layers.DHCPv4
		err = dhcp2.DecodeFromBytes(buffer[:n], gopacket.NilDecodeFeedback)
		if err != nil {
			chan2 <- err
			return
		}

		result := DHCP4{&dhcp2}
		chan1 <- &result
	}()

	return chan1, chan2
}

// Block outgoing traffic until context is finished
func (c *UDPConn) Block(ctx context.Context) chan bool {
	rc := make(chan bool)
	c.outmux.Lock()

	go func() {
		defer c.outmux.Unlock()
		<-ctx.Done()
		close(rc)
	}()

	return rc
}
