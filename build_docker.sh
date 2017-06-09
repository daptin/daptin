#!/usr/bin/env bash


rm -rf docker_dir
mkdir docker_dir
cp main docker_dir/goms

#cd gocms
#npm run build
#cd ..

go build  -ldflags '-linkmode external -extldflags -static -w' main.go

cp -Rf gocms/dist docker_dir/static

cp Dockerfile docker_dir/Dockerfile

cd docker_dir
docker build -t goms/goms  .

cd ..
