#!/bin/sh

docker build -t rest2dhcp:latest -f Dockerfile .
./scripts/extract.sh