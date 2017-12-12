# Setting up Daptin

You can setup daptin on any machine/server of your choice.

## Native binary

Daptin is available as a native binary. You can fetch the lastest binary from the releases

[https://github.com/daptin/daptin/releases](https://github.com/daptin/daptin/releases)

To start daptin, execute ```./daptin``` which will create a local sqlite database and start listening on port 6336. To change the database or port, read below.

## Docker

A docker image is also available which can be deployed on any docker compatible hosting provider (aws, gce, linode, digitalocean, azure)

<img src="/images/aws.png" style="width: 200px">
<img src="/images/digitalocean.jpg" style="width: 200px">
<img src="/images/gce.png" style="width: 200px">
<img src="/images/linode.jpg" style="width: 200px">
<img src="/images/azure.jpg" style="width: 200px">

[https://hub.docker.com/r/daptin/daptin/](https://hub.docker.com/r/daptin/daptin/)

To start daptin using docker

```docker run daptin/daptin```

## Docker compose

```yaml
version: '3'
services:
    web:
        image: devsupport
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


## Kubernetes

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

## Database and data persistence

Daptin can use one of the following database for data persistence

- Mysql
- Postgres
- SQLite [Default]

If nothing specified, a sqlite database is created on the local file system and is used for all purposes. (uploads/blobs are not stored in database)

You can customise the database connection properties when starting daptin

### mysql

To use mysql, start daptin as follows

```./daptin -db_type=mysql -db_connection_string='<username>:<password>@tcp(<hostname>:<port>)/<db_name>'```

### postgres

```./daptin -db_type=postgres -db_connection_string='host=<hostname> port=<port> user=<username> password=<password> dbname=<db_name> sslmode=enable/disable'```

### sqlite

By default a "daptin.db" file is created to store data

```./daptin -db_type=sqlite -db_connection_string=db_file_name.db```

## Port

Daptin will listen on port 6336 by default. You can change it by using the following argument

```-port=8080```

## Restart

Daptin relies on self restarts to configure new entities and apis and changes to the other parts of the ststem. As soon as you upload a schema file, daptin will write the file to disk, and restart itself. When it starts it will read the schema file, make appropriate changes to the database and expose JSON apis for the entities and actions.

You can issue a daptin restart from the dashboard. Daptin takes about 15 seconds approx to start up and configure everything.