#!/usr/bin/env bash


rm -rf docker_dir
mkdir docker_dir

go build -o main
cp main docker_dir/main
cp -Rf gocms/dist docker_dir/dist
cp -Rf gocms/static docker_dir/static
cp Dockerfile docker_dir/Dockerfile

cd docker_dir
docker build -t gocms .

docker ps | grep gocms

cd ..
rm -rf docker_dir
