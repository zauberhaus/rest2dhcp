#!/bin/sh

export CGO_ENABLED=0

sudo go test -v -coverpkg=./... -coverprofile=coverage.out ./...
go tool cover -func coverage.out