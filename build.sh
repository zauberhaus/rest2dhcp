#!/bin/sh

if [ ! -f "$GOPATH/bin/esc" ] ; then
    echo "Install esc"
    go install github.com/mjibson/esc@v0.2.0
fi

DIFF=`git diff --stat`
VERSION=`git describe --tags --always --dirty`
NOW=`date`
COMMIT=`git rev-parse --short HEAD`

if test ! -z "$DIFF"; then
  STATE='dirty'
else
  STATE='clean'
fi

mkdir -p ./build

if [ ! -f "./service/doc.go" ] ; then
    echo "Generate embedded files"
    go generate ./service/server.go
fi

FLAGS="-X 'main.gitCommit=$COMMIT' -X 'main.buildTime=$NOW' -X 'main.treeState=$STATE' -X 'main.tag=$VERSION' -w -s"

CGO_ENABLED=0 go build -ldflags "$FLAGS"
echo "Version: $VERSION"
