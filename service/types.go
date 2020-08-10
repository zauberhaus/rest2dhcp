package service

import (
	"fmt"
	"net"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/zauberhaus/rest2dhcp/client"
)

type Key byte

const (
	//Content key in context
	Content Key = iota
)

type Query struct {
	Hostname string     `json:"hostname" xml:"hostname"`
	Mac      client.MAC `json:"mac" xml:"mac"`
	IP       net.IP     `json:"ip" xml:"ip"`
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
			query.Mac = client.MAC{tmp}
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
