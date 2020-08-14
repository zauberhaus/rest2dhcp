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

package service

import (
	"fmt"
	"net"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/zauberhaus/rest2dhcp/client"
)

// Key is the context key
type Key byte

//Context keys
const (
	Content Key = iota
)

// Query contains the request parametes
type Query struct {
	Hostname string     `json:"hostname" xml:"hostname"`
	Mac      client.MAC `json:"mac" xml:"mac"`
	IP       net.IP     `json:"ip" xml:"ip"`
}

// NewQuery creates a new object from a http request
// * @param request - the http.Request
func NewQuery(request *http.Request) (*Query, error) {
	query := Query{}

	vars := mux.Vars(request)
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
			query.Mac = client.MAC(tmp)
		} else {
			return nil, fmt.Errorf("Invalid MAC format")
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
