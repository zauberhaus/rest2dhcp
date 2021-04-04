#!/bin/sh

if [ "$1" = "--build" ] ; then
    docker-compose build
fi

docker-compose up -d

PORTS="8080,8081,8082" ginkgo

docker-compose down