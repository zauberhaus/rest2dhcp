#!/bin/sh

ROUTE=`ip route get 1.1.2.1 | head -n1`
SERVER=`echo $ROUTE |  awk '{ print $3}'`
RELAY=`echo $ROUTE |  awk '{ print $7}'`

IMAGE="rest2dhcp:latest"

if test -z "$(docker images -q $IMAGE 2> /dev/null)"; then
    echo "Build docker image"
    docker build -t $IMAGE -f Dockerfile .
fi

docker run -it --rm -p 8080:8080 -p 67:67/udp -e SERVER=$SERVER -e RELAY=$RELAY $IMAGE
