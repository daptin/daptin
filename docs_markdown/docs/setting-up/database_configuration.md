# Database

Daptin can use one of the following database for data persistence

- Mysql
- Postgres
- SQLite [Default]

If nothing specified, a **sqlite** database is created on the local file system and is used for all purposes. (uploads/blobs are not stored in database)

You can customise the database connection properties when starting daptin

## mysql

To use mysql, start daptin as follows

```./daptin -db_type=mysql -db_connection_string='<username>:<password>@tcp(<hostname>:<port>)/<db_name>'```

## postgres

```./daptin -db_type=postgres -db_connection_string='host=<hostname> port=<port> user=<username> password=<password> dbname=<db_name> sslmode=enable/disable'```

## sqlite

By default a "daptin.db" file is created to store data

```./daptin -db_type=sqlite -db_connection_string=db_file_name.db```
