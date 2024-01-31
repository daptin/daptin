#!/usr/bin/env bash

docker pull crazymax/xgo:1.12.1
go get github.com/kolaente/xgo
go get github.com/GeertJohan/go.rice
go get github.com/GeertJohan/go.rice/rice
/Users/artpar/workspace/code/daptin/xgo/xgo --image="crazymax/xgo"  --targets="*/*" --tags netgo -ldflags='-linkmode external -extldflags "-static"' .
