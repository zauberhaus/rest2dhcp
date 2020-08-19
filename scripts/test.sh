#!/bin/sh

export CGO_ENABLED=0

go test -coverpkg=./... -coverprofile=coverage.out ./cmd ./client ./dhcp ./service $@ && \
go tool cover -func coverage.out
