#!/bin/sh

cd ..

export VERSION=$(git describe --tags)

docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 --push -t dil001/lms-control .
docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 --push -t dil001/lms-control:v$VERSION .

