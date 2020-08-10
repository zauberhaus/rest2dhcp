#!/bin/bash

if [[ $(git diff --stat) != '' ]]; then
  state='dirty'
else
  state='clean'
fi

tags=$(git describe --tags 2> /dev/null)

# notice how we avoid spaces in $now to avoid quotation hell in go build command
now=$(date +%Y-%m-%dT%H:%M:%S%Z)
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-X main.gitCommit=`git rev-parse HEAD` -X main.buildTime=$now -X main.treeState=$state  -X main.tag=$tags"
