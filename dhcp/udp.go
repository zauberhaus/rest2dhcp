package dhcp

import (
	"log"
	"net"
	"sync"

	"github.com/google/gopacket/layers"
)

type UDPConn struct {
	conn   *net.UDPConn
	inmux  sync.Mutex
	outmux sync.Mutex
}

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

func (c *UDPConn) Close() error {
	log.Printf("Close %s -> %s", c.conn.LocalAddr().String(), c.conn.RemoteAddr().String())
	return c.conn.Close()
}

func (c *UDPConn) Local() *net.UDPAddr {
	return c.conn.LocalAddr().(*net.UDPAddr)
}

func (c *UDPConn) Remote() *net.UDPAddr {
	return c.conn.RemoteAddr().(*net.UDPAddr)
}

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
