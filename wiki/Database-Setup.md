# Database Setup

Supported databases and configuration.

## Supported Databases

| Database | Driver | Current source status |
|----------|--------|------------------|
| SQLite | sqlite3 | Verified for development and single-node use |
| MySQL/MariaDB | mysql | MariaDB 10.11 clean initialization and restart verified |
| PostgreSQL 15 | postgres | Verified for persistence and two-node shared-database use |

The published v0.13.14 release predates the MariaDB schema corrections. See
[[Release-v0.13.14-Feature-Status]] when operating that release.

## SQLite (Default)

No configuration needed. Creates `daptin.db` in working directory.

```bash
./daptin
```

Or specify path:

```bash
DAPTIN_DB_TYPE=sqlite3 DAPTIN_DB_CONNECTION_STRING=./data/daptin.db ./daptin
```

## MySQL/MariaDB

MariaDB 10.11 is covered by a clean-schema manifest test and a restart test.
The test initializes every registered built-in table, including `document` and
the generated `exchange_run` access table.

### Connection String

```bash
DAPTIN_DB_TYPE=mysql \
DAPTIN_DB_CONNECTION_STRING="user:password@tcp(localhost:3306)/daptin?charset=utf8mb4&parseTime=True" \
./daptin
```

### Docker Compose

```yaml
version: '3'
services:
  mysql:
    image: mariadb:10.11
    environment:
      MARIADB_ROOT_PASSWORD: rootpass
      MARIADB_DATABASE: daptin
      MARIADB_USER: daptin
      MARIADB_PASSWORD: daptinpass
    volumes:
      - mysql_data:/var/lib/mysql

  daptin:
    image: daptin/daptin
    depends_on:
      - mysql
    environment:
      DAPTIN_DB_TYPE: mysql
      DAPTIN_DB_CONNECTION_STRING: "daptin:daptinpass@tcp(mysql:3306)/daptin?charset=utf8mb4&parseTime=True"
    ports:
      - "6336:6336"

volumes:
  mysql_data:
```

### MySQL Configuration

Recommended settings in `my.cnf`:

```ini
[mysqld]
character-set-server=utf8mb4
collation-server=utf8mb4_unicode_ci
max_connections=200
innodb_buffer_pool_size=1G
```

## PostgreSQL

PostgreSQL 15 is the production database path validated for v0.13.14. The
audit covered restart persistence, database stop/recovery, two Daptin nodes
sharing one database, and a 50-concurrent-create smoke test.

### Connection String

```bash
DAPTIN_DB_TYPE=postgres \
DAPTIN_DB_CONNECTION_STRING="host=localhost port=5432 user=daptin password=pass dbname=daptin sslmode=disable" \
./daptin
```

### Docker Compose

```yaml
version: '3'
services:
  postgres:
    image: postgres:15
    environment:
      POSTGRES_DB: daptin
      POSTGRES_USER: daptin
      POSTGRES_PASSWORD: daptinpass
    volumes:
      - pg_data:/var/lib/postgresql/data

  daptin:
    image: daptin/daptin
    depends_on:
      - postgres
    environment:
      DAPTIN_DB_TYPE: postgres
      DAPTIN_DB_CONNECTION_STRING: "host=postgres port=5432 user=daptin password=daptinpass dbname=daptin sslmode=disable"
    ports:
      - "6336:6336"

volumes:
  pg_data:
```

### SSL Mode Options

| Mode | Description |
|------|-------------|
| disable | No SSL |
| require | SSL without verification |
| verify-ca | Verify server certificate |
| verify-full | Full verification |

### Verify initialization

`/ready` proves that the runtime is accepting traffic and that the database is
reachable. After first start, inspect startup logs for schema errors and
perform authenticated reads of the built-in resources your deployment needs.
At minimum, verify `world`, `user_account`, `usergroup`, `action`, `task`,
`document`, and any enabled audit/resource relationship endpoints.

For a PostgreSQL operator-level schema check (read-only, not an application
workflow), confirm the minimum built-in set is present:

```sql
SELECT table_name
FROM information_schema.tables
WHERE table_schema = current_schema()
  AND table_name IN
      ('world','user_account','usergroup','action','task','document')
ORDER BY table_name;
```

Expect six rows, then extend the list with every enabled feature/audit/join
table from the deployed schema. A table-name check alone is not enough; follow
it with authenticated resource and relationship reads.

## Connection Pool

Daptin manages connection pooling automatically.

### Pool Settings

Configure via environment:

```bash
DAPTIN_DB_MAX_OPEN_CONNECTIONS=100
DAPTIN_DB_MAX_IDLE_CONNECTIONS=10
DAPTIN_DB_CONNECTION_MAX_LIFETIME=3600
```

## Database Migrations

Daptin handles migrations automatically:

1. Tables created on first run
2. Columns added when schema changes
3. Indexes created for performance

## Backup

### SQLite

```bash
cp daptin.db daptin.db.backup
```

### MySQL

```bash
mysqldump -u daptin -p daptin > backup.sql
```

### PostgreSQL

```bash
pg_dump -U daptin daptin > backup.sql
```

## Restore

### SQLite

```bash
cp daptin.db.backup daptin.db
```

### MySQL

```bash
mysql -u daptin -p daptin < backup.sql
```

### PostgreSQL

```bash
psql -U daptin daptin < backup.sql
```

## Performance Tuning

### Indexes

Daptin creates indexes for:
- Primary keys
- Foreign keys (relationships)
- Unique columns
- Columns declared with `IsIndexed: true`
- Multi-column lookup paths declared with `CompositeIndexes`
- Multi-column uniqueness rules declared with `CompositeKeys`

Use one declaration path for each intent: `IsIndexed` for a single column,
`CompositeIndexes` for two or more non-unique columns, and `CompositeKeys`
when the database must reject duplicate column combinations.

### Query Optimization

- Use pagination for large datasets
- Use filters to reduce result size
- Enable caching for read-heavy workloads

## Troubleshooting

### Connection Refused

1. Check database is running
2. Verify host/port
3. Check firewall rules

### Access Denied

1. Verify username/password
2. Check user permissions
3. Verify database exists

### Character Set Issues

Use UTF8MB4 for MySQL:

```
charset=utf8mb4
```
