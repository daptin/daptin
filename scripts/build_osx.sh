#!/usr/bin/env bash


cd daptinweb
npm run build
cd ..


export GOPATH=/Users/artpar/workspace/gocode
rm -rf rice-box.go

#go build  -ldflags '-linkmode external -extldflags -static -w' main.go

docker run --rm -v "$PWD":/usr/src/myapp -w /usr/src/myapp golang:1.8 go get
docker run --rm -v "$PWD":/usr/src/myapp -w /usr/src/myapp golang:1.8 rice embed-go
docker run --rm -v "$PWD":/usr/src/myapp -w /usr/src/myapp golang:1.8 go build -v -ldflags '-linkmode external -extldflags -static -w'

rice append --exec main

rm -rf docker_dir
mkdir docker_dir

cp main docker_dir/main
cp -Rf daptinweb/dist docker_dir/static

cp Dockerfile docker_dir/Dockerfile

cd docker_dir
docker build -t daptin/daptin  .

cd ..
docker images | grep daptin | grep latest
