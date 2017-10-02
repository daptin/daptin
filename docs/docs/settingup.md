# Setting up Daptin

You can setup daptin on any machine/server of your choice.

## Native binary

Daptin is available as a native binary for almost all systems. You can fetch the lastest binary from the releases

[https://github.com/artpar/daptin/releases](https://github.com/artpar/daptin/releases)

## Docker

A docker image is also available which can be deployed on any docker compatible host

[https://hub.docker.com/r/daptin/daptin/](https://hub.docker.com/r/daptin/daptin/)

To start daptin using docker

```docker run daptin/daptin```

## Data storage

Daptin can use one of the following database for persistence

- Mysql
- Postgres
- SQLite

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

Daptin relies on self restarts to configure new entities and apis. As soon as you upload a schema file, daptin will write the file to disk, and restart itself. When it starts it will read the schema file, make appropriate changes to the database and expose JSON apis for the entities and actions.

You can issue a daptin restart from the dashboard. Daptin takes about 15 seconds approx to start up and configure everything.