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

	log "github.com/sirupsen/logrus"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// DualConn is a udp listener on a local port and a packet connection to use free src port for outgoing messages
type DualConn struct {
	out net.PacketConn
	in  *net.UDPConn

	local  *net.UDPAddr
	remote *net.UDPAddr

	inmux  sync.Mutex
	outmux sync.Mutex

	fixPort bool
	cnt     int
}

// NewDualConn initializes a new connection
func NewDualConn(local *net.UDPAddr, remote *net.UDPAddr, fixPort bool) Connection {

	out, err := net.ListenPacket("ip4:udp", local.IP.String())
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Listen packet %s", out.LocalAddr().String())

	in, err := net.ListenUDP("udp4", local)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Listen upd4 %s", local.String())

	return &DualConn{
		out:     out,
		in:      in,
		local:   local,
		remote:  remote,
		fixPort: fixPort,
		cnt:     0,
	}
}

// Close the connection
func (c *DualConn) Close() error {
	log.Printf("Close packet listener %s", c.out.LocalAddr().String())
	err1 := c.in.Close()
	log.Printf("Close udp4 listener %s", c.local.String())
	err2 := c.out.Close()

	if err1 == nil {
		return err1
	}

	return err2

}

// Local returns the local udp address
func (c *DualConn) Local() *net.UDPAddr {
	return c.local
}

// Remote returns the remote udp address
func (c *DualConn) Remote() *net.UDPAddr {
	return c.remote
}

// Send a DHCP data packet
func (c *DualConn) Send(dhcp *DHCP4) (chan int, chan error) {
	chan1 := make(chan int)
	chan2 := make(chan error)

	go func() {

		ip := &layers.IPv4{
			SrcIP:    c.local.IP,
			DstIP:    c.remote.IP,
			Protocol: layers.IPProtocolTCP,
		}

		udp := &layers.UDP{
			SrcPort: c.getPort(),
			DstPort: layers.UDPPort(67),
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

		if err := c.out.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
			log.Fatal(err)
		}

		i, err := c.out.WriteTo(buf.Bytes(), &net.IPAddr{IP: c.remote.IP})
		if err != nil {
			chan2 <- err
		} else {
			chan1 <- i
		}
	}()

	return chan1, chan2
}

// Receive a DHCP data packet
func (c *DualConn) Receive() (chan *DHCP4, chan error) {
	chan1 := make(chan *DHCP4)
	chan2 := make(chan error)

	go func() {
		c.inmux.Lock()
		defer c.inmux.Unlock()

		buffer := make([]byte, 2048)
		n, _, err := c.in.ReadFromUDP(buffer)

		if err != nil {
			chan2 <- err
			return
		}

		var dhcp2 layers.DHCPv4
		err = dhcp2.DecodeFromBytes(buffer[:n], nil)
		if err != nil {
			chan2 <- err
			return
		}

		result := DHCP4{&dhcp2}
		chan1 <- &result
	}()

	return chan1, chan2
}

func (c *DualConn) getPort() layers.UDPPort {
	if c.fixPort {
		return 68
	}

	if c.cnt < 1 || c.cnt > 60000 {
		c.cnt = 1
		//port := 1
		//now := time.Now()
		//c.cnt = port + now.Second()*100 + now.Nanosecond()/10000000
	} else {
		c.cnt++
	}

	return layers.UDPPort(c.cnt)
}

// Block outgoing traffic until context is finished
func (c *DualConn) Block(ctx context.Context) chan bool {
	rc := make(chan bool)

	go func() {
		c.outmux.Lock()
		defer c.outmux.Unlock()
		<-ctx.Done()
		close(rc)
	}()

	return rc
}
