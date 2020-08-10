package service_test

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/zauberhaus/rest2dhcp/client"
	"github.com/zauberhaus/rest2dhcp/dhcp"
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
	server := service.NewServer(nil, nil, nil, dhcp.AutoDetect, ":8080", 30*time.Second)
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
	t.Run("TestWorkflowYAML", DHCPWorkflow(client.YAML))
	t.Run("TestWorkflowJSON", DHCPWorkflow(client.JSON))
	t.Run("TestWorkflowXML", DHCPWorkflow(client.XML))
}

func TestVersion(t *testing.T) {
	t.Run("TestVersionYAML", Version(client.YAML))
	t.Run("TestVersionJSON", Version(client.JSON))
	t.Run("TestVersionXML", Version(client.XML))
}

func TestMetrics(t *testing.T) {
}

func DHCPWorkflow(mime client.ContentType) func(t *testing.T) {
	return func(t *testing.T) {

		hostname := "client1"

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
	}
}

func Version(mime client.ContentType) func(t *testing.T) {
	return func(t *testing.T) {
		resp := request(t, "GET", versionURL, mime)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Wrong http status: %v", resp.Status)
		}

		value := resp.Header.Get("Content-Type")
		if client.ContentType(value) != mime {
			t.Fatalf("Wrong content type: %v != %v", value, mime)
		}

		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("%v", err)
		}

		var version client.VersionInfo
		unmarshal(t, data, &version, mime)

		if version.ServiceVersion.GitCommit == "" {
			t.Errorf("Invalid GitCommit")
		}

	}
}

func request(t *testing.T, method string, url string, mime client.ContentType) *http.Response {
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

func checkResult(t *testing.T, resp *http.Response, hostname string, mime client.ContentType) *client.Result {
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Wrong http status: %v", resp.Status)
	}

	value := resp.Header.Get("Content-Type")
	if client.ContentType(value) != mime {
		t.Fatalf("Wrong content type: %v != %v", value, mime)
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("%v", err)
	}

	var result client.Result
	unmarshal(t, data, &result, mime)

	if result.Hostname != hostname {
		t.Fatalf("Wrong hostname: '%v' != '%v'", result.Hostname, hostname)
	}

	if result.IP == nil {
		t.Fatalf("Invalid return ip")
	}

	if result.Mac.HardwareAddr == nil {
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
