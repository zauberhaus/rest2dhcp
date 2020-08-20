#!/bin/sh

# exit on error
set -e

if test -z "`docker ps --filter "name=^/dhcp$" --format '{{.Names}}'`" ; then
    echo "build and start the DHCP server..."
    docker build -t dhcp -f Dockerfile.dhcp .
    DHCP_ID=`docker run -d --rm --name dhcp dhcp`
    echo "DHCP container started $DHCP_ID"
fi

DHCP_IP=`docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' dhcp`
echo "DHCP server runs on $DHCP_IP"

go clean -testcache
SERVER=$DHCP_IP MODE=udp go test ./cmd ./client ./dhcp ./service
go clean -testcache
SERVER=$DHCP_IP MODE=packet go test ./cmd ./client ./dhcp ./service
go clean -testcache
SERVER=$DHCP_IP MODE=dual go test ./cmd ./client ./dhcp ./service

if test ! -z "$DHCP_ID" ; then 
    docker stop $DHCP_ID
fi    