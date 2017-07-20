
rm -rf docker_dir
mkdir docker_dir

cp main docker_dir/main
cp -Rf gomsweb/dist docker_dir/static

cp Dockerfile docker_dir/Dockerfile

cd docker_dir
docker build -t goms/goms  .

cd ..
docker images | grep goms | grep latest