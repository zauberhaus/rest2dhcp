# rest2dhcp


A REST webservice gateway to a DHCP server
The service acts as an REST webservice for clients and as an DHCP relay to the DHCP server.
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
  -r, --relay ip                Relay IP for DHCP relay client
  -x, --retry duration          DHCP retry time (default 15s)
  -s, --server ip               DHCP server ip
  -t, --timeout duration        Service query timeout (default 30s)
```

## Parameter

| Parameter      | Description                          | Default         |
|--------------|--------------------------------|-----------------|
| client       | IP of the local DHCP listener  | IP to default gateway       |
| server       | IP of the remote DHCP server   | Default gateway |
| relay        | Published DHCP relay IP        | Client IP       |
| listen       | IP:Port of the web listener    | 0.0.0.0:8080    |
| mode         | Connection mode                | auto            |
| timeout      | Web service timeout            | 30s             |
| dhcp-timeout | DHCP response timeout          | 5s             |
| retry        | Wait time before retry         | 15s             |


## Connection Mode

Unfortunately, the DHCP relay implementations are often very buggy or have some strange DoS protections.
The gateway has 4 different implementations for the DHCP connection and a auto detection.

| Mode      | Description                               |
|-----------|-------------------------------------------|
| auto      | A very simple auto detection (only for development and testing) |
| udp       | A udp connection using port 67 for incoming and outgoing traffic (Openwrt) |
| packet    | Like the udp connection, but with a udo packet connection for outgoing traffic |
| fritzbox  | A udp packet connection sending DHCP packages with increasing src ports to port 67 and a udp listener on port 67 (Fritz!Box 7590) |   
| broken    | A packer listener using port 68 for incoming and port 67 for outgoing traffic (Android WiFi hotspot) |

Openwrt needs a very long time to respond on an IP request for a unknown host. It seems to be part of the DoS protection.

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
./build-docker.sh
```
or
```
./run-docker.sh
```
