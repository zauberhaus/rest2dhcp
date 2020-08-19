#!/bin/sh

DIFF=`git diff --stat`
TAGS=`git describe --tags 2> /dev/null`
NOW=`date +%FT%T.%3N%:z`
COMMIT=`git rev-parse --short HEAD`

UPX=`which upx`

if test ! -z "$DIFF"; then
  STATE='dirty'
else
  STATE='clean'
fi

go generate ./...
CGO_ENABLED=0 go build -ldflags "-X main.gitCommit=$COMMIT -X main.buildTime=$NOW -X main.treeState=$STATE -X main.tag=$TAGS $FLAGS"
git describe --tags