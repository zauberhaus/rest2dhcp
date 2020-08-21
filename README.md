# rest2dhcp

![Tests](https://github.com/zauberhaus/rest2dhcp/workflows/Tests/badge.svg)
![Release](https://github.com/zauberhaus/rest2dhcp/workflows/Release/badge.svg)

#### A REST web service gateway to a DHCP server
The service acts as a REST web service for clients and as a DHCP relay to the DHCP server.  
Therefore, it's possible to request IPs for more than one hostname or MAC address.
```
Usage:
  rest2dhcp [flags]
  rest2dhcp [command]

Available Commands:
  help        Help about any command
  version     Show the version info

Flags:
  -c, --client ip               Local IP for DHCP relay client
      --config string           Config file (default is $HOME/rest2dhcp.yaml)
  -d, --dhcp-timeout duration   DHCP query timeout (default 5s)
  -h, --help                    help for rest2dhcp
  -l, --listen string           Address of the web service (default ":8080")
  -m, --mode string             DHCP connection mode: auto, udp, packet, fritzbox, broken (default "auto")
  -q, --quiet                   Only access log messages
  -r, --relay ip                Relay IP for DHCP relay client
  -x, --retry duration          DHCP retry time (default 15s)
  -s, --server ip               DHCP server ip
  -t, --timeout duration        Service query timeout (default 30s)
  -v, --verbose                 Verbose messages
```

## Flags and Environment Variables

| Parameter    | Env Variable | Description                      | Default               |
|--------------|--------------|----------------------------------|-----------------------|
| client       | CLIENT       | IP of the local DHCP listener    | IP to default gateway |
| server       | SERVER       | IP of the remote DHCP server     | Default gateway       |
| relay        | RELAY        | Published DHCP relay IP          | Client IP             |
| listen       | LISTEN       | IP:Port of the web listener      | :8080                 |
| mode         | MODE         | Connection mode                  | auto                  |
| timeout      | TIMEOUT      | Web service timeout              | 30s                   |
| dhcp-timeout | DHCP_TIMEOUT | DHCP response timeout            | 5s                    |
| retry        | RETRY        | Wait time before retry           | 15s                   |


## Connection Mode

Unfortunately, the DHCP relay implementations are often very buggy or have some strange DoS protections.
The gateway has four different implementations for the DHCP connection and an auto-detection.

| Mode      | Description                               | Test system  |
|-----------|-------------------------------------------|---|
| auto      | A very simple auto-detection (only for development and testing) ||
| udp       | A UDP connection using port 67 for incoming and outgoing traffic |openwrt-19.07<br>ISC DHCP|
| dual      | Like the UDP connection, but with a UDP packet connection for outgoing traffic |openwrt-19.07<br>ISC DHCP|
| fritzbox  | A UDP packet connection sending DHCP packages with increasing src ports to port 67 and a UDP listener on port 67 | Fritz!Box 7590 |   
| packet    | A packet listener using port 67 for incoming and port 67 for outgoing traffic |openwrt-19.07<br>ISC DHCP|
| broken    | A packet listener using port 68 for incoming and port 67 for outgoing traffic |Android 10 WiFi hotspot |

Openwrt needs a very long time to respond on an IP request for an unknown host. 
It seems to be part of the DoS protection.
Therefore, the timeout must be selected large enough.

A Fritzbox will not respond if the time between two DHCP relay requests with the same source port is less than 15 seconds. 
Therefore, the fritzbox connector increases the source port after each request.

The Android WiFi hotspot Android incorrectly sends responses to DHCP relay requests to port 68.

## Docker usage

You can also use rest2dhcp as a docker-container.

```bash
SERVER=`IP of the DHCP server`
RELAY=`External IP of Docker host`

docker run -e SERVER=$SERVER -e RELAY=$RELAY -e MODE=udp -p 8080:8080 -p 67:67/udp  zauberhaus/rest2dhcp
```

You can use curl to test the server
```bash
curl http://localhost:8080/version
```
and should get a result like this

```yaml
buildDate: 2020-08-20T23:08:04UTC
compiler: gc
gitCommit: 8af5609
gitTreeState: dirty
gitVersion: v0.9.22
goVersion: go1.14.7
platform: linux/amd64
dhcp: 192.168.1.1
relay: 192.168.1.4
mode: packet
```

## API

The service provides an online documentation under the following url:

http://localhost:8080/api

## Client

```go
package main

import (
	"context"
	"fmt"

	"github.com/zauberhaus/rest2dhcp/client"
)

func main() {
	cl := client.NewClient("http://localhost:8080")

	version, err := cl.Version(context.Background())
	if err != nil {
		panic(err)
	}

	fmt.Println(version)

	lease, err := cl.Lease(context.Background(), "test", nil)
	if err != nil {
		panic(err)
	}

	fmt.Println(lease)

	lease, err = cl.Renew(context.Background(), lease.Hostname, lease.Mac, lease.IP)
	if err != nil {
		panic(err)
	}

	fmt.Println(lease)

	err = cl.Release(context.Background(), lease.Hostname, lease.Mac, lease.IP)
	if err != nil {
		panic(err)
	}
}
```

## Build

Requirements:
* Linux
* Go 1.14 

```
./build.sh
```

## Docker build

Requirements:
* Docker

```
sudo ./scripts/build-docker.sh
```

The script creates a folder ./build with the executable
