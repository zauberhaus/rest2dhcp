#!/bin/sh

ROUTE=`ip route get 1.1.2.1 | head -n1`
REMOTE=`echo $ROUTE |  awk '{ print $3}'`
RELAY=`echo $ROUTE |  awk '{ print $7}'`

IMAGE="rest2dhcp.debug:latest"

cd ..

echo "Build docker image"
docker build -t $IMAGE -f Dockerfile.debug .

docker run --name rest2dhcp -it --rm -p 2345:2345 --env "REMOTE=$REMOTE" --env "RELAY=$RELAY" -p 8080:8080 -p 67:67/udp -v $(pwd):/src rest2dhcp.debug:latest go test ./cmd ./client ./dhcp ./service