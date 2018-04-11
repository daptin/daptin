### Docker compose

Docker compose is a great tool to bring up a mysql/postgres backed daptin instance


```yaml
version: '3'
services:
    web:
        image: daptin/daptin
        ports:
            - "8090:8080"
        restart: always
        environment:
          DAPTIN_PORT: '8080'
          DAPTIN_DB_TYPE: 'mysql'
          DAPTIN_DB_CONNECTION_STRING: 'dev:dev@tcp(mysqldb:3306)/daptin'
        depends_on:
            - mysqldb
    mysqldb:
        image: mysql
        container_name: ${MYSQL_HOST}
        restart: always
        env_file:
            - ".env"
        environment:
            - MYSQL_DATABASE=${MYSQL_DATABASE}
            - MYSQL_ROOT_PASSWORD=${MYSQL_ROOT_PASSWORD}
            - MYSQL_USER=${MYSQL_USER}
            - MYSQL_PASSWORD=${MYSQL_PASSWORD}
        ports:
            - "8989:3306"
        volumes:
            - "./data/db/mysql:/var/lib/mysql"
```
