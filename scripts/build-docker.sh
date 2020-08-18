#!/bin/sh

cd ..

IMAGE="rest2dhcp:latest"
docker build -t $IMAGE -f Dockerfile .
