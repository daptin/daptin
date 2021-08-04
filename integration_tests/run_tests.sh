#!/usr/bin/env bash
set -x

docker-compose down
docker network create my_net
testcase=$1
echo "Running test case $testcase"

host=http://daptin:8080


rm -rf db_init
cp -Rf cases/$testcase/db_init db_init
bunzip2 db_init/*.sql.bz2
ls -lah db_init
docker-compose up -d --force-recreate


ip=`docker inspect --format='{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' daptin`
echo "ip: $ip"

until $(curl --max-time 5 http://$ip:8080/api/user_account -H "Hostname: dashboard"); do
    ip=`docker inspect --format='{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' daptin`
    printf '.'
    sleep 5
done

docker ps

docker run  --network=my_net --rm -v $PWD/cases/$testcase/testcases:/tests thoom/pyresttest $host "/tests/$testcase.yml"  --log debug
