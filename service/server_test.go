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

package service_test

import (
	"encoding/json"
	"encoding/xml"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zauberhaus/rest2dhcp/client"
	"github.com/zauberhaus/rest2dhcp/service"
	test_test "github.com/zauberhaus/rest2dhcp/test"
	"gopkg.in/yaml.v3"
)

const (
	url        = "http://localhost:8080/ip/"
	versionURL = "http://localhost:8080/version"
	metricsURL = "http://localhost:8080/metrics"

	buildDate    = "2020-08-11T10:06:44NZST"
	gitCommit    = "03fd9a8658c81c088fb548cc43b56703e6ee145b"
	gitVersion   = "v0.0.1"
	gitTreeState = "dirty"
)

var (
	server = test_test.TestServer{}
)

func TestMain(m *testing.M) {
	server.Run(m)
}

func TestService(t *testing.T) {
	testCases := []struct {
		Name     string
		Hostname string
		Mime     client.ContentType
	}{
		{
			Name:     "Test workflow via HTTP request with content type XML",
			Hostname: "srever-test-xml",
			Mime:     client.XML,
		},
		{
			Name:     "Test workflow via HTTP request with content type YAML",
			Hostname: "server-test-yaml",
			Mime:     client.YAML,
		},
		{
			Name:     "Test workflow via HTTP request with content type JSON",
			Hostname: "server-test-json",
			Mime:     client.JSON,
		},
		{
			Name:     "Test workflow via HTTP request without a content type",
			Hostname: "server-test-unknown",
			Mime:     client.Unknown,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			mime := tc.Mime

			hostname := tc.Hostname

			resp := request(t, "GET", url+hostname, mime)
			result := checkResult(t, resp, hostname, mime)

			resp2 := request(t, "GET", url+hostname+"/"+result.Mac.String(), mime)
			result2 := checkResult(t, resp2, hostname, mime)

			if result.IP.String() != result2.IP.String() {
				t.Fatalf("Different IP's %v != %v", result.IP, result2.IP)
			}

			resp = request(t, "GET", url+hostname+"/"+result.Mac.String()+"/"+result.IP.String(), mime)
			result = checkResult(t, resp, hostname, mime)

			resp = request(t, "DELETE", url+hostname+"/"+result.Mac.String()+"/"+result.IP.String(), mime)

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("Wrong http status: %v", resp.Status)
			}
		})
	}
}

func TestVersion(t *testing.T) {
	testCases := []struct {
		Name string
		Mime client.ContentType
	}{
		{
			Name: "Read version via HTTP request with content type XML",
			Mime: client.XML,
		},
		{
			Name: "Read version via HTTP request with content type YAML",
			Mime: client.YAML,
		},
		{
			Name: "Read version via HTTP request with content type JSON",
			Mime: client.JSON,
		},
		{
			Name: "Read version via HTTP request without a content type",
			Mime: client.Unknown,
		},
	}

	for _, tc := range testCases {
		tc := tc // We run our tests twice one with this line & one without
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			mime := tc.Mime

			resp := request(t, "GET", versionURL, mime)

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("Wrong http status: %v", resp.Status)
			}

			if mime == client.Unknown {
				mime = client.YAML
			}

			value := resp.Header.Get("Content-Type")
			if client.ContentType(value) != mime {
				t.Fatalf("Wrong content type: %v != %v", value, mime)
			}

			data, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("%v", err)
			}

			var version client.Version
			unmarshal(t, data, &version, mime)

			if server.IsStarted() {
				assert.Equal(t, service.Version, &version, "Invalid Version info")
			} else {
				if version.GitCommit == "" {
					t.Errorf("Invalid version info:\n%v", version)
				}
			}
		})
	}
}

func TestUnsupportedMediaType(t *testing.T) {
	req, err := http.NewRequest("GET", versionURL, nil)
	if err != nil {
		t.Fatalf("%v", err)
	}

	req.Header.Set("Accept", "text/dummy")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%v", err)
	}

	if resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("Unexpected response status: %v", resp.Status)
	}
}

func TestMetrics(t *testing.T) {
}

func request(t *testing.T, method string, url string, mime client.ContentType) *http.Response {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		t.Fatalf("%v", err)
	}

	if mime != client.Unknown {
		req.Header.Set("Accept", string(mime))
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%v", err)
	}

	return resp
}

func checkResult(t *testing.T, resp *http.Response, hostname string, mime client.ContentType) *client.Lease {
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Wrong http status: %v", resp.Status)
	}

	if mime == client.Unknown {
		mime = client.YAML
	}

	value := resp.Header.Get("Content-Type")
	if client.ContentType(value) != mime {
		t.Fatalf("Wrong content type: %v != %v", value, mime)
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("%v", err)
	}

	var result client.Lease
	unmarshal(t, data, &result, mime)

	if result.Hostname != hostname {
		t.Fatalf("Wrong hostname: '%v' != '%v'", result.Hostname, hostname)
	}

	if result.IP == nil {
		t.Fatalf("Invalid return ip")
	}

	if result.Mac == nil {
		t.Fatalf("Empty mac")
	}

	resp.Body.Close()
	return &result
}

func unmarshal(t *testing.T, data []byte, result interface{}, mime client.ContentType) {
	switch mime {
	case client.YAML:
		err := yaml.Unmarshal(data, result)
		if err != nil {
			t.Fatalf("%v", err)
			return
		}
	case client.JSON:
		err := json.Unmarshal(data, result)
		if err != nil {
			t.Fatalf("%v", err)
			return
		}
	case client.XML:
		err := xml.Unmarshal(data, result)
		if err != nil {
			t.Fatalf("%v", err)
			return
		}
	}
}
