#!/usr/bin/env bash

docker run --rm -it -v "$GOPATH":/go -e "GOARCH=amd64" -w /go/src/github.com/daptin/daptin golang-crosscompile  ./scripts/build_docker.sh
docker run --rm -it -v "$GOPATH":/go -e GOARCH="386" -e CC="gcc -m32" -w /go/src/github.com/daptin/daptin golang-crosscompile  ./scripts/build_docker.sh
#docker run --rm -it -v "$GOPATH":/go -e GOOS=windows -e GOARCH=amd64 -w /go/src/github.com/daptin/daptin golang-crosscompile  ./scripts/build_docker.sh
#docker run --rm -it -v "$GOPATH":/go -e GOOS=linux -e GOARCH=amd64 -w /go/src/github.com/daptin/daptin golang-crosscompile  ./scripts/build_docker.sh
#docker run --rm -it -v "$GOPATH":/go -e GOOS=linux -e GOARCH=arm -e GOARM=6 -w /go/src/github.com/daptin/daptin golang-crosscompile  ./scripts/build_docker.sh
#docker run --rm -it -v "$GOPATH":/go -e GOOS=darwin -e GOARCH=amd64 -w /go/src/github.com/daptin/daptin golang-crosscompile  ./scripts/build_docker.sh