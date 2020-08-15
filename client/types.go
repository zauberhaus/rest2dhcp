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

package client

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net"
	"net/http"
	"runtime"
	"time"

	"github.com/zauberhaus/rest2dhcp/dhcp"
	"gopkg.in/yaml.v3"
)

//ContentType of output
type ContentType string

// ContentType values
const (
	Unknown ContentType = "text/html"
	JSON                = "application/json"
	YAML                = "application/yaml"
	XML                 = "application/xml"
)

// Parse the content type from string
func (t *ContentType) Parse(val string) {
	switch val {
	case JSON:
		*t = JSON
	case YAML:
		*t = YAML
	case XML:
		*t = XML
	default:
		*t = Unknown
	}
}

func (t ContentType) String() string {
	return string(t)
}

// Lease is the result of a lease or renew request
type Lease struct {
	Hostname string    `yaml:"hostname" json:"hostname" xml:"hostname"`
	Mac      MAC       `yaml:"mac" json:"mac" xml:"mac"`
	IP       net.IP    `yaml:"ip" json:"ip" xml:"ip"`
	Mask     net.IP    `json:"mask,omitempty" xml:"mask,omitempty" yaml:"mask,omitempty"`
	DNS      net.IP    `json:"dns,omitempty" xml:"dns,omitempty" yaml:"dns,omitempty"`
	Router   net.IP    `json:"router,omitempty" xml:"router,omitempty" yaml:"router,omitempty"`
	Renew    time.Time `json:"renew" xml:"renew"`
	Rebind   time.Time `json:"rebind" xml:"rebind"`
	Expire   time.Time `json:"expire" xml:"expire"`
}

// NewLease initialises a new lease object from a DHCP package
func NewLease(hostname string, d dhcp.DHCP4) *Lease {
	return &Lease{
		Hostname: hostname,
		Mac:      MAC(d.ClientHWAddr),
		IP:       d.YourClientIP,
		Mask:     d.GetSubnetMask(),
		DNS:      d.GetDNS(),
		Router:   d.GetRouter(),
		Renew:    d.GetRenewalTime(),
		Rebind:   d.GetRebindTime(),
		Expire:   d.GetExpireTime(),
	}
}

// Error implementation wird status code
type Error struct {
	msg  string
	code int
}

// NewError intitialises a new error object
func NewError(code int, msg string) *Error {
	return &Error{
		msg:  msg,
		code: code,
	}
}

// Msg returns the error message
func (e *Error) Msg() string {
	return e.msg
}

// Code returns the status code
func (e *Error) Code() int {
	return e.code
}

func (e *Error) Error() string {
	return fmt.Sprintf("(%v %s) %s", e.code, http.StatusText(e.code), e.msg)
}

type MAC net.HardwareAddr

func (m *MAC) UnmarshalYAML(value *yaml.Node) error {
	mac, err := net.ParseMAC(value.Value)
	if err == nil {
		*m = MAC(mac)
	}

	return err
}

func (m *MAC) UnmarshalJSON(b []byte) error {
	var txt string
	err := json.Unmarshal(b, &txt)

	if err != nil {
		return err
	}

	mac, err := net.ParseMAC(txt)
	if err == nil {
		*m = MAC(mac)
	}

	return err
}

func (m *MAC) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var txt string
	if err := d.DecodeElement(&txt, &start); err != nil {
		return err
	}

	mac, err := net.ParseMAC(txt)
	if err == nil {
		*m = MAC(mac)
	}

	return err
}

func (m MAC) MarshalYAML() (interface{}, error) {
	return m.String(), nil
}

func (m MAC) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.String())
}

func (m MAC) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	return e.EncodeElement(m.String(), start)
}

func (m MAC) String() string {
	return net.HardwareAddr(m).String()
}

type Version struct {
	BuildDate    string              `yaml:"buildDate,omitempty" json:"buildDate,omitempty" xml:"buildDate,omitempty"`
	Compiler     string              `yaml:"compiler" json:"compiler" xml:"compiler"`
	GitCommit    string              `yaml:"gitCommit,omitempty" json:"gitCommit,omitempty" xml:"gitCommit,omitempty"`
	GitTreeState string              `yaml:"gitTreeState,omitempty" json:"gitTreeState,omitempty" xml:"gitTreeState,omitempty"`
	GitVersion   string              `yaml:"gitVersion,omitempty" json:"gitVersion,omitempty" xml:"gitVersion,omitempty"`
	GoVersion    string              `yaml:"goVersion" json:"goVersion" xml:"goVersion"`
	Platform     string              `yaml:"platform" json:"platform" xml:"platform"`
	DHCPServer   net.IP              `yaml:"dhcp,omitempty" json:"dhcp,omitempty" xml:"dhcp,omitempty"`
	RelayIP      net.IP              `yaml:"relay,omitempty" json:"relay,omitempty" xml:"relay,omitempty"`
	Mode         dhcp.ConnectionType `yaml:"mode,omitempty" json:"mode,omitempty" xml:"mode,omitempty"`
}

func (v *Version) String() string {
	data, _ := yaml.Marshal(v)
	return string(data)
}

func NewVersion(buildDate string, gitCommit string, tag string, treeState string) *Version {
	return &Version{
		BuildDate:    buildDate,
		Compiler:     runtime.Compiler,
		GitCommit:    gitCommit,
		GitTreeState: treeState,
		GitVersion:   tag,
		GoVersion:    runtime.Version(),
		Platform:     fmt.Sprintf("%v/%v", runtime.GOOS, runtime.GOARCH),
	}
}
