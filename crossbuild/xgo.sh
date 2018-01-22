#!/usr/bin/env bash

docker pull karalabe/xgo-latest
go get github.com/karalabe/xgo
go get github.com/GeertJohan/go.rice
go get github.com/GeertJohan/go.rice/rice
xgo github.com/daptin/daptin