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
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net"

	"gopkg.in/yaml.v3"
)

// Connection is an interface for a DHCP connection
type Connection interface {
	Close() error
	Send(dhcp *DHCP4) (chan int, chan error)
	Receive() (chan *DHCP4, chan error)
	Local() *net.UDPAddr
	Remote() *net.UDPAddr

	Block(ctx context.Context) chan bool
}

// ConnectionType is enumeration of teh connection types
type ConnectionType string

// Existing connection types
const (
	AutoDetect   ConnectionType = "auto"
	DefaultRelay ConnectionType = "udp"
	Relay        ConnectionType = "packet"
	Fritzbox     ConnectionType = "fritzbox"
	BrokenRelay  ConnectionType = "broken"
)

// AllConnectionTypes is a list of all possible connection types
var AllConnectionTypes = []string{
	AutoDetect.String(),
	DefaultRelay.String(),
	Relay.String(),
	Fritzbox.String(),
	BrokenRelay.String(),
}

func (c ConnectionType) String() string {
	return string(c)
}

// Parse a string
func (c *ConnectionType) Parse(txt string) error {
	switch txt {
	case string(AutoDetect):
	case "":
		*c = AutoDetect
	case string(DefaultRelay):
		*c = DefaultRelay
	case string(Relay):
		*c = Relay
	case string(Fritzbox):
		*c = Fritzbox
	case string(BrokenRelay):
		*c = BrokenRelay
	default:
		return fmt.Errorf("Unknown connection type '%s'", txt)
	}

	return nil
}

// UnmarshalYAML custom unmarshal for YAMLv3
func (c *ConnectionType) UnmarshalYAML(value *yaml.Node) error {
	return c.Parse(value.Value)
}

// UnmarshalJSON custom unmarshal for JSON
func (c *ConnectionType) UnmarshalJSON(b []byte) error {
	var txt string
	err := json.Unmarshal(b, &txt)

	if err != nil {
		return err
	}

	return c.Parse(txt)
}

// UnmarshalXML custom unmarshal for XML
func (c *ConnectionType) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var txt string
	if err := d.DecodeElement(&txt, &start); err != nil {
		return err
	}

	return c.Parse(txt)
}

// MarshalYAML custom marshal for YAMLv3
func (c *ConnectionType) MarshalYAML() (interface{}, error) {
	return c.String(), nil
}

// MarshalJSON custom marshal for JSON
func (c *ConnectionType) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.String())
}

// MarshalXML custom marshal for XML
func (c *ConnectionType) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	return e.EncodeElement(c.String(), start)
}
