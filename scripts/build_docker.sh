#!/usr/bin/env bash


cd daptinweb
# npm run build
cd ..
go get github.com/GeertJohan/go.rice
go get github.com/GeertJohan/go.rice/rice
# curl https://glide.sh/get | sh
echo "start go get"
# glide install
echo "finish go get"
rm -rf rice-box.go
rice embed-go
CGO_ENABLED=1

for GOOS1 in darwin linux; do
    export GOOS=$GOOS1
    echo "Building $GOOS-$GOARCH"
    go build -ldflags '-linkmode external -extldflags -static -w' -o bin/daptin-$GOOS-$GOARCH
done

export GOOS=windows
export CC="i686-w64-mingw32-gcc -fno-stack-protector -D_FORTIFY_SOURCE=0 -lssp"
export GOARCH=386
go build -ldflags '-linkmode external -extldflags -static -w' -o bin/daptin-$GOOS-$GOARCH

export GOOS=windows
export CC="x86_64-w64-mingw32-gcc -fno-stack-protector -D_FORTIFY_SOURCE=0 -lssp"
export GOARCH=amd64
go build -ldflags '-linkmode external -extldflags -static -w' -o bin/daptin-$GOOS-$GOARCH


rice append --exec main

rm -rf docker_dir
mkdir docker_dir


