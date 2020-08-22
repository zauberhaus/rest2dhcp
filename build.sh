#!/bin/sh

if ! which esc1 > /dev/null ; then 
  P=`pwd`
  cd ..
  go get github.com/mjibson/esc
  go get github.com/mdomke/git-semver
  cd "$P"
fi

DIFF=`git diff --stat`
VERSION=`git semver -prefix v`
NOW=`date +%FT%T.%3N%:z`
COMMIT=`git rev-parse --short HEAD`

UPX=`which upx`

if test ! -z "$DIFF"; then
  STATE='dirty'
else
  STATE='clean'
fi


go generate ./...
CGO_ENABLED=0 go build -ldflags "-X main.gitCommit=$COMMIT -X main.buildTime=$NOW -X main.treeState=$STATE -X main.tag=$VERSION $FLAGS"
git describe --tags
