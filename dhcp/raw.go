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
	"log"
	"net"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// RawConn is a packet connection between local and remote
type RawConn struct {
	conn net.PacketConn

	local  *net.UDPAddr
	remote *net.UDPAddr

	inmux  sync.Mutex
	outmux sync.Mutex
}

// NewRawConn initializes a new connection
func NewRawConn(local *net.UDPAddr, remote *net.UDPAddr) Connection {

	out, err := net.ListenPacket("ip4:udp", local.IP.String())
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Listen packet %s", out.LocalAddr().String())

	return &RawConn{
		conn:   out,
		local:  local,
		remote: remote,
	}
}

// Close the connection
func (c *RawConn) Close() error {
	log.Printf("Close listener %s", c.conn.LocalAddr().String())
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
func (c *RawConn) Send(dhcp *DHCP4) (chan int, chan error) {
	chan1 := make(chan int)
	chan2 := make(chan error)

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
			log.Fatal(err)
		}

		c.outmux.Lock()
		defer c.outmux.Unlock()

		if err := c.conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
			log.Fatal(err)
		}

		i, err := c.conn.WriteTo(buf.Bytes(), &net.IPAddr{IP: c.remote.IP})

		if err := c.conn.SetDeadline(time.Now().Add(24 * time.Hour)); err != nil {
			log.Fatal(err)
		}

		if err != nil {
			chan2 <- err
		} else {
			chan1 <- i
		}
	}()

	return chan1, chan2

}

// Receive a DHCP data packet
func (c *RawConn) Receive() (chan *DHCP4, chan error) {
	chan1 := make(chan *DHCP4)
	chan2 := make(chan error)

	go func() {
		c.inmux.Lock()
		defer c.inmux.Unlock()

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

			err = udp.DecodeFromBytes(buffer[:n], nil)
			if err != nil {
				chan2 <- err
				return
			}

			if udp.DstPort == layers.UDPPort(c.local.Port) {
				break
			}
		}

		err := dhcp.DecodeFromBytes(udp.Payload, nil)
		if err != nil {
			chan2 <- err
			return
		}

		result := DHCP4{&dhcp}
		chan1 <- &result
	}()

	return chan1, chan2
}

func (c *RawConn) serialize(dhcp *layers.DHCPv4) []byte {
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		ComputeChecksums: true,
		FixLengths:       true,
	}

	err := dhcp.SerializeTo(buf, opts)
	if err != nil {
		log.Fatal(err)
	}

	l := len(buf.Bytes())
	if l <= 300 {
		buf.AppendBytes(301 - l)
	}

	return buf.Bytes()
}

// Block outgoing traffic until context is finished
func (c *RawConn) Block(ctx context.Context) chan bool {
	rc := make(chan bool)

	go func() {
		c.outmux.Lock()
		defer c.outmux.Unlock()
		<-ctx.Done()
		close(rc)
	}()

	return rc
}
