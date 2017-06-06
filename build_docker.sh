#!/usr/bin/env bash



#cd gocms
#npm run build
#cd ..

go build  -ldflags '-linkmode external -extldflags -static -w' main.go

cp -Rf gocms/dist docker_dir/static

cp Dockerfile docker_dir/Dockerfile

cd docker_dir
docker build -t goms .

cd ..
