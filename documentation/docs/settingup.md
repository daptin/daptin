# Setting up GoMS

You can setup goms on any machine/server of your choice.

## Native binary

GoMS is available as a native binary for almost all systems. You can fetch the lastest binary from the releases

[https://github.com/artpar/goms/releases](https://github.com/artpar/goms/releases)

## Docker

A docker image is also available which can be deployed on any docker compatible host

[https://hub.docker.com/r/goms/goms/](https://hub.docker.com/r/goms/goms/)

To start goms using docker

```docker run goms/goms```

## Data storage

GoMS can use one of the following database for persistence

- Mysql
- Postgres
- SQLite

If nothing specified, a sqlite database is created on the local file system and is used for all purposes. (uploads/blobs are not stored in database)

You can customise the database connection properties when starting goms

### mysql

To use mysql, start goms as follows

```./goms -db_type=mysql -db_connection_string='<username>:<password>@tcp(<hostname>:<port>)/<db_name>'```

### postgres

```./goms -db_type=postgres -db_connection_string='host=<hostname> port=<port> user=<username> password=<password> dbname=<db_name> sslmode=enable/disable'```

### sqlite

By default a "goms.db" file is created to store data

```./goms -db_type=sqlite -db_connection_string=db_file_name.db```

## Port

GoMS will listen on port 6336 by default. You can change it by using the following argument

```-port=8080```

## Restart

Goms relies on self restarts to configure new entities and apis. As soon as you upload a schema file, goms will write the file to disk, and restart itself. When it starts it will read the schema file, make appropriate changes to the database and expose JSON apis for the entities and actions.

You can issue a goms restart from the dashboard. Goms takes about 15 seconds approx to start up and configure everything.