package service

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net"
	"net/http"
	"runtime"
	"time"

	"github.com/gorilla/mux"
	"gopkg.in/yaml.v3"
)

type Key byte

const (
	//Content key in context
	Content Key = iota
)

//ContentType of output
type ContentType byte

const (
	Unknown ContentType = iota
	JSON
	YAML
	XML
)

type Query struct {
	Hostname string `json:"hostname" xml:"hostname"`
	Mac      MAC    `json:"mac" xml:"mac"`
	IP       net.IP `json:"ip" xml:"ip"`
}

func NewQuery(r *http.Request) (*Query, error) {
	query := Query{}

	vars := mux.Vars(r)
	hostname, ok := vars["hostname"]
	if ok {
		if hostnameExp.MatchString(hostname) {
			query.Hostname = hostname
		} else {
			return nil, fmt.Errorf("Invalid hostname")
		}
	}

	mac, ok := vars["mac"]
	if ok {
		tmp, err := net.ParseMAC(mac)
		if err == nil {
			query.Mac = MAC{tmp}
		}
	}

	ip, ok := vars["ip"]
	if ok {
		tmp := net.ParseIP(ip)
		if tmp == nil {
			return nil, fmt.Errorf("Invalid IP format")
		}

		query.IP = tmp
	}

	return &query, nil
}

type Lease struct {
	XMLName  xml.Name  `xml:"lease" json:"-" yaml:"-"`
	Hostname string    `json:"hostname" xml:"hostname"`
	Mac      MAC       `json:"mac" xml:"mac"`
	Cid      string    `json:"client-id,omitempty" xml:"client-id,omitempty" yaml:"client-id,omitempty"`
	IP       string    `json:"ip" xml:"ip"`
	Renew    time.Time `json:"renew" xml:"renew"`
	Expire   time.Time `json:"expire" xml:"expire"`
}

type Result struct {
	Lease `json:"lease" xml:"lease"`
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
	ServiceVersion Version  `yaml:"rest2dhcp" xml:"rest2dhcp" json:"rest2dhcp"`
}

type Version struct {
	BuildDate    string `yaml:"buildDate" json:"buildDate" xml:"build-date"`
	Compiler     string `yaml:"compiler" json:"compiler" xml:"compiler"`
	GitCommit    string `yaml:"gitCommit" json:"gitCommit" xml:"git-commit"`
	GitTreeState string `yaml:"gitTreeState" json:"gitTreeState" xml:"git-tree-state"`
	GitVersion   string `yaml:"gitVersion" json:"gitVersion" xml:"git-version"`
	GoVersion    string `yaml:"goVersion" json:"goVersion" xml:"go-version"`
	Platform     string `yaml:"platform" json:"platform" xml:"platform"`
}

func NewVersionInfo(buildDate string, gitCommit string, tag string, treeState string) *VersionInfo {
	return &VersionInfo{
		ServiceVersion: Version{
			BuildDate:    buildDate,
			Compiler:     runtime.Compiler,
			GitCommit:    gitCommit,
			GitTreeState: treeState,
			GitVersion:   tag,
			GoVersion:    runtime.Version(),
			Platform:     fmt.Sprintf("%v/%v", runtime.GOOS, runtime.GOARCH),
		},
	}
}
