package dhcp

import (
	"log"
	"net"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

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

func (c *DualConn) Local() *net.UDPAddr {
	return c.local
}

func (c *DualConn) Remote() *net.UDPAddr {
	return c.remote
}

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

		c.cnt++

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

	if c.cnt == 0 {
		port := 67
		now := time.Now()
		c.cnt = port + now.Second()*100 + now.Nanosecond()/10000000
	} else {
		c.cnt++
	}

	return layers.UDPPort(c.cnt)
}
