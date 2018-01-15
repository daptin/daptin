#!/usr/bin/env bash

docker run --rm -it -v "$GOPATH":/go -e GOOS=windows -e GOARCH=386 -w /go/src/github.com/daptin/daptin golang-crosscompile  ./scripts/build_docker.sh
docker run --rm -it -v "$GOPATH":/go -e GOOS=linux -e GOARCH=amd64 -w /go/src/github.com/daptin/daptin golang-crosscompile  ./scripts/build_docker.sh
docker run --rm -it -v "$GOPATH":/go -e GOOS=linux -e GOARCH=arm -e GOARM=6 -w /go/src/github.com/daptin/daptin golang-crosscompile  ./scripts/build_docker.sh