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

	"github.com/google/gopacket/layers"
)

// UDPConn is a simple upd connection using the same src and destination port
type UDPConn struct {
	conn   *net.UDPConn
	inmux  sync.Mutex
	outmux sync.Mutex
}

// NewUDPConn initialises a new udp connection
func NewUDPConn(local *net.UDPAddr, remote *net.UDPAddr) Connection {
	sc, err := net.DialUDP("udp4", local, remote)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Connect %s -> %s", sc.LocalAddr().String(), sc.RemoteAddr().String())

	return &UDPConn{
		conn: sc,
	}
}

// Close the connection
func (c *UDPConn) Close() error {
	log.Printf("Close %s -> %s", c.conn.LocalAddr().String(), c.conn.RemoteAddr().String())
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
func (c *UDPConn) Send(dhcp *DHCP4) (chan int, chan error) {
	chan1 := make(chan int)
	chan2 := make(chan error)

	go func() {
		c.outmux.Lock()
		defer c.outmux.Unlock()

		buf := dhcp.Serialize()
		i, err := c.conn.Write(buf)
		if err != nil {
			chan2 <- err
		} else {
			chan1 <- i
		}
	}()

	return chan1, chan2
}

// Receive a DHCP data packet
func (c *UDPConn) Receive() (chan *DHCP4, chan error) {
	chan1 := make(chan *DHCP4)
	chan2 := make(chan error)

	go func() {
		c.inmux.Lock()
		defer c.inmux.Unlock()

		buffer := make([]byte, 2048)
		n, _, err := c.conn.ReadFromUDP(buffer)

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

// Block outgoing traffic until contect is finished
func (c *UDPConn) Block(ctx context.Context) chan bool {
	rc := make(chan bool)

	go func() {
		c.outmux.Lock()
		defer c.outmux.Unlock()
		<-ctx.Done()
		close(rc)
	}()

	return rc
}
