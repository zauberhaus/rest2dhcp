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

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/zauberhaus/rest2dhcp/logger"
)

// RawConn is a packet connection between local and remote
type RawConn struct {
	conn net.PacketConn

	local  *net.UDPAddr
	remote *net.UDPAddr

	inmux  sync.Mutex
	outmux sync.Mutex

	logger logger.Logger
}

// NewRawConn initializes a new connection
func NewRawConn(local *net.UDPAddr, remote *net.UDPAddr, conn net.PacketConn, logger logger.Logger) Connection {

	if conn == nil {
		c, err := net.ListenPacket("ip4:udp", local.IP.String())
		if err != nil {
			logger.Fatal(err)
		} else {
			conn = c
		}
	}

	logger.Infof("Listen packet %s", conn.LocalAddr().String())

	return &RawConn{
		conn:   conn,
		local:  local,
		remote: remote,
		logger: logger,
	}
}

// Close the connection
func (c *RawConn) Close() error {
	c.logger.Infof("Close listener %s", c.conn.LocalAddr().String())
	return c.conn.Close()
}

// Local returns the local udp address
func (c *RawConn) Local() *net.UDPAddr {
	return c.local
}

// Remote returns the remote udp address
func (c *RawConn) Remote() *net.UDPAddr {
	return c.remote
}

// Send a DHCP data packet
func (c *RawConn) Send(ctx context.Context, dhcp *DHCP4) (chan int, chan error) {
	chan1 := make(chan int, 1)
	chan2 := make(chan error, 1)
	done := make(chan bool)

	go func() {
		select {
		case <-done:
			return
		case <-ctx.Done():
			c.logger.Debugf("Send Error: %v", ctx.Err())
			c.conn.SetWriteDeadline(time.Now())
			chan2 <- ctx.Err()
		}
	}()

	go func() {

		ip := &layers.IPv4{
			SrcIP:    c.local.IP,
			DstIP:    c.remote.IP,
			Protocol: layers.IPProtocolTCP,
		}

		udp := &layers.UDP{
			SrcPort: layers.UDPPort(c.local.Port),
			DstPort: layers.UDPPort(c.remote.Port),
		}

		udp.SetNetworkLayerForChecksum(ip)

		buf := gopacket.NewSerializeBuffer()
		opts := gopacket.SerializeOptions{
			ComputeChecksums: true,
			FixLengths:       true,
		}

		if err := gopacket.SerializeLayers(buf, opts, udp, dhcp); err != nil {
			chan2 <- err
			return
		}

		c.outmux.Lock()
		defer c.outmux.Unlock()

		if err := c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
			chan2 <- err
			return
		}

		i, err := c.conn.WriteTo(buf.Bytes(), &net.IPAddr{IP: c.remote.IP})

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
func (c *RawConn) Receive(ctx context.Context) (chan *DHCP4, chan error) {
	chan1 := make(chan *DHCP4, 1)
	chan2 := make(chan error, 1)
	done := make(chan bool)

	go func() {
		select {
		case <-done:
			return
		case <-ctx.Done():
			c.logger.Debugf("Receive Error: %v", ctx.Err())
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

		var (
			udp  layers.UDP
			dhcp layers.DHCPv4
		)

		for {
			buffer := make([]byte, 2048)
			n, _, err := c.conn.ReadFrom(buffer)
			if err != nil {
				chan2 <- err
				return
			}

			err = udp.DecodeFromBytes(buffer[:n], gopacket.NilDecodeFeedback)
			if err != nil {
				chan2 <- err
				return
			}

			if udp.DstPort == layers.UDPPort(c.local.Port) {
				break
			}
		}

		close(done)

		//ioutil.WriteFile("./testdata/payload.dat", udp.Payload, 0644)

		err := dhcp.DecodeFromBytes(udp.Payload, gopacket.NilDecodeFeedback)
		if err != nil {
			chan2 <- err
			return
		}

		result := DHCP4{&dhcp}
		chan1 <- &result
	}()

	return chan1, chan2
}

// Block outgoing traffic until context is finished
func (c *RawConn) Block(ctx context.Context) chan bool {
	rc := make(chan bool)
	c.outmux.Lock()

	go func() {
		defer c.outmux.Unlock()
		<-ctx.Done()
		close(rc)
	}()

	return rc
}
