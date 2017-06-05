#!/usr/bin/env bash


rm -rf docker_dir
mkdir docker_dir

go build -o main

cd gocms
npm run build
cd ..

cp main docker_dir/main
cp -Rf gocms/dist docker_dir/static

cp Dockerfile docker_dir/Dockerfile

cd docker_dir
docker build -t goms .

cd ..
