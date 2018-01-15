#!/usr/bin/env bash


cd daptinweb
# npm run build
cd ..
go get github.com/GeertJohan/go.rice
go get github.com/GeertJohan/go.rice/rice
curl https://glide.sh/get | sh
echo "start go get"
glide install
echo "finish go get"
rm -rf rice-box.go
rice embed-go
CGO_ENABLED=1

go build  -ldflags '-linkmode external -extldflags -static -w' main.go
rice append --exec main

rm -rf docker_dir
mkdir docker_dir


