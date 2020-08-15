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
	cryptorand "crypto/rand"
	"encoding/binary"
	"log"
	"net"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// DHCP4 extends layers.DHCPv4 with helper functions
type DHCP4 struct {
	*layers.DHCPv4
}

// Lease extends DHCP4 with extra fields and an error
type Lease struct {
	*DHCP4
	Timestamp time.Time
	Hostname  string
	Done      chan bool
	err       error
}

// NewLease creates a lease object
func NewLease(msgType layers.DHCPMsgType, xid uint32, chaddr net.HardwareAddr, options layers.DHCPOptions) *Lease {
	dhcp := NewPackage(msgType, xid, chaddr, options)

	return &Lease{
		DHCP4: dhcp,
		Done:  make(chan bool, 10),
	}
}

// NewPackage creates a DHCPv4 package
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

// NewLeaseError creates an empty lease with an error
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

// SetError adds an error
func (l *Lease) SetError(err error) {
	l.err = err
}

// Ok returns false if there is an error
func (l *Lease) Ok() bool {
	return l.err == nil
}

// Touch updates the timestamp
func (l *Lease) Touch() {
	l.Timestamp = time.Now()
}

// CheckResponseType checks if a new status is allowed
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

// SetMsgType sets the DHCP message type option
func (d *DHCP4) SetMsgType(msgType layers.DHCPMsgType) {
	d.SetOption(layers.DHCPOptMessageType, []byte{uint8(msgType)})
}

// GetMsgType returns the DHCP message type option
func (d *DHCP4) GetMsgType() layers.DHCPMsgType {
	o := d.GetOption(layers.DHCPOptMessageType)
	if o == nil {
		return layers.DHCPMsgTypeUnspecified
	}

	return layers.DHCPMsgType(o.Data[0])
}

// GetOption returns a requested DHCP option
func (d *DHCP4) GetOption(option layers.DHCPOpt) *layers.DHCPOption {

	for _, v := range d.Options {
		if v.Type == option {
			return &v
		}
	}

	return nil
}

// SetOption sets a DHCP option
func (d *DHCP4) SetOption(opt layers.DHCPOpt, data []byte) {

	pos := d.findOption(opt)

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

func (d *DHCP4) findOption(option layers.DHCPOpt) int {

	for i, v := range d.Options {
		if v.Type == option {
			return i
		}
	}

	return -1
}

// GetExpireTime return the option layers.DHCPOptLeaseTime
func (d *DHCP4) GetExpireTime() time.Time {

	expire := time.Now()

	o := d.GetOption(layers.DHCPOptLeaseTime)
	if o != nil {
		duration := time.Duration(binary.BigEndian.Uint32(o.Data)) * time.Second
		expire = expire.Add(duration)
	}

	return expire
}

// GetRebindTime return the option layers.DHCPOptT2
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

// GetRenewalTime return the option layers.DHCPOptT1
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

// GetSubnetMask return the option layers.DHCPOptSubnetMask
func (d *DHCP4) GetSubnetMask() net.IP {
	o := d.GetOption(layers.DHCPOptSubnetMask)
	if o != nil && o.Length == 4 {
		return o.Data
	}

	return nil
}

// GetDNS return the option layers.DHCPOptDNS
func (d *DHCP4) GetDNS() net.IP {
	o := d.GetOption(layers.DHCPOptDNS)
	if o != nil && o.Length == 4 {
		return o.Data
	}

	return nil
}

// GetRouter return the option layers.DHCPOptRouter
func (d *DHCP4) GetRouter() net.IP {
	o := d.GetOption(layers.DHCPOptRouter)
	if o != nil && o.Length == 4 {
		return o.Data
	}

	return nil
}

func (d *DHCP4) serialize() []byte {
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

// SetHostname sets the DHCP option layers.DHCPOptHostname
func (d *DHCP4) SetHostname(hostname string) {
	d.SetOption(layers.DHCPOptHostname, []byte(hostname))
}

// SetHostname sets the DHCP option layers.DHCPOptHostname and the hostname field
func (l *Lease) SetHostname(hostname string) {
	l.Hostname = hostname
	l.DHCP4.SetHostname(hostname)
}

// GetRequest creates a DHCP request out of an lease
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

// GenerateXID generates a random uint32 value
func GenerateXID() uint32 {
	buf := make([]byte, 4)
	if _, err := cryptorand.Read(buf); err != nil {
		log.Fatal(err)
	}

	return binary.LittleEndian.Uint32(buf)
}
