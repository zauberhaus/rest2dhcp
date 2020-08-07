package main

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/zauberhaus/rest2dhcp/client"
	"github.com/zauberhaus/rest2dhcp/service"
	"gopkg.in/yaml.v3"
)

var (
	url        = "http://localhost:8080/ip/"
	versionURL = "http://localhost:8080/version"
	metricsURL = "http://localhost:8080/metrics"
)

func check() bool {
	resp, err := http.Get(versionURL)
	if err != nil {
		return true
	}

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Can't read version: %s (%v)", resp.Status, resp.StatusCode)
		os.Exit(1)
	}

	return false
}

func setup() (*service.Server, context.CancelFunc) {
	server := service.NewServer(nil, nil, client.AutoDetect, ":8080", 30*time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	server.Start(ctx)
	return server, cancel
}

func TestMain(m *testing.M) {
	start := check()

	if start {
		server, cancel := setup()
		code := m.Run()
		cancel()
		<-server.Done
		os.Exit(code)
	} else {
		code := m.Run()
		os.Exit(code)
	}
}

func TestService(t *testing.T) {
	t.Run("TestWorkflowYAML", DHCPWorkflow(service.YAML))
	t.Run("TestWorkflowJSON", DHCPWorkflow(service.JSON))
	t.Run("TestWorkflowXML", DHCPWorkflow(service.XML))
}

func TestVersion(t *testing.T) {
	t.Run("TestVersionYAML", Version(service.YAML))
	t.Run("TestVersionJSON", Version(service.JSON))
	t.Run("TestVersionXML", Version(service.XML))
}

func TestMetrics(t *testing.T) {
}

func DHCPWorkflow(mime service.ContentType) func(t *testing.T) {
	return func(t *testing.T) {

		hostname := "test"

		resp := request(t, "GET", url+hostname, mime)
		result := checkResult(t, resp, hostname, mime)

		resp2 := request(t, "GET", url+hostname+"/"+result.Mac.String(), mime)
		result2 := checkResult(t, resp2, hostname, mime)

		if result.IP != result2.IP {
			t.Fatalf("Different IP's %v != %v", result.IP, result2.IP)
		}

		resp = request(t, "GET", url+hostname+"/"+result.Mac.String()+"/"+result.IP, mime)
		result = checkResult(t, resp, hostname, mime)

		resp = request(t, "DELETE", url+hostname+"/"+result.Mac.String()+"/"+result.IP, mime)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Wrong http status: %v", resp.Status)
		}
	}
}

func Version(mime service.ContentType) func(t *testing.T) {
	return func(t *testing.T) {
		resp := request(t, "GET", versionURL, mime)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Wrong http status: %v", resp.Status)
		}

		value := resp.Header.Get("Content-Type")
		if service.ContentType(value) != mime {
			t.Fatalf("Wrong content type: %v != %v", value, mime)
		}

		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("%v", err)
		}

		var version service.VersionInfo
		unmarshal(t, data, &version, mime)

		if version.ServiceVersion.GitCommit == "" {
			t.Errorf("Invalid GitCommit")
		}

	}
}

func request(t *testing.T, method string, url string, mime service.ContentType) *http.Response {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		t.Fatalf("%v", err)
	}

	req.Header.Set("Accept", string(mime))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%v", err)
	}

	return resp
}

func checkResult(t *testing.T, resp *http.Response, hostname string, mime service.ContentType) *service.Result {
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Wrong http status: %v", resp.Status)
	}

	value := resp.Header.Get("Content-Type")
	if service.ContentType(value) != mime {
		t.Fatalf("Wrong content type: %v != %v", value, mime)
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("%v", err)
	}

	var result service.Result
	unmarshal(t, data, &result, mime)

	if result.Hostname != hostname {
		t.Fatalf("Wrong hostname: '%v' != '%v'", result.Hostname, hostname)
	}

	ip := net.ParseIP(result.IP)
	if ip == nil {
		t.Fatalf("Invalid return ip")
	}

	if result.Mac.HardwareAddr == nil {
		t.Fatalf("Empty mac")
	}

	resp.Body.Close()
	return &result
}

func unmarshal(t *testing.T, data []byte, result interface{}, mime service.ContentType) {
	switch mime {
	case service.YAML:
		err := yaml.Unmarshal(data, result)
		if err != nil {
			t.Fatalf("%v", err)
			return
		}
	case service.JSON:
		err := json.Unmarshal(data, result)
		if err != nil {
			t.Fatalf("%v", err)
			return
		}
	case service.XML:
		err := xml.Unmarshal(data, result)
		if err != nil {
			t.Fatalf("%v", err)
			return
		}
	}
}
