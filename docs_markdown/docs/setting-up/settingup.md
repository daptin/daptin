# Setting up Daptin

Daptin is built in golang and a static artifact is available for most targets

## Deploy and get started

| Deployment preference      | Getting started                                                                                                               |
| -------------------------- | ----------------------------------------------------------------------------------------------------------------------------- |
| Heroku                     | [![Deploy](https://www.herokucdn.com/deploy/button.svg)](https://heroku.com/deploy?template=https://github.com/daptin/daptin) |
| Docker                     | docker run -p 8080:8080 daptin/daptin                                                                                         |
| Kubernetes                 | [Service & Deployment YAML](#kubernetes)                                                                                      |
| Development                | go get github.com/daptin/daptin                                                                                               |
| Linux (386/amd64/arm5,6,7) | [Download static linux builds](https://github.com/daptin/daptin/releases)                                                     |
| Windows                    | go get github.com/daptin/daptin                                                                                               |
| OS X                       | go get github.com/daptin/daptin                                                                                               |
| Load testing               | [Docker compose](#docker-compose)                                                                                             |
| Raspberry Pi               | [Linux arm 7 static build](https://github.com/daptin/daptin/releases)                                                         |



# Next

The first thing you want to do after deploying a new instance is register yourself as a user on the dashboard. This part can be automated for redistributable applications.

## Become admin

Only the first user can become an **administrator** and only until no one else signs up. If the first user doesn't invoke "Become admin" before another user signs up, then it becomes a public instance which is something you would rarely want.

When you "Become admin", daptin will restart itself and schedule an update for itself where it makes you [the owner](/auth/authorization.md) of everything and update permission of all exposed apis. At this point guest users will not be allowed to invoke sign up process.

## Usage Roadmap

Quick road map to various things you can do from here:

* [Enable sign up for guests](/auth/users_and_usergroups.md#sign-up)
* [Expose APIs](/setting-up/entities.md)
* [Set access permission](/auth/permissions.md)
* [Get a client library](http://jsonapi.org/implementations) for your frontend
* [Enable Auditing](/data-modeling/auditing.md) to maintain change logs
* [Connect to a cloud storage](/data-modeling/data_storage.md)
* Host a [static-site](/subsite/subsite.md)


## Import data

* [XLS](/actions-streams/default_actions/#upload-xls)
* [CSV](/actions-streams/default_actions/#upload-csv)
* [JSON](/actions-streams/default_actions/#upload-json)
* [Schema](/actions-streams/default_actions/#upload-schema)



# Detailed instructions

## Native binary

Daptin is available as a native binary. You can fetch the lastest binary from the releases

[https://github.com/daptin/daptin/releases](https://github.com/daptin/daptin/releases)

To start daptin, execute ```./daptin``` which will create a local sqlite database and start listening on port 6336. To change the database or port, read below.

## Docker image

Deploy the docker image is on any docker compatible hosting provider (aws, gce, linode, digitalocean, azure)

<a target=_blank href=https://hub.docker.com/r/daptin/daptin/>
<img class="cloud-provider" src="/images/aws.png">
<img class="cloud-provider" src="/images/digitalocean.jpg">
<img class="cloud-provider" src="/images/gce.png">
<img class="cloud-provider" src="/images/linode.jpg">
<img class="cloud-provider" src="/images/azure.jpg">
<img class="cloud-provider" src="/images/docker.png">
</a>
[https://hub.docker.com/r/daptin/daptin/](https://hub.docker.com/r/daptin/daptin/)

To start daptin on your local machine using docker

```docker run -p 8080:8080 daptin/daptin```

## Docker compose

Docker compose is a great tool to bring up a mysql/postgres backed daptin instance


!!! example
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


## Kubernetes deployment

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
