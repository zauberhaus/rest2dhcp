# rest2dhcp

A REST webservice gateway to a DHCP server

Usage:
  rest2dhcp [flags]
  rest2dhcp [command]

Available Commands:
  help        Help about any command
  version     Show the version info

Flags:
  -c, --client ip               Local IP for DHCP relay client (autodetect)
      --config string           config file (default is $HOME/.rest2dhcp.yaml)
  -d, --dhcp-timeout duration   DHCP query timeout (default 5s)
  -h, --help                    help for rest2dhcp
  -l, --listen string           Address of the web service (default ":8080")
  -m, --mode string             DHCP connection mode: auto|udp|packet|fritzbox|broken (default "auto")
  -r, --relay ip                Relay IP for DHCP relay client (client IP)
  -x, --retry duration          DHCP retry time (default 15s)
  -s, --server ip               DHCP server ip (autodetect)
  -t, --timeout duration        Service query timeout (default 30s)

