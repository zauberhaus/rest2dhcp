package main

import (
	"context"
	"encoding/json"
	"encoding/xml"
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

type MimeType string

var (
	yamlMime MimeType = "application/yaml"
	jsonMime MimeType = "application/json"
	xmlMime  MimeType = "application/xml"
)

func setup() (*service.Server, context.CancelFunc) {
	server := service.NewServer(nil, nil, client.AutoDetect, ":8080", 30*time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	server.Start(ctx)
	return server, cancel
}

func TestMain(m *testing.M) {
	start := len(os.Getenv("SERVER")) > 0

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

/*
func TestLeaseHostname(t *testing.T) {
	hostname := "test"

	req, err := http.NewRequest("GET", "http://localhost:8080/"+hostname, nil)
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	ctype := "application/json"

	req.Header.Set("Accept", ctype)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	defer resp.Body.Close()

	result := checkResult(t, resp, hostname, ctype)

	req, err = http.NewRequest("GET", "http://localhost:8080/"+hostname+"/"+result.Mac.String()+"/"+result.IP, nil)
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	req.Header.Set("Accept", ctype)

	resp2, err := client.Do(req)
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	defer resp2.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Wrong http status: %v", resp.Status)
		return
	}

	value = resp.Header.Get("Content-Type")
	if value != ctype {
		t.Errorf("Wrong content type: %v != %v", value, ctype)
		return
	}

	data, err = ioutil.ReadAll(resp2.Body)
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	err = json.Unmarshal(data, &result)
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	if result.Hostname != hostname {
		t.Errorf("Wrong hostname: '%v' != '%v'", result.Hostname, hostname)
		return
	}

	ip = net.ParseIP(result.IP)
	if ip == nil {
		t.Errorf("Invalid return ip")
		return
	}

	if result.Mac.HardwareAddr == nil {
		t.Errorf("Empty mac")
		return
	}

	req, err = http.NewRequest("DELETE", "http://localhost:8080/"+hostname+"/"+result.Mac.String()+"/"+ip.String(), nil)
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	req.Header.Set("Accept", ctype)

	resp3, err := client.Do(req)
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	defer resp3.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Wrong http status: %v", resp.Status)
		return
	}

}
*/

func TestService(t *testing.T) {
	t.Run("TestWorkflowYAML", DHCPWorkflow(yamlMime))
	t.Run("TestWorkflowJSON", DHCPWorkflow(jsonMime))
	t.Run("TestWorkflowXML", DHCPWorkflow(xmlMime))
}

func DHCPWorkflow(mime MimeType) func(t *testing.T) {
	return func(t *testing.T) {

		hostname := "test"

		mime := yamlMime
		resp := request(t, "GET", "http://localhost:8080/"+hostname, mime)
		result := checkResult(t, resp, hostname, mime)

		resp2 := request(t, "GET", "http://localhost:8080/"+hostname+"/"+result.Mac.String(), mime)
		result2 := checkResult(t, resp2, hostname, mime)

		if result.IP != result2.IP {
			t.Fatalf("Different IP's %v != %v", result.IP, result2.IP)
		}

		resp = request(t, "GET", "http://localhost:8080/"+hostname+"/"+result.Mac.String()+"/"+result.IP, mime)
		result = checkResult(t, resp, hostname, mime)

		resp = request(t, "DELETE", "http://localhost:8080/"+hostname+"/"+result.Mac.String()+"/"+result.IP, mime)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Wrong http status: %v", resp.Status)
		}
	}
}

func request(t *testing.T, method string, url string, mime MimeType) *http.Response {
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

func checkResult(t *testing.T, resp *http.Response, hostname string, mime MimeType) *service.Result {
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Wrong http status: %v", resp.Status)
	}

	value := resp.Header.Get("Content-Type")
	if MimeType(value) != mime {
		t.Fatalf("Wrong content type: %v != %v", value, mime)
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("%v", err)
	}

	result := unmarshal(t, data, mime)

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
	return result
}

func unmarshal(t *testing.T, data []byte, mime MimeType) *service.Result {
	var result service.Result
	switch mime {
	case yamlMime:
		err := yaml.Unmarshal(data, &result)
		if err != nil {
			t.Fatalf("%v", err)
			return nil
		}
	case jsonMime:
		err := json.Unmarshal(data, &result)
		if err != nil {
			t.Fatalf("%v", err)
			return nil
		}
	case xmlMime:
		err := xml.Unmarshal(data, &result)
		if err != nil {
			t.Fatalf("%v", err)
			return nil
		}
	}

	return &result
}
