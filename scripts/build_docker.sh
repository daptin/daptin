
rm -rf docker_dir
mkdir docker_dir

cp main docker_dir/main
cp -Rf daptinweb/dist docker_dir/static

cp Dockerfile docker_dir/Dockerfile

cd docker_dir
docker build -t daptin/daptin  .

cd ..
docker images | grep daptin | grep latest