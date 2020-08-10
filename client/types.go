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

const (
	Unknown ContentType = "text/html"
	JSON                = "application/json"
	YAML                = "application/yaml"
	XML                 = "application/xml"
)

func (t ContentType) String() string {
	return string(t)
}

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

type Lease struct {
	XMLName  xml.Name  `xml:"lease" json:"-" yaml:"-"`
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

func NewLease(hostname string, d dhcp.DHCP4) *Lease {
	return &Lease{
		Hostname: hostname,
		Mac:      MAC{d.ClientHWAddr},
		IP:       d.YourClientIP,
		Mask:     d.GetSubnetMask(),
		DNS:      d.GetDNS(),
		Router:   d.GetRouter(),
		Renew:    d.GetRenewalTime(),
		Rebind:   d.GetRebindTime(),
		Expire:   d.GetExpireTime(),
	}
}

type Result struct {
	*Lease `json:"lease" xml:"lease" ymal:"lease"`
}

type ClientError struct {
	Msg  string
	Code int
}

func NewError(code int, msg string) *ClientError {
	return &ClientError{
		Msg:  msg,
		Code: code,
	}
}

func (e *ClientError) Error() string {
	return fmt.Sprintf("(%v %s) %s", e.Code, http.StatusText(e.Code), e.Msg)
}

type MAC struct {
	net.HardwareAddr
}

func (m *MAC) UnmarshalYAML(value *yaml.Node) error {
	mac, err := net.ParseMAC(value.Value)
	if err == nil {
		m.HardwareAddr = mac
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
		m.HardwareAddr = mac
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
		m.HardwareAddr = mac
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

type VersionInfo struct {
	XMLName        xml.Name `xml:"version" json:"-" yaml:"-"`
	ServiceVersion *Version `yaml:"rest2dhcp" xml:"rest2dhcp" json:"rest2dhcp"`
}

type Version struct {
	BuildDate    string              `yaml:"buildDate,omitempty" json:"buildDate,omitempty" xml:"build-date,omitempty"`
	Compiler     string              `yaml:"compiler" json:"compiler" xml:"compiler"`
	GitCommit    string              `yaml:"gitCommit,omitempty" json:"gitCommit,omitempty" xml:"git-commit,omitempty"`
	GitTreeState string              `yaml:"gitTreeState,omitempty" json:"gitTreeState,omitempty" xml:"git-tree-state,omitempty"`
	GitVersion   string              `yaml:"gitVersion,omitempty" json:"gitVersion,omitempty" xml:"git-version,omitempty"`
	GoVersion    string              `yaml:"goVersion" json:"goVersion" xml:"go-version"`
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
