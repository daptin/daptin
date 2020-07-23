## _config table

Entries:


| Setting Name | Purpose |
|--------------|---------|
| hostname    | used for identification as IMAP/SMTP server|
|jwt.secret | used for signing the jwt tokens issue at login|
|logs.enable | enable/disable the /_logs endpoint which streams live logs|
|encryption.secret | secret used to encrypt data for storing in encrypted columns|
|jwt.token.issuer | issuer identifier in the jwt tokens|
|rclone.retries | number of default retries set for rclone related actions|
|imap.enabled | enable/disable IMAP endpoint |
|jwt.token.life.hours | the life time of tokens issued at login|
|totp.secret | TOTP secret used for CSRF token generation and 2factor token generator |a

### Get value _config table API

```bash
curl \
-H "Authorization: Bearer <ADMIN_TOKEN>" localhost:6336/_config/backend/<setting.name>
```

### Set new value _config table API

```bash
curl \
-H "Authorization: Bearer <ADMIN_TOKEN>" localhost:6336/_config/backend/<setting.name> \
-- data "New Value"
```
