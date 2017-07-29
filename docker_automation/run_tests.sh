#!/usr/bin/env bash


docker-compose down

testcase=$1
echo "Running test case $testcase"

host=http://goms:8080


rm -rf db_init
cp -Rf case_$testcase/db_init db_init

docker-compose up -d --force-recreate


ip=`docker inspect --format='{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' goms`


until $(curl --output /dev/null --silent --fail http://$ip:8080/api/user); do
    ip=`docker inspect --format='{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' goms`
    printf '.'
    sleep 5
done



docker ps

docker run  --network=my_net --rm -v $PWD/case_$testcase/testcases:/tests thoom/pyresttest $host "/tests/users.yml"  --log debug
