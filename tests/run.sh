#!/bin/sh

if [ "$1" = "--build" ] ; then
    docker-compose build
fi

if [ ! -f "$GOPATH/bin/ginkgo" ] ; then
    echo "Install ginkgo"
    go install github.com/onsi/ginkgo/ginkgo@latest
fi    

docker-compose up -d

PORTS="8080,8081,8082" "$GOPATH/bin/ginkgo" --tags=integration

docker-compose down