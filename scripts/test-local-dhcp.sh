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

echo "build test container"
docker build -t rest2dhcp:test -f Dockerfile.wf .

docker run --rm -e SERVER=172.17.0.3 -e MODE=udp rest2dhcp:test
docker run --rm -e SERVER=172.17.0.3 -e MODE=dual rest2dhcp:test
docker run --rm -e SERVER=172.17.0.3 -e MODE=packet rest2dhcp:test

if test ! -z "$DHCP_ID" ; then 
    docker stop $DHCP_ID
fi    