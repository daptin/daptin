# _config table

Global configurations are stored in `_config`. `_config` table is not exposed as CRUD API like other tables.

Only users who belong to administrators group cann reading/writing config entries from the config API.

Any change in _config requires a re-init of daptin for them to take effect.

## Hostname

Used for identification as IMAP/SMTP server Default value from os hostname

## JWT secret

Used for signing the jwt tokens issue at login

Changing this would force all users logout

## Encryption secret

Secret used to encrypt data for storing in encrypted columns

Changing the secret would make the data stored in all encrypted column unrecoverable.

## JWT token issuer

The issuer name for JWT tokens

## Language default

The default language expected in the Accept-Language header. Different value in Accept-Language header in request will
trigger a lookup in corresponding translations table if enabled.

## Max connection limit

The limit for max connections from a single IP

## Rate limit

The limit for request rate limit per minute

## Enable Graphql

Graphql endpoint `/graphql` is disabled by default. Set to true to use graphql endpoint

## Enable IMAP

IMAP interface is disabled by default. Set to true to start listening to IMAP port

## JWT token lifetime (hours)

Life time in hours of JWT tokens generated for login

## TOTP secret

TOTP secret used for CSRF token generation and 2factor token generator

## Enable FTP

FTP interface for sites is disabled by default (even if enabled per site). Set to true to start FTP services.

# Default values

| id |         name          | configtype | configstate | configenv |                value                 | valuetype | previousvalue |         created_at         | updated_at |
|----|-----------------------|------------|-------------|-----------|--------------------------------------|-----------|---------------|----------------------------|------------|
| 1 | hostname              | backend    | enabled     | release   | abbad.local                          |           |               | 2021-01-02 15:11:56.836475 |
| 3 | language.default      | backend    | enabled     | release   | en                                   |           |               | 2021-01-02 15:11:56.95177  |
| 4 | limit.max_connections | backend    | enabled     | release   | 100                                  |           |               | 2021-01-02 15:11:56.96863  |
| 5 | limit.rate            | backend    | enabled     | release   | 100                                  |           |               | 2021-01-02 15:11:56.990064 |
| 6 | jwt.secret            | backend    | enabled     | release   | d4f5ca52-74d3-4a50-ae6e-27b72be759b0 |           |               | 2021-01-02 15:11:57.026539 |
| 8 | graphql.enable        | backend    | enabled     | release   | false                                |           |               | 2021-01-02 15:11:57.100476 |
| 9 | encryption.secret     | backend    | enabled     | release   | 1cdb8101fc0047e688f24c9071de76f0     |           |               | 2021-01-02 15:11:57.128269 |
| 10 | jwt.token.issuer      | backend    | enabled     | release   | daptin-40f1e5                        |           |               | 2021-01-02 15:11:57.148896 |
| 11 | rclone.retries        | backend    | enabled     | release   | 5                                    |           |               | 2021-01-02 15:11:57.470469 |
| 12 | imap.enabled          | backend    | enabled     | release   | false                                |           |               | 2021-01-02 15:11:57.523543 |
| 13 | jwt.token.life.hours  | backend    | enabled     | release   | 72                                   |           |               | 2021-01-02 15:11:57.709687 |
| 14 | totp.secret           | backend    | enabled     | release   | 2DOEBQZYQBITVPTW                     |           |               | 2021-01-02 15:11:57.752502 |
| 15 | ftp.enable            | backend    | enabled     | release   | false                                |           |               | 2021-01-02 15:11:57.999189 |

### Get value _config table API

```bash
curl \
-H "Authorization: Bearer <ADMIN_TOKEN>" http://localhost:6336/_config/backend/<setting.name>
```

### Set new value _config table API

```bash
curl \
-H "Authorization: Bearer <ADMIN_TOKEN>" http://localhost:6336/_config/backend/<setting.name> \
--data "New Value"
```
