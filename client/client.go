package client

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"

	"gopkg.in/yaml.v3"
)

type Client struct {
	url         string
	ContentType ContentType
}

func NewClient(url string) *Client {
	return &Client{
		url:         url,
		ContentType: JSON,
	}
}

func (c *Client) Lease(hostname string, mac *MAC) (*Lease, error) {
	if hostname == "" {
		return nil, NewError(http.StatusBadRequest, "Empty hostname")
	}

	url := fmt.Sprintf("%s/ip/%s", c.url, hostname)

	if mac != nil {
		url += "/" + mac.String()
	}

	resp, err := c.request("GET", url, c.ContentType)
	if err != nil {
		return nil, err
	}

	data, contentType, err := c.read(resp)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, NewError(resp.StatusCode, string(data))
	}

	var result Result
	err = c.unmarshal(data, &result, contentType)
	if err != nil {
		return nil, err
	}

	return result.Lease, nil
}

func (c *Client) Renew(hostname string, mac *MAC, ip net.IP) (*Lease, error) {
	if hostname == "" {
		return nil, NewError(http.StatusBadRequest, "Empty hostname")
	}

	if mac == nil {
		return nil, NewError(http.StatusBadRequest, "Missing mac address")
	}

	if ip == nil {
		return nil, NewError(http.StatusBadRequest, "Missing ip address")
	}

	url := fmt.Sprintf("%s/ip/%s/%v/%v", c.url, hostname, mac, ip)

	resp, err := c.request("GET", url, c.ContentType)
	if err != nil {
		return nil, err
	}

	data, contentType, err := c.read(resp)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, NewError(resp.StatusCode, string(data))
	}

	var result Result
	err = c.unmarshal(data, &result, contentType)
	if err != nil {
		return nil, err
	}

	return result.Lease, nil
}

func (c *Client) Release(hostname string, mac *MAC, ip net.IP) error {
	if hostname == "" {
		return NewError(http.StatusBadRequest, "Empty hostname")
	}

	if mac == nil {
		return NewError(http.StatusBadRequest, "Missing mac address")
	}

	if ip == nil {
		return NewError(http.StatusBadRequest, "Missing ip address")
	}

	url := fmt.Sprintf("%s/ip/%s/%v/%v", c.url, hostname, mac, ip)

	resp, err := c.request("DELETE", url, c.ContentType)
	if err != nil {
		return err
	}

	data, _, err := c.read(resp)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return NewError(resp.StatusCode, string(data))
	}

	return nil
}

func (c *Client) Version() (*Version, error) {
	url := fmt.Sprintf("%s/version", c.url)

	resp, err := c.request("GET", url, c.ContentType)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(resp.Status)
	}

	data, contentType, err := c.read(resp)
	if err != nil {
		return nil, err
	}

	var info VersionInfo
	err = c.unmarshal(data, &info, contentType)
	if err != nil {
		return nil, err
	}

	return info.ServiceVersion, nil
}

func (c *Client) request(method string, url string, mime ContentType) (*http.Response, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", string(mime))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (c *Client) read(resp *http.Response) ([]byte, ContentType, error) {
	var contentType ContentType
	value := resp.Header.Get("Content-Type")
	contentType.Parse(value)

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, Unknown, err
	}

	return data, contentType, nil
}

func (c *Client) unmarshal(data []byte, result interface{}, mime ContentType) error {
	switch mime {
	case YAML:
		err := yaml.Unmarshal(data, result)
		if err != nil {
			return err
		}
	case JSON:
		err := json.Unmarshal(data, result)
		if err != nil {
			return err
		}
	case XML:
		err := xml.Unmarshal(data, result)
		if err != nil {
			return err
		}
	}

	return nil
}
