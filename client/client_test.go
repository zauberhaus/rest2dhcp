package client_test

import (
	"net"
	"testing"
	"time"

	"github.com/zauberhaus/rest2dhcp/client"
	"github.com/zauberhaus/rest2dhcp/dhcp"
	"github.com/zauberhaus/rest2dhcp/test"
)

const (
	versionURL = "http://localhost:8080/version"

	buildDate    = "2020-08-11T10:06:44NZST"
	gitCommit    = "03fd9a8658c81c088fb548cc43b56703e6ee145b"
	gitVersion   = "v0.0.1"
	gitTreeState = "dirty"
)

var (
	host = "http://localhost:8080"

	leases []*client.Lease
	server = test.TestServer{}
)

func TestMain(m *testing.M) {
	server.Run(m)
}

func TestClientVersion(t *testing.T) {
	t.Parallel()
	t.Run("VersionYaml", getVersion(client.YAML))
	t.Run("VersionJSON", getVersion(client.YAML))
	t.Run("VersionXML", getVersion(client.YAML))
}

func TestClient(t *testing.T) {
	t.Run("LeaseYaml", getLease(client.YAML, "test-yaml", "01:02:03:04:05:06"))
	t.Run("LeaseJSON", getLease(client.JSON, "test-json", "01:02:03:04:05:07"))
	t.Run("LeaseXML", getLease(client.XML, "test-xml", "01:02:03:04:05:08"))

	l1 := leases[0]
	l2 := leases[2]
	l3 := leases[4]

	t.Run("RelaseAll", relaseAll())

	leases = []*client.Lease{l1, l2, l3}

	t.Run("RenewYaml", renew(client.YAML, 0))
	t.Run("RenewJSON", renew(client.YAML, 1))
	t.Run("RenewXML", renew(client.YAML, 2))

	t.Run("RelaseRest", relaseAll())
}

func TestLeaseBadHostname(t *testing.T) {
	cl := client.NewClient(host)
	_, err := cl.Lease("test_123", nil)

	clerr, ok := err.(*client.Error)
	if !ok {
		t.Fatalf("Unexpected error type")
	}

	if clerr.Code() != 400 {
		t.Fatalf("Unexpected statuc code: %v - %v", clerr.Code(), clerr.Msg())
	}
}

func TestLeaseMissingHostname(t *testing.T) {
	cl := client.NewClient(host)
	_, err := cl.Lease("", nil)

	clerr, ok := err.(*client.Error)
	if !ok {
		t.Fatalf("Unexpected error type")
	}

	if clerr.Code() != 400 {
		t.Fatalf("Unexpected statuc code: %v - %v", clerr.Code(), clerr.Msg())
	}
}

func TestRenewBadHostname(t *testing.T) {
	cl := client.NewClient(host)
	_, err := cl.Renew("test_123", &client.MAC{net.HardwareAddr{1, 2, 3, 4, 5, 6}}, net.IP{1, 2, 3, 4})

	clerr, ok := err.(*client.Error)
	if !ok {
		t.Fatalf("Unexpected error type")
	}

	if clerr.Code() != 400 {
		t.Fatalf("Unexpected statuc code: %v - %v", clerr.Code(), clerr.Msg())
	}
}

func TestRenewMissingHostname(t *testing.T) {
	cl := client.NewClient(host)
	_, err := cl.Renew("", nil, nil)

	clerr, ok := err.(*client.Error)
	if !ok {
		t.Fatalf("Unexpected error type")
	}

	if clerr.Code() != 400 {
		t.Fatalf("Unexpected statuc code: %v - %v", clerr.Code(), clerr.Msg())
	}
}

func TestRenewMissingMac(t *testing.T) {
	cl := client.NewClient(host)
	_, err := cl.Renew("test_123", nil, nil)

	clerr, ok := err.(*client.Error)
	if !ok {
		t.Fatalf("Unexpected error type")
	}

	if clerr.Code() != 400 {
		t.Fatalf("Unexpected statuc code: %v - %v", clerr.Code(), clerr.Msg())
	}
}

func getVersion(c client.ContentType) func(t *testing.T) {
	return func(t *testing.T) {
		cl := client.NewClient(host)
		cl.ContentType = c

		version, err := cl.Version()
		if err != nil {
			t.Fatalf("Version() failed: %v", err)
		}

		if version == nil {
			t.Errorf("Invalid Version info")
		}

		if server.IsStarted() {
			if version.BuildDate != buildDate {
				t.Errorf("Invalid buid date %v!=%v", version.BuildDate, buildDate)
			}

			if version.GitCommit != gitCommit {
				t.Errorf("Invalid git commit %v!=%v", version.GitCommit, gitCommit)
			}

			if version.GitVersion != gitVersion {
				t.Errorf("Invalid git version %v!=%v", version.GitVersion, gitVersion)
			}

			if version.GitTreeState != gitTreeState {
				t.Errorf("Invalid git tree state %v!=%v", version.GitTreeState, gitTreeState)
			}
		} else {
			if version.GitCommit == "" {
				t.Errorf("Invalid version info:\n%v", version)
			}
		}
	}
}

func getLease(c client.ContentType, hostname string, addr string) func(t *testing.T) {

	return func(t *testing.T) {

		cl := client.NewClient(host)
		cl.ContentType = c

		lease, err := cl.Lease(hostname+"-1", nil)

		if err != nil {
			t.Errorf("client.GetLease: %v", err)
		}

		if lease == nil {
			t.Errorf("Empty lease")
		}

		if lease.IP == nil {
			t.Errorf("Empty IP")
		}

		mac := lease.Mac

		lease2, err := cl.Lease(lease.Hostname, &lease.Mac)

		if err != nil {
			t.Errorf("client.GetLease: %v", err)
		}

		add(lease2)

		if lease2.Mac.String() != mac.String() {
			t.Fatalf("Different mac addresses %v != %v", mac, lease2.Mac)
		}

		if lease.IP.String() != lease2.IP.String() {
			t.Fatalf("Different IP addresses %v != %v", lease.IP, lease2.IP)
		}

		if server.GetMode() == dhcp.Fritzbox && checkDNS(lease) != 2 {
			t.Errorf("DNS entry %s not found.", lease.Hostname)
		}

		mac2, err := net.ParseMAC(addr)
		if err != nil {
			t.Fatalf("%v", err)
		}

		lease3, err := cl.Lease(hostname+"-2", &client.MAC{mac2})

		if err != nil {
			t.Errorf("client.GetLease: %v", err)
		}

		add(lease3)

		if lease3.Mac.String() != mac2.String() {
			t.Fatalf("Got wrong mac address %v != %v", mac2, lease3.Mac)
		}

		if lease.IP.String() == lease3.IP.String() {
			t.Fatalf("IP's shouldn't be equal %v == %v", lease.IP, lease3.IP)
		}
	}
}

func renew(c client.ContentType, pos int) func(t *testing.T) {
	return func(t *testing.T) {
		cl := client.NewClient(host)
		cl.ContentType = c
		l := leases[pos]

		lease, err := cl.Renew(l.Hostname, &l.Mac, l.IP)
		if err != nil {
			t.Fatalf("client.GetLease: %v", err)
		}

		if lease == nil {
			t.Fatalf("client.Renew: lease is empty")
		}

		if lease.Hostname != l.Hostname || lease.IP.String() != l.IP.String() || lease.Mac.String() != l.Mac.String() {
			t.Errorf("Renewed is different")
		}

		if server.GetMode() == dhcp.Fritzbox && checkDNS(lease) != 2 {
			t.Errorf("DNS entry %s not found.", lease.Hostname)
		}
	}
}

func relaseAll() func(t *testing.T) {
	return func(t *testing.T) {
		cl := client.NewClient(host)

		for _, l := range leases {
			//fmt.Printf("Relase %s/%v/%v\n", l.Hostname, l.Mac, l.IP)
			err := cl.Release(l.Hostname, &l.Mac, l.IP)

			if err != nil {
				t.Errorf("client.Release: %v", err)
			}
		}

		if server.GetMode() == dhcp.Fritzbox {
			time.Sleep(10 * time.Second)

			for _, l := range leases {
				if checkDNS(l) != 0 {
					t.Errorf("DNS entry %s is still there.", l.Hostname)
				}
			}
		}

		leases = []*client.Lease{}
	}
}

func checkDNS(l *client.Lease) int {
	ips, err := net.LookupIP(l.Hostname)
	if err != nil {
		return 0
	}

	for _, ip := range ips {
		if ip.String() == l.IP.String() {
			return 2
		}
	}

	return 1
}

func add(l *client.Lease) {
	leases = append(leases, l)
}
