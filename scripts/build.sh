#!/usr/bin/env bash


cd daptinweb
npm run build
cd ..
echo "start go get"
# glide install
echo "finish go get"
go get github.com/artpar/goagain
export GOPATH=/media/artpar/ddrive/workspace/newgocode
rm -f cmd/daptin/rice-box.go
(cd cmd/daptin && rice embed-go)
CGO_ENABLED=1
go build -o main -ldflags '-linkmode external -extldflags -static -w' ./cmd/daptin
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
