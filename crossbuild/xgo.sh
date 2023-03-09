#!/usr/bin/env bash

docker pull kolaente/xgo-latest
go get github.com/kolaente/xgo
go get github.com/GeertJohan/go.rice
go get github.com/GeertJohan/go.rice/rice
/Users/artpar/workspace/code/daptin/xgo/xgo  --targets="*/*" --tags netgo -ldflags='-linkmode external -extldflags "-static"' .
