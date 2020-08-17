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

package client_test

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/zauberhaus/rest2dhcp/client"
	"github.com/zauberhaus/rest2dhcp/dhcp"
	"github.com/zauberhaus/rest2dhcp/service"
	helper_test "github.com/zauberhaus/rest2dhcp/test"
)

var (
	host   = "http://localhost:8080"
	server = helper_test.TestServer{}
)

func TestMain(m *testing.M) {
	server.Run(m)
}

func TestClientVersion(t *testing.T) {
	testCases := []struct {
		//Name string
		Mime client.ContentType
	}{
		{
			//Name: "XML",
			Mime: client.XML,
		},
		{
			//Name: "YAML",
			Mime: client.YAML,
		},
		{
			//Name: "JSON",
			Mime: client.JSON,
		},
		{
			//Name: "Read version via HTTP request without a content type",
			Mime: client.Unknown,
		},
	}

	for _, tc := range testCases {
		tc := tc // We run our tests twice one with this line & one without
		t.Run(tc.Mime.String(), func(t *testing.T) {
			t.Parallel()

			cl := client.NewClient(host)
			cl.ContentType = tc.Mime
			ctx := context.Background()

			version, err := cl.Version(ctx)
			if cl.ContentType == client.Unknown {
				clientError, ok := err.(*client.Error)
				if !ok {
					t.Fatalf("Unexpected error type")
				}

				assert.Equal(t, 415, clientError.Code(), "Unexpected status code")

			} else if assert.NoError(t, err, "client.Version failed") {
				if assert.NotNil(t, version, "Empty version info") {
					if server.IsStarted() {
						assert.Equal(t, service.Version, version, "Invalid Version info")
					} else {
						assert.NotEmpty(t, version.GitCommit)
					}
				}
			}
		})
	}
}

func TestClient(t *testing.T) {
	testCases := []struct {
		Name     string
		Mime     client.ContentType
		Hostname string
		Mac      client.MAC
	}{
		{
			Name:     "Run DHCP workflow via HTTP request with content type XML",
			Mime:     client.XML,
			Hostname: "test-xml",
			Mac:      client.MAC{1, 2, 3, 4, 5, 6},
		},
		{
			Name:     "Run DHCP workflow via HTTP request with content type YAML",
			Mime:     client.YAML,
			Hostname: "test-yaml",
			Mac:      client.MAC{1, 2, 3, 4, 5, 7},
		},
		{
			Name:     "Run DHCP workflow via HTTP request with content type JSON",
			Mime:     client.JSON,
			Hostname: "test-json",
			Mac:      client.MAC{1, 2, 3, 4, 5, 8},
		},
		{
			Name:     "Run DHCP workflow via HTTP request without a content type",
			Mime:     client.Unknown,
			Hostname: "test-unknown",
			Mac:      client.MAC{1, 2, 3, 4, 5, 9},
		},
	}

	for _, tc := range testCases {
		tc := tc // We run our tests twice one with this line & one without
		t.Run(tc.Name, func(t *testing.T) {
			//t.Parallel()

			cl := client.NewClient(host)
			cl.ContentType = tc.Mime

			ctx := context.Background()

			lease, err := cl.Lease(ctx, tc.Hostname+"-1", nil)

			if tc.Mime == client.Unknown {
				clientError, ok := err.(*client.Error)
				if !ok {
					assert.Fail(t, "Unexpected error type")
				}

				assert.Equal(t, 415, clientError.Code(), "Unexpected status code")

				return
			}

			if assert.NoError(t, err) {

				if assert.NotNil(t, lease) && assert.NotNil(t, lease.IP) && assert.NotNil(t, lease.Mac) {

					lease2, err := cl.Lease(ctx, lease.Hostname, lease.Mac)

					if assert.NoError(t, err) {

						assert.Equal(t, lease2.Mac, lease.Mac)
						assert.Equal(t, lease2.IP, lease.IP)

						if server.GetMode() == dhcp.Fritzbox {
							checkDNSExists(t, lease)
						}

						lease3, err := cl.Lease(ctx, tc.Hostname+"-2", tc.Mac)

						if assert.NoError(t, err) {

							assert.Equal(t, tc.Mac, lease3.Mac)
							assert.NotEqual(t, lease.IP, lease3.IP)

							err1 := cl.Release(ctx, lease.Hostname, lease.Mac, lease.IP)
							err2 := cl.Release(ctx, lease3.Hostname, lease3.Mac, lease3.IP)

							if assert.NoError(t, err1) && assert.NoError(t, err2) {
								if server.GetMode() == dhcp.Fritzbox {
									checkDNSNotExists(t, lease)
									checkDNSNotExists(t, lease3)
								}
							}
						}

						lease4, err := cl.Renew(ctx, lease.Hostname, lease.Mac, lease.IP)
						if assert.NoError(t, err) {
							if assert.NotNil(t, lease4) {

								assert.Equal(t, lease.Hostname, lease4.Hostname)
								assert.Equal(t, lease.Mac, lease4.Mac)
								assert.Equal(t, lease.IP, lease4.IP)

								if server.GetMode() == dhcp.Fritzbox {
									checkDNSExists(t, lease4)
								}

								err = cl.Release(ctx, lease4.Hostname, lease4.Mac, lease4.IP)
								assert.NoError(t, err)
							}
						}
					}
				}
			}
			fmt.Printf("DONE %v\n", tc.Name)
		})
	}
}

func TestClientInvalidLease(t *testing.T) {
	testCases := []struct {
		Name     string
		Hostname string
		Mac      client.MAC
		Code     int
	}{
		{
			Name:     "Invalid hostname",
			Hostname: "test_123",
			Mac:      nil,
			Code:     400,
		},
		{
			Name:     "Invalid mac",
			Hostname: "test",
			Mac:      client.MAC{1, 2, 3, 4, 5, 6, 7},
			Code:     400,
		},
		{
			Name:     "Empty hostname",
			Hostname: "",
			Mac:      nil,
			Code:     400,
		},
	}

	for _, tc := range testCases {
		tc := tc // We run our tests twice one with this line & one without
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			cl := client.NewClient(host)
			_, err := cl.Lease(ctx, tc.Hostname, tc.Mac)

			if assert.Error(t, err) {
				clientError, ok := err.(*client.Error)
				if assert.True(t, ok) {
					assert.Equal(t, tc.Code, clientError.Code())
				}
			}
		})
	}
}

func TestClientInvalidRenew(t *testing.T) {
	testCases := []struct {
		Name     string
		Hostname string
		Mac      client.MAC
		IP       net.IP
		Code     int
	}{
		{
			Name:     "Invalid hostname",
			Hostname: "test_123",
			Mac:      client.MAC{1, 2, 3, 4, 5, 6},
			IP:       net.IP{1, 2, 3, 4},
			Code:     400,
		},
		{
			Name:     "Empty hostname",
			Hostname: "",
			Mac:      client.MAC{1, 2, 3, 4, 5, 6},
			IP:       net.IP{1, 2, 3, 4},
			Code:     400,
		},
		{
			Name:     "Empty mac",
			Hostname: "test",
			Mac:      nil,
			IP:       net.IP{1, 2, 3, 4},
			Code:     400,
		},
		{
			Name:     "Empty ip",
			Hostname: "test",
			Mac:      client.MAC{1, 2, 3, 4, 5, 6},
			IP:       nil,
			Code:     400,
		},
		{
			Name:     "Invalid lease",
			Hostname: "test",
			Mac:      client.MAC{1, 2, 3, 4, 5, 6},
			IP:       net.IP{1, 2, 3, 4},
			Code:     417,
		},
	}

	for _, tc := range testCases {
		tc := tc // We run our tests twice one with this line & one without
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			cl := client.NewClient(host)
			_, err := cl.Renew(ctx, tc.Hostname, tc.Mac, tc.IP)

			if assert.Error(t, err) {
				clientError, ok := err.(*client.Error)
				if assert.True(t, ok) {
					assert.Equal(t, tc.Code, clientError.Code())
				}
			}
		})
	}
}

func checkDNSExists(t *testing.T, l *client.Lease) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	rc := false
	empty := true

	for {
		ips, err := net.LookupIP(l.Hostname)
		if err != nil {
			select {
			case <-ctx.Done():
				assert.Fail(t, "DNS entry %v not found", l.Hostname)
				break
			case <-time.After(100 * time.Millisecond):
				empty = false
				fmt.Print(".")
				continue
			}
		}

		for _, ip := range ips {
			if ip.String() == l.IP.String() {
				rc = true
				break
			}
		}

		if !rc {
			assert.Fail(t, "Wrong IPs for DNS entry %v not found", l.Hostname)
		}

		break
	}

	if !empty {
		fmt.Println("")
	}

	return rc
}

func checkDNSNotExists(t *testing.T, l *client.Lease) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	for {
		_, err := net.LookupIP(l.Hostname)
		if err == nil {
			select {
			case <-ctx.Done():
				fmt.Print("\n")
				return assert.Fail(t, "DNS entry %v found", l.Hostname)
			case <-time.After(100 * time.Millisecond):
				fmt.Print(".")
				continue
			}
		}

		fmt.Print("\n")
		return true
	}
}
