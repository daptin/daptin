# Native binary

Daptin is available as a native binary. You can download the binary for the following os from [github releases](https://github.com/daptin/daptin/releases)

- Windows 32/64
- OS X  64
- Linux  32/64/arm/mips

[https://github.com/daptin/daptin/releases](https://github.com/daptin/daptin/releases)

Execute ```./daptin``` to run daptin.

It will create a sqlite database on the disk and start listening on port 6336.

Arguments:

- -port: set the port to listen
- -db_type: mysql/postgres/sqlite3
- -db_connection_string:
  - SQLite: ```test.db```
  - MySql: ```<username>:<password>@tcp(<hostname>:<port>)/<db_name>```
  - Postgres: ```host=<hostname> port=<port> user=<username> password=<password> dbname=<db_name> sslmode=enable/disable```