#!/usr/bin/env bash

docker pull karalabe/xgo-latest
go get github.com/karalabe/xgo
xgo github.com/daptin/daptin