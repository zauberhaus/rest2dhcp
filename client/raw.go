package client

import (
	"log"
	"net"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

type RawConn struct {
	conn net.PacketConn

	local  *net.UDPAddr
	remote *net.UDPAddr

	inmux  sync.Mutex
	outmux sync.Mutex
}

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

func (c *RawConn) Close() error {
	log.Printf("Close listener %s", c.conn.LocalAddr().String())
	return c.conn.Close()
}

func (c *RawConn) Local() *net.UDPAddr {
	return c.local
}

func (c *RawConn) Remote() *net.UDPAddr {
	return c.remote
}

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

func (c *RawConn) Receive() (chan *DHCP4, chan error) {
	chan1 := make(chan *DHCP4)
	chan2 := make(chan error)

	go func() {
		c.inmux.Lock()
		defer c.inmux.Unlock()

		//c.conn.SetDeadline(time.Now().Add(2 * time.Second))

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
