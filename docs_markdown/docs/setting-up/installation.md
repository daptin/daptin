# Installation

## Deploying a new instance

| Deployment preference      | Getting started                                                                                                               |
| -------------------------- | ----------------------------------------------------------------------------------------------------------------------------- |
| Heroku                     | [![Deploy](https://www.herokucdn.com/deploy/button.svg)](https://heroku.com/deploy?template=https://github.com/daptin/daptin) |
| Docker                     | docker run -p 8080:8080 [daptin/daptin](https://hub.docker.com/r/daptin/daptin)                                               |
| Kubernetes                 | [Service & Deployment YAML](#kubernetes)                                                                                      |
| Development                | go get github.com/daptin/daptin                                                                                               |
| Linux (386/amd64/arm5,6,7) | [Download static linux builds](https://github.com/daptin/daptin/releases)                                                     |
| Windows                    | go get github.com/daptin/daptin                                                                                               |
| OS X                       | go get github.com/daptin/daptin                                                                                               |
| Load testing               | [Docker compose](#docker-compose)                                                                                             |
| Raspberry Pi               | [Linux arm 7 static build](https://github.com/daptin/daptin/releases)                                                         |



### Native binary

Daptin is available as a native binary. You can download the binary for the following os from [github releases](https://github.com/daptin/daptin/releases)

- Windows 32/64
- OS X  64
- Linux  32/64/arm/mips

[https://github.com/daptin/daptin/releases](https://github.com/daptin/daptin/releases)

Execute ```./daptin``` to run daptin.

It will create a sqlite database on the disk and start listening on port 6336.

### CLI Options:

Argument | Definition
--- | ---
port | set the port to listen
http_port | set the https port to listen
runtime | runtime test/debug/release for logs
dashboard | path to default dashboard static build served at [ <listen_address>/ ]
db_type | mysql/postgres/sqlite3
db_connection_string |   Database Connection String


### Database connection string

#### SQLite

```-db_connection_string test.db```

#### MySQL

```-db_connection_string "<username>:<password>@tcp(<hostname>:<port>)/<db_name>"```

#### PostgreSQL:

```-db_connection_string "host=<hostname> port=<port> user=<username> password=<password> dbname=<db_name> sslmode=enable/disable"```

### Heroku deployment

Heroku is the best way to test out a live instance of daptin. Daptin has a very low memory footprint and can run smoothly even on heroku's smallest instance.

[![Deploy](https://www.herokucdn.com/deploy/button.svg)](https://heroku.com/deploy?template=https://github.com/daptin/daptin)

Note: Heroku puts instances to sleep after 30 minutes of idleness, which will erase all the data. It will behave like a fresh instance when it wakes up. You can subscribe to their minimum paid plan to remove this sleep due to idleness.

### Docker image

Deploy the docker image

Start ```daptin``` on your machine using docker

```docker run -p 8080:8080 daptin/daptin```


<a target=_blank href=https://hub.docker.com/r/daptin/daptin/>
    <img width="200px" class="cloud-provider" src="/images/aws.png">
    <img width="200px" class="cloud-provider" src="/images/digitalocean.jpg">
    <img width="200px" class="cloud-provider" src="/images/gce.png">
    <img width="200px" class="cloud-provider" src="/images/linode.jpg">
    <img width="200px" class="cloud-provider" src="/images/azure.jpg">
    <img width="200px" class="cloud-provider" src="/images/docker.png">
</a>

[https://hub.docker.com/r/daptin/daptin/](https://hub.docker.com/r/daptin/daptin/)


### Docker-compose

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


### Kubernetes deployment

Daptin can be infinitely scaled on kubernetes

!!! example
    ```yaml
    apiVersion: v1
    kind: Service
    metadata:
      name: daptin-instance
      labels:
        app: daptin
    spec:
      ports:
        - port: 8080
      selector:
        app: daptin
        tier: production
    ---
    apiVersion: extensions/v1beta1
    kind: Deployment
    metadata:
      name: daptin-daptin
      labels:
        app: daptin
    spec:
      strategy:
        type: Recreate
      template:
        metadata:
          labels:
            app: daptin
            tier: testing
        spec:
          containers:
          - image: daptin/daptin:latest
            name: daptin
            args: ['-db_type', 'mysql', '-db_connection_string', 'user:password@tcp(<mysql_service>:3306)/daptin']
            ports:
            - containerPort: 8080
              name: daptin
    ---
    apiVersion: extensions/v1beta1
    kind: Ingress
    metadata:
      name: daptin-test
    spec:
      rules:
      - host: hello.website
        http:
          paths:
          - backend:
              serviceName: daptin-testing
              servicePort: 8080
    ```


## Database configuration

Daptin can use one of the following database for data persistence

- Mysql
- Postgres
- SQLite [Default]

If nothing specified, a **sqlite** database is created on the local file system and is used for all purposes. (uploads/blobs are not stored in database)

You can customise the database connection properties when starting daptin

### MySQL

To use mysql, start daptin as follows

```./daptin -db_type=mysql -db_connection_string='<username>:<password>@tcp(<hostname>:<port>)/<db_name>'```

### PostgreSQL

```./daptin -db_type=postgres -db_connection_string='host=<hostname> port=<port> user=<username> password=<password> dbname=<db_name> sslmode=enable/disable'```

### SQLite

By default a "daptin.db" file is created to store data

```./daptin -db_type=sqlite -db_connection_string=db_file_name.db```


## Port

Daptin will use the following ports for various services (when enabled)

```-port :8080```

| Service             | Port               | To change                                        |
| ------------------- | ------------------ | ------------------------------------------------ |
| HTTP (JSON/GraphQL) | 6336               | CLI option ```-port :80```                       |
| HTTPS               | 6443               | CLI option ```-https_port :80```                 |
| IMAP                | 6443               | [_config entry](/setting-up/enabling-features) |
| SMTP                | 2525               | [/mail_server](/features/enable-smtp-imap) row entry                       |



## Restart

Various low level configure changes requires a reset of the server to take place. Restart can be triggered using an action API and takes about 5-10 seconds.

You can issue a daptin restart from the dashboard. Daptin takes about 15 seconds approx to start up and configure everything.
