#!/usr/bin/env bash


rm -rf docker_dir
mkdir docker_dir

#cd webgoms
#npm run build
#cd ..

go build  -ldflags '-linkmode external -extldflags -static -w' main.go

cp main docker_dir/goms
cp -Rf webgoms/dist docker_dir/static

cp Dockerfile docker_dir/Dockerfile

cd docker_dir
docker build -t goms/goms  .

cd ..
