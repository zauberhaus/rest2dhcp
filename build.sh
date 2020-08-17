#!/bin/sh

DIFF=`git diff --stat`
TAGS=`git describe --tags 2> /dev/null`
NOW=`date +%FT%T.%3N%:z`
COMMIT=`git rev-parse HEAD`

UPX=`which upx`

if test ! -z "$DIFF"; then
  STATE='dirty'
else
  STATE='clean'
fi

if test -z "$OUT"; then
  OUT="./rest2dhcp"
fi

cd `go env GOPATH`/src/github.com/zauberhaus/rest2dhcp

GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-X main.gitCommit=$COMMIT -X main.buildTime=$NOW -X main.treeState=$STATE -X main.tag=$TAGS $FLAGS" -o $OUT-amd64
which upx && `which upx` $OUT-amd64

GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "-X main.gitCommit=$COMMIT -X main.buildTime=$NOW -X main.treeState=$STATE -X main.tag=$TAGS" -o $OUT-arm64
which upx && `which upx` $OUT-arm64

GOOS=linux GOARCH=arm CGO_ENABLED=0 go build -ldflags "-X main.gitCommit=$COMMIT -X main.buildTime=$NOW -X main.treeState=$STATE -X main.tag=$TAGS" -o $OUT-arm
which upx && `which upx` $OUT-arm

GOOS=linux GOARCH=386 CGO_ENABLED=0 go build -ldflags "-X main.gitCommit=$COMMIT -X main.buildTime=$NOW -X main.treeState=$STATE -X main.tag=$TAGS" -o $OUT-386
which upx && `which upx` $OUT-386
