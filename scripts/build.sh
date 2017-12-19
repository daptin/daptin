#!/usr/bin/env bash


cd daptinweb
npm run build
cd ..
echo "start go get"
# glide install
echo "finish go get"
go get github.com/artpar/goagain
export GOPATH=/media/artpar/ddrive/workspace/newgocode
rm -rf rice-box.go
rice embed-go
go build  -ldflags '-linkmode external -extldflags -static -w' main.go
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
