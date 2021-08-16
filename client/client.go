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

package client

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"

	"gopkg.in/yaml.v3"
)

const (
	emptyHostname = "Empty hostname"
	missingIP     = "Missing ip address"
	missingMac    = "Missing mac address"
	invalidIP     = "Not a valid IP4 address"
)

type Client interface {
	SetContentType(t ContentType)
	GetContentType() ContentType

	Lease(ctx context.Context, hostname string, mac MAC) (*Lease, error)
	Renew(ctx context.Context, hostname string, mac MAC, ip net.IP) (*Lease, error)
	Release(ctx context.Context, hostname string, mac MAC, ip net.IP) error

	Version(ctx context.Context) (*Version, error)
}

// implClient is a go client to use the service
type implClient struct {
	url         string
	contentType ContentType
}

// NewClient initializes a new client
// * @param url - url of the service
func NewClient(url string) Client {
	return &implClient{
		url:         url,
		contentType: JSON,
	}
}

func (c *implClient) SetContentType(t ContentType) {
	c.contentType = t
}

func (c *implClient) GetContentType() ContentType {
	return c.contentType
}

/*
Lease - request a IP lease for a given hostname and a generated mac address.
 * @param ctx context.Context - for authentication, logging, cancellation, deadlines, tracing, etc. Passed from http.Request or context.Background().
 * @param hostname - Hostname
 * @param mac - Mac address (optional - if nil a mac address is generated from the hostname)

@return Lease
*/
func (c *implClient) Lease(ctx context.Context, hostname string, mac MAC) (*Lease, error) {
	if hostname == "" {
		return nil, NewError(http.StatusBadRequest, emptyHostname)
	}

	url := fmt.Sprintf("%s/ip/%s", c.url, hostname)

	if mac != nil {
		url += "/" + mac.String()
	}

	resp, err := c.request(ctx, "GET", url, c.contentType)
	if err != nil {
		return nil, err
	}

	data, contentType, err := c.read(resp)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		if data[len(data)-1] == '\n' {
			data = data[:len(data)-1]
		}

		return nil, NewError(resp.StatusCode, string(data))
	}

	var result Lease
	err = c.unmarshal(data, &result, contentType)
	if err != nil {
		return nil, NewError(-2, err.Error())
	}

	return &result, nil
}

/*
Renew an IP lease for a given hostname, mac and IP address.
 * @param ctx context.Context - for authentication, logging, cancellation, deadlines, tracing, etc. Passed from http.Request or context.Background().
 * @param hostname - Hostname
 * @param mac - Mac address
 * @param ip - IP address

@return Lease
*/
func (c *implClient) Renew(ctx context.Context, hostname string, mac MAC, ip net.IP) (*Lease, error) {
	if hostname == "" {
		return nil, NewError(http.StatusBadRequest, emptyHostname)
	}

	if mac == nil {
		return nil, NewError(http.StatusBadRequest, missingMac)
	}

	if ip == nil {
		return nil, NewError(http.StatusBadRequest, missingIP)
	}

	if ip.To4() == nil {
		return nil, NewError(http.StatusBadRequest, invalidIP)
	}

	url := fmt.Sprintf("%s/ip/%s/%v/%v", c.url, hostname, mac, ip.To4())

	resp, err := c.request(ctx, "GET", url, c.contentType)
	if err != nil {
		fmt.Println(err)

		return nil, err
	}

	data, contentType, err := c.read(resp)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		if data[len(data)-1] == '\n' {
			data = data[:len(data)-1]
		}

		return nil, NewError(resp.StatusCode, string(data))
	}

	var result Lease
	err = c.unmarshal(data, &result, contentType)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

/*
Release the IP for a given hostname, mac and IP address.
 * @param ctx context.Context - for authentication, logging, cancellation, deadlines, tracing, etc. Passed from http.Request or context.Background().
 * @param hostname - Hostname
 * @param mac - Mac address
 * @param ip - IP address
*/
func (c *implClient) Release(ctx context.Context, hostname string, mac MAC, ip net.IP) error {
	if hostname == "" {
		return NewError(http.StatusBadRequest, emptyHostname)
	}

	if mac == nil {
		return NewError(http.StatusBadRequest, missingMac)
	}

	if ip == nil {
		return NewError(http.StatusBadRequest, missingIP)
	}

	url := fmt.Sprintf("%s/ip/%s/%v/%v", c.url, hostname, mac, ip)

	resp, err := c.request(ctx, "DELETE", url, c.contentType)
	if err != nil {
		return err
	}

	data, _, err := c.read(resp)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		if data[len(data)-1] == '\n' {
			data = data[:len(data)-1]
		}

		return NewError(resp.StatusCode, string(data))
	}

	return nil
}

/*
Version returns the service version information
 * @param ctx context.Context - for authentication, logging, cancellation, deadlines, tracing, etc. Passed from http.Request or context.Background().

@return Version
*/
func (c *implClient) Version(ctx context.Context) (*Version, error) {
	url := fmt.Sprintf("%s/version", c.url)

	resp, err := c.request(ctx, "GET", url, c.contentType)
	if err != nil {
		return nil, err
	}

	data, contentType, err := c.read(resp)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		if data[len(data)-1] == '\n' {
			data = data[:len(data)-1]
		}

		return nil, NewError(resp.StatusCode, string(data))
	}

	var info Version
	err = c.unmarshal(data, &info, contentType)
	if err != nil {
		return nil, err
	}

	return &info, nil
}

func (c *implClient) request(ctx context.Context, method string, url string, mime ContentType) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", string(mime))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, NewError(-1, err.Error())
	}

	return resp, nil
}

func (c *implClient) read(resp *http.Response) ([]byte, ContentType, error) {
	var contentType ContentType
	value := resp.Header.Get("Content-Type")
	contentType.Parse(value)

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, Unknown, NewError(-1, err.Error())
	}

	return data, contentType, nil
}

func (c *implClient) unmarshal(data []byte, result interface{}, mime ContentType) error {
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
	default:
		return fmt.Errorf("unexpected content type")
	}

	return nil
}
