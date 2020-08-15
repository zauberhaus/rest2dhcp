#!/bin/sh

ROUTE=`ip route get 1.1.2.1 | head -n1`
REMOTE=`echo $ROUTE |  awk '{ print $3}'`
RELAY=`echo $ROUTE |  awk '{ print $7}'`

IMAGE="rest2dhcp.debug:latest"

if test -z "$(docker images -q $IMAGE 2> /dev/null)"; then
    echo "Build docker image"
    docker build -t $IMAGE -f Dockerfile.debug .
fi

docker run -it --rm -p 2345:2345 -p 8080:8080 -p 67:67/udp --env "REMOTE=$REMOTE" --env "RELAY=$RELAY" -v $(pwd):/src rest2dhcp.debug:latest