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

package dhcp_test

import (
	"encoding/json"
	"encoding/xml"
	"net"
	"testing"

	"github.com/google/gopacket/layers"
	"github.com/stretchr/testify/assert"
	"github.com/zauberhaus/rest2dhcp/dhcp"
	"gopkg.in/yaml.v3"
)

func getTestAddr() (*net.UDPAddr, *net.UDPAddr) {
	local := &net.UDPAddr{
		IP:   net.ParseIP("1.1.1.1"),
		Port: 2000,
	}

	remote := &net.UDPAddr{
		IP:   net.ParseIP("2.2.2.2"),
		Port: 3000,
	}

	return local, remote
}

func getDHCP4() *dhcp.DHCP4 {
	mac, _ := net.ParseMAC("00:01:02:03:04:05")
	lease := dhcp.NewLease(layers.DHCPMsgTypeDiscover, 1999, mac, nil)
	return lease.DHCP4
}

func TestConnectionType_String(t *testing.T) {
	tests := []struct {
		c    dhcp.ConnectionType
		want string
	}{
		{
			c:    dhcp.AutoDetect,
			want: "auto",
		},
		{
			c:    dhcp.UDP,
			want: "udp",
		},
		{
			c:    dhcp.Dual,
			want: "dual",
		},
		{
			c:    dhcp.Fritzbox,
			want: "fritzbox",
		},
		{
			c:    dhcp.Broken,
			want: "broken",
		},
		{
			c:    dhcp.Packet,
			want: "packet",
		},
	}
	for _, tt := range tests {
		t.Run(tt.c.String(), func(t *testing.T) {
			if got := tt.c.String(); got != tt.want {
				t.Errorf("ConnectionType.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConnectionType_Parse(t *testing.T) {
	var c dhcp.ConnectionType

	tests := []struct {
		txt     string
		want    dhcp.ConnectionType
		wantErr bool
	}{
		{
			txt:  "",
			want: dhcp.AutoDetect,
		},
		{
			txt:  "udp",
			want: dhcp.UDP,
		},
		{
			txt:  "dual",
			want: dhcp.Dual,
		},
		{
			txt:  "fritzbox",
			want: dhcp.Fritzbox,
		},
		{
			txt:  "broken",
			want: dhcp.Broken,
		},
		{
			txt:  "packet",
			want: dhcp.Packet,
		},
		{
			txt:     "dummy",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.want.String(), func(t *testing.T) {
			err := c.Parse(tt.txt)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, c, tt.want)
			}
		})
	}
}

func TestConnectionType_UnmarshalYAML(t *testing.T) {
	type data struct {
		CT dhcp.ConnectionType
	}

	txt := "ct: udp\n"
	result := data{}

	err := yaml.Unmarshal([]byte(txt), &result)

	if assert.NoError(t, err) {
		assert.Equal(t, dhcp.UDP, result.CT)
	}
}

func TestConnectionType_UnmarshalInvalidYAML(t *testing.T) {
	type data struct {
		CT dhcp.ConnectionType
	}

	result := data{}

	err := yaml.Unmarshal([]byte("xzy"), &result)

	assert.EqualError(t, err, "yaml: unmarshal errors:\n  line 1: cannot unmarshal !!str `xzy` into dhcp_test.data")

	err = yaml.Unmarshal([]byte("ct: xyz\n"), &result)

	assert.EqualError(t, err, "Unknown connection type 'xyz'")
}

func TestConnectionType_UnmarshalJSON(t *testing.T) {
	type data struct {
		CT dhcp.ConnectionType
	}

	txt := "{\"CT\":\"udp\"}"
	result := data{}

	err := json.Unmarshal([]byte(txt), &result)

	if assert.NoError(t, err) {
		assert.Equal(t, dhcp.UDP, result.CT)
	}
}

func TestConnectionType_UnmarshalInvalidJSON(t *testing.T) {
	type data struct {
		CT dhcp.ConnectionType
	}

	result := data{}

	err := json.Unmarshal([]byte("xzy"), &result)

	assert.EqualError(t, err, "invalid character 'x' looking for beginning of value")

	err = json.Unmarshal([]byte("{\"CT\":\"xyz\"}"), &result)

	assert.EqualError(t, err, "Unknown connection type 'xyz'")
}

func TestConnectionType_UnmarshalXML(t *testing.T) {
	type data struct {
		CT dhcp.ConnectionType
	}

	txt := "<data><CT>udp</CT></data>"
	result := data{}

	err := xml.Unmarshal([]byte(txt), &result)

	if assert.NoError(t, err) {
		assert.Equal(t, dhcp.UDP, result.CT)
	}
}

func TestConnectionType_UnmarshalInvalidXML(t *testing.T) {
	type data struct {
		CT dhcp.ConnectionType
	}

	result := data{}

	err := xml.Unmarshal([]byte("xzy"), &result)

	assert.EqualError(t, err, "EOF")

	err = xml.Unmarshal([]byte("<data><CT>xyz</CT></data>"), &result)

	assert.EqualError(t, err, "Unknown connection type 'xyz'")
}

func TestConnectionType_MarshalYAML(t *testing.T) {
	var m yaml.Marshaler = dhcp.UDP

	buffer, err := yaml.Marshal(m)

	if assert.NoError(t, err) {
		txt := string(buffer)
		assert.Equal(t, "udp\n", txt)
	}
}

func TestConnectionType_MarshalJSON(t *testing.T) {
	var m json.Marshaler = dhcp.UDP

	buffer, err := json.Marshal(m)

	if assert.NoError(t, err) {
		txt := string(buffer)
		assert.Equal(t, "\"udp\"", txt)
	}
}

func TestConnectionType_MarshalXML(t *testing.T) {
	var m xml.Marshaler = dhcp.UDP

	buffer, err := xml.Marshal(m)

	if assert.NoError(t, err) {
		txt := string(buffer)
		assert.Equal(t, "<ConnectionType>udp</ConnectionType>", txt)
	}
}

func TestKubeServiceConfig_String(t *testing.T) {
	c := &dhcp.KubeServiceConfig{
		Config:    "./kube.config",
		Namespace: "default",
		Service:   "rest2dhcp",
	}

	got := c.String()
	assert.Equal(t, got, "config: ./kube.config\nnamespace: default\nservice: rest2dhcp\n")
}
