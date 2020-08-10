package dhcp

import (
	cryptorand "crypto/rand"
	"encoding/binary"
	"log"
	"net"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

type DHCP4 struct {
	*layers.DHCPv4
}

type Lease struct {
	*DHCP4
	Timestamp time.Time
	Hostname  string
	Done      chan bool
	err       error
}

func NewLease(msgType layers.DHCPMsgType, xid uint32, chaddr net.HardwareAddr, options layers.DHCPOptions) *Lease {
	dhcp := NewPackage(msgType, xid, chaddr, options)

	return &Lease{
		DHCP4: dhcp,
		Done:  make(chan bool, 10),
	}
}

func NewPackage(msgType layers.DHCPMsgType, xid uint32, chaddr net.HardwareAddr, options layers.DHCPOptions) *DHCP4 {
	if xid == 0 {
		xid = GenerateXID()
	}

	packet := layers.DHCPv4{
		Operation:    layers.DHCPOpRequest,
		HardwareType: layers.LinkTypeEthernet,
		ClientHWAddr: chaddr,
		Xid:          xid,
	}

	packet.Options = append(packet.Options, layers.DHCPOption{
		Type:   layers.DHCPOptMessageType,
		Data:   []byte{byte(msgType)},
		Length: 1,
	})

	// append DHCP options
	for _, option := range options {
		packet.Options = append(packet.Options, layers.DHCPOption{
			Type:   option.Type,
			Data:   option.Data,
			Length: uint8(len(option.Data)),
		})
	}

	return &DHCP4{
		&packet,
	}
}

func NewLeaseError(err error) *Lease {
	return &Lease{
		DHCP4: nil,
		Done:  nil,
		err:   err,
	}
}

func (l *Lease) Error() error {
	return l.err
}

func (l *Lease) SetError(err error) {
	l.err = err
}

func (l *Lease) Ok() bool {
	return l.err == nil
}

func (l *Lease) Touch() {
	l.Timestamp = time.Now()
}

func (l *Lease) CheckResponseType(dhcp *DHCP4) bool {
	old := l.GetMsgType()
	new := dhcp.GetMsgType()

	if old == layers.DHCPMsgTypeDiscover && new == layers.DHCPMsgTypeOffer {
		return true
	} else if old == layers.DHCPMsgTypeRequest && (new == layers.DHCPMsgTypeAck || new == layers.DHCPMsgTypeNak) {
		return true
	} else if old == layers.DHCPMsgTypeInform && (new == layers.DHCPMsgTypeAck || new == layers.DHCPMsgTypeNak) {
		return true
	}

	return false
}

func (d *DHCP4) SetMsgType(msgType layers.DHCPMsgType) {
	d.SetOption(layers.DHCPOptMessageType, []byte{uint8(msgType)})
}

func (d *DHCP4) GetMsgType() layers.DHCPMsgType {
	o := d.GetOption(layers.DHCPOptMessageType)
	if o == nil {
		return layers.DHCPMsgTypeUnspecified
	}

	return layers.DHCPMsgType(o.Data[0])
}

func (d *DHCP4) GetOption(option layers.DHCPOpt) *layers.DHCPOption {

	for _, v := range d.Options {
		if v.Type == option {
			return &v
		}
	}

	return nil
}

func (d *DHCP4) FindOption(option layers.DHCPOpt) int {

	for i, v := range d.Options {
		if v.Type == option {
			return i
		}
	}

	return -1
}

func (d *DHCP4) GetExpireTime() time.Time {

	expire := time.Now()

	o := d.GetOption(layers.DHCPOptLeaseTime)
	if o != nil {
		duration := time.Duration(binary.BigEndian.Uint32(o.Data)) * time.Second
		expire = expire.Add(duration)
	}

	return expire
}

func (d *DHCP4) GetRebindTime() time.Time {

	renewal := time.Now()

	o := d.GetOption(layers.DHCPOptT2)
	if o != nil {
		duration := time.Duration(binary.BigEndian.Uint32(o.Data)) * time.Second
		renewal = renewal.Add(duration)
	} else {
		o := d.GetOption(layers.DHCPOptLeaseTime)
		if o != nil {
			duration := time.Duration(binary.BigEndian.Uint32(o.Data)*7/8) * time.Second
			renewal = renewal.Add(duration)
		}
	}

	return renewal
}

func (d *DHCP4) GetRenewalTime() time.Time {

	renewal := time.Now()

	o := d.GetOption(layers.DHCPOptT1)
	if o != nil {
		duration := time.Duration(binary.BigEndian.Uint32(o.Data)) * time.Second
		renewal = renewal.Add(duration)
	} else {
		o := d.GetOption(layers.DHCPOptLeaseTime)
		if o != nil {
			duration := time.Duration(binary.BigEndian.Uint32(o.Data)/2) * time.Second
			renewal = renewal.Add(duration)
		}
	}

	return renewal
}

func (d *DHCP4) GetSubnetMask() net.IP {
	o := d.GetOption(layers.DHCPOptSubnetMask)
	if o != nil && o.Length == 4 {
		return o.Data
	}

	return nil
}

func (d *DHCP4) GetDNS() net.IP {
	o := d.GetOption(layers.DHCPOptDNS)
	if o != nil && o.Length == 4 {
		return o.Data
	}

	return nil
}

func (d *DHCP4) GetRouter() net.IP {
	o := d.GetOption(layers.DHCPOptRouter)
	if o != nil && o.Length == 4 {
		return o.Data
	}

	return nil
}

func (d *DHCP4) SetOption(opt layers.DHCPOpt, data []byte) {

	pos := d.FindOption(opt)

	if pos >= 0 {
		d.Options[pos].Data = data
		d.Options[pos].Length = uint8(len(data))
	} else {
		d.Options = append(d.Options, layers.DHCPOption{
			Type:   opt,
			Data:   data,
			Length: uint8(len(data)),
		})
	}
}

func (d *DHCP4) Serialize() []byte {
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		ComputeChecksums: true,
		FixLengths:       true,
	}

	err := d.SerializeTo(buf, opts)
	if err != nil {
		log.Fatal(err)
	}

	l := len(buf.Bytes())
	if l <= 300 {
		buf.AppendBytes(301 - l)
	}

	return buf.Bytes()
}

func (d *DHCP4) SetHostname(hostname string) {
	d.SetOption(layers.DHCPOptHostname, []byte(hostname))
}

func (l *Lease) SetHostname(hostname string) {
	l.Hostname = hostname
	l.DHCP4.SetHostname(hostname)
}

func (l *Lease) GetRequest(msgType layers.DHCPMsgType, options layers.DHCPOptions) *DHCP4 {
	lease := NewPackage(msgType, l.Xid, l.ClientHWAddr, options)

	lease.RelayAgentIP = l.RelayAgentIP

	if l.Hostname != "" {
		lease.SetHostname(l.Hostname)
	}

	if msgType != layers.DHCPMsgTypeRequest {
		lease.ClientIP = l.YourClientIP
	} else {
		lease.Options = append(lease.Options, layers.DHCPOption{
			Type:   layers.DHCPOptRequestIP,
			Data:   l.YourClientIP.To4(),
			Length: 4,
		})
	}

	return lease
}

func GenerateXID() uint32 {
	buf := make([]byte, 4)
	if _, err := cryptorand.Read(buf); err != nil {
		log.Fatal(err)
	}

	return binary.LittleEndian.Uint32(buf)
}
