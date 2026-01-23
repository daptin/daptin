# Action Reference

Complete reference of all built-in actions in Daptin.

## User Account Actions

### signup

Register a new user account.

| Property | Value |
|----------|-------|
| Entity | `user_account` |
| Instance Required | No |

**Input Fields:**
| Field | Type | Required |
|-------|------|----------|
| name | label | Yes |
| email | email | Yes |
| mobile | label | No |
| password | password | Yes |
| passwordConfirm | password | Yes |

```bash
curl -X POST http://localhost:6336/action/user_account/signup \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "name": "John Doe",
      "email": "john@example.com",
      "password": "secure123",
      "passwordConfirm": "secure123"
    }
  }'
```

### signin

Authenticate and get JWT token.

| Property | Value |
|----------|-------|
| Entity | `user_account` |
| Instance Required | No |

**Input Fields:**
| Field | Type | Required |
|-------|------|----------|
| email | email | Yes |
| password | password | Yes |

```bash
curl -X POST http://localhost:6336/action/user_account/signin \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "email": "john@example.com",
      "password": "secure123"
    }
  }'
```

### reset-password

Initiate password reset flow.

| Property | Value |
|----------|-------|
| Entity | `user_account` |
| Instance Required | No |

**Input Fields:**
| Field | Type | Required |
|-------|------|----------|
| email | email | Yes |

```bash
curl -X POST http://localhost:6336/action/user_account/reset-password \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "email": "john@example.com"
    }
  }'
```

### reset-password-verify

Complete password reset with verification code.

| Property | Value |
|----------|-------|
| Entity | `user_account` |
| Instance Required | No |

**Input Fields:**
| Field | Type | Required |
|-------|------|----------|
| email | email | Yes |
| otp | value | Yes |

## OTP Actions

### register_otp

Register a mobile number for OTP authentication.

| Property | Value |
|----------|-------|
| Entity | `user_account` |
| Instance Required | Yes |

**Input Fields:**
| Field | Type | Required |
|-------|------|----------|
| mobile_number | label | Yes |

### verify_otp

Login using OTP code.

| Property | Value |
|----------|-------|
| Entity | `user_account` |
| Instance Required | No |

**Input Fields:**
| Field | Type | Required |
|-------|------|----------|
| otp | label | Yes |
| mobile_number | label | No |
| email | label | No |

### send_otp

Send OTP to registered mobile number.

| Property | Value |
|----------|-------|
| Entity | `user_otp_account` |
| Instance Required | No |

**Input Fields:**
| Field | Type | Required |
|-------|------|----------|
| mobile_number | label | Yes |
| email | label | No |

### verify_mobile_number

Verify a mobile number with OTP.

| Property | Value |
|----------|-------|
| Entity | `user_otp_account` |
| Instance Required | No |

**Input Fields:**
| Field | Type | Required |
|-------|------|----------|
| mobile_number | label | Yes |
| email | label | No |
| otp | label | Yes |

## OAuth Actions

### oauth_login_begin

Start OAuth authentication flow.

| Property | Value |
|----------|-------|
| Entity | `oauth_connect` |
| Instance Required | Yes |

**Action Performer:** `oauth.client.redirect`

### oauth.login.response

Handle OAuth callback with code and state.

| Property | Value |
|----------|-------|
| Entity | `oauth_token` |
| Instance Required | No |

**Input Fields:**
| Field | Type | Required |
|-------|------|----------|
| code | hidden | Yes |
| state | hidden | Yes |
| authenticator | hidden | Yes |

## Admin Actions

### become_an_administrator

Promote user to system administrator (first-time setup only).

| Property | Value |
|----------|-------|
| Entity | `world` |
| Instance Required | No |

**Action Performer:** `__become_admin`

```bash
curl -X POST http://localhost:6336/action/world/become_an_administrator \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"attributes":{}}'
```

### restart_daptin

Restart the Daptin server to apply configuration changes.

| Property | Value |
|----------|-------|
| Entity | `world` |
| Instance Required | No |

```bash
curl -X POST http://localhost:6336/action/world/restart_daptin \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"attributes":{}}'
```

### download_system_schema

Download complete system schema as JSON.

| Property | Value |
|----------|-------|
| Entity | `world` |
| Instance Required | No |

**Action Performer:** `__download_cms_config`

### upload_system_schema

Upload schema file to update system configuration.

| Property | Value |
|----------|-------|
| Entity | `world` |
| Instance Required | No |

**Input Fields:**
| Field | Type | Required |
|-------|------|----------|
| schema_file | file.json\|yaml\|toml\|hcl | Yes |

## Table Management Actions

### remove_table

Delete a table from the system.

| Property | Value |
|----------|-------|
| Entity | `world` |
| Instance Required | Yes |

**Action Performer:** `world.delete`

### remove_column

Delete a column from a table.

| Property | Value |
|----------|-------|
| Entity | `world` |
| Instance Required | Yes |

**Input Fields:**
| Field | Type | Required |
|-------|------|----------|
| column_name | label | Yes |

**Action Performer:** `world.column.delete`

### rename_column

Rename an existing column.

| Property | Value |
|----------|-------|
| Entity | `world` |
| Instance Required | No |

**Input Fields:**
| Field | Type | Required |
|-------|------|----------|
| table_name | label | Yes |
| column_name | label | Yes |
| new_column_name | label | Yes |

**Action Performer:** `world.column.rename`

## Data Import/Export Actions

### import_data

Import data from dump file.

| Property | Value |
|----------|-------|
| Entity | `world` |
| Instance Required | Yes |

**Input Fields:**
| Field | Type | Required |
|-------|------|----------|
| dump_file | file.json\|yaml\|toml\|hcl\|csv\|docx\|xlsx\|pdf\|html | Yes |
| truncate_before_insert | truefalse | No |
| batch_size | measurement | No |

**Action Performer:** `__data_import`

```bash
curl -X POST http://localhost:6336/action/world/import_data/WORLD_REFERENCE_ID \
  -H "Authorization: Bearer $TOKEN" \
  -F "dump_file=@data.json" \
  -F "truncate_before_insert=false"
```

### export_data

Export table data in various formats.

| Property | Value |
|----------|-------|
| Entity | `world` |
| Instance Required | No |

**Input Fields:**
| Field | Type | Required |
|-------|------|----------|
| table_name | label | Yes |
| format | label | No (default: json) |
| columns | label | No |
| include_headers | truefalse | No |

**Action Performer:** `__data_export`

### export_csv_data

Export table data as CSV.

| Property | Value |
|----------|-------|
| Entity | `world` |
| Instance Required | No |

**Input Fields:**
| Field | Type | Required |
|-------|------|----------|
| table_name | label | Yes |

**Action Performer:** `__csv_data_export`

### upload_csv_to_system_schema

Import CSV data into entity.

| Property | Value |
|----------|-------|
| Entity | `world` |
| Instance Required | No |

**Input Fields:**
| Field | Type | Required |
|-------|------|----------|
| data_csv_file | file.csv | Yes |
| entity_name | label | Yes |
| create_if_not_exists | truefalse | No |
| add_missing_columns | truefalse | No |

**Action Performer:** `__upload_csv_file_to_entity`

### upload_xls_to_system_schema

Import Excel data into entity.

| Property | Value |
|----------|-------|
| Entity | `world` |
| Instance Required | No |

**Input Fields:**
| Field | Type | Required |
|-------|------|----------|
| data_xls_file | file.xls\|xlsx | Yes |
| entity_name | label | Yes |
| create_if_not_exists | truefalse | Yes |
| add_missing_columns | truefalse | Yes |

**Action Performer:** `__upload_xlsx_file_to_entity`

### import_files_from_store

Import files from cloud store to table.

| Property | Value |
|----------|-------|
| Entity | `world` |
| Instance Required | Yes |

**Input Fields:**
| Field | Type | Required |
|-------|------|----------|
| table_name | label | Yes |

**Action Performer:** `cloud_store.files.import`

### generate_random_data

Generate random test data for a table.

| Property | Value |
|----------|-------|
| Entity | `world` |
| Instance Required | No |

**Input Fields:**
| Field | Type | Required |
|-------|------|----------|
| count | measurement | Yes (>0) |
| table_name | label | Yes |

**Action Performer:** `generate.random.data`

## Cloud Storage Actions

### upload_file

Upload file to cloud store.

| Property | Value |
|----------|-------|
| Entity | `cloud_store` |
| Instance Required | Yes |

**Input Fields:**
| Field | Type | Required |
|-------|------|----------|
| file | file.* | Yes |
| path | label | No |

**Action Performer:** `cloudstore.file.upload`

### create_folder

Create folder in cloud store.

| Property | Value |
|----------|-------|
| Entity | `cloud_store` |
| Instance Required | Yes |

**Input Fields:**
| Field | Type | Required |
|-------|------|----------|
| path | label | No |
| name | label | Yes |

**Action Performer:** `cloudstore.folder.create`

### delete_path

Delete path from cloud store.

| Property | Value |
|----------|-------|
| Entity | `cloud_store` |
| Instance Required | Yes |

**Input Fields:**
| Field | Type | Required |
|-------|------|----------|
| path | label | Yes |

**Action Performer:** `site.file.delete`

### move_path

Move file or folder in cloud store.

| Property | Value |
|----------|-------|
| Entity | `cloud_store` |
| Instance Required | Yes |

**Input Fields:**
| Field | Type | Required |
|-------|------|----------|
| source | label | Yes |
| destination | label | Yes |

**Action Performer:** `cloudstore.path.move`

### create_site

Create a new site from cloud store.

| Property | Value |
|----------|-------|
| Entity | `cloud_store` |
| Instance Required | Yes |

**Input Fields:**
| Field | Type | Required |
|-------|------|----------|
| site_type | label | Yes |
| path | label | Yes |
| hostname | label | Yes |

**Action Performer:** `cloudstore.site.create`

## Site Actions

### list_files

List files in site path.

| Property | Value |
|----------|-------|
| Entity | `site` |
| Instance Required | Yes |

**Input Fields:**
| Field | Type | Required |
|-------|------|----------|
| path | label | Yes |

**Action Performer:** `site.file.list`

### get_file

Get file content from site.

| Property | Value |
|----------|-------|
| Entity | `site` |
| Instance Required | Yes |

**Input Fields:**
| Field | Type | Required |
|-------|------|----------|
| path | label | Yes |

**Action Performer:** `site.file.get`

### delete_file

Delete file from site.

| Property | Value |
|----------|-------|
| Entity | `site` |
| Instance Required | Yes |

**Input Fields:**
| Field | Type | Required |
|-------|------|----------|
| path | label | Yes |

**Action Performer:** `site.file.delete`

### sync_site_storage

Sync site with cloud storage.

| Property | Value |
|----------|-------|
| Entity | `site` |
| Instance Required | Yes |

**Input Fields:**
| Field | Type | Required |
|-------|------|----------|
| path | label | No |

**Action Performer:** `site.storage.sync`

### sync_column_storage

Sync column with cloud storage.

| Property | Value |
|----------|-------|
| Entity | `world` |
| Instance Required | No |

**Input Fields:**
| Field | Type | Required |
|-------|------|----------|
| table_name | label | Yes |
| column_name | label | Yes |
| credential_name | label | Yes |

**Action Performer:** `column.storage.sync`

## Certificate Actions

### generate_acme_certificate

Generate Let's Encrypt ACME certificate.

| Property | Value |
|----------|-------|
| Entity | `certificate` |
| Instance Required | Yes |

**Input Fields:**
| Field | Type | Required |
|-------|------|----------|
| email | label | Yes |

**Action Performer:** `acme.tls.generate`

```bash
curl -X POST http://localhost:6336/action/certificate/generate_acme_certificate/CERT_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "email": "admin@example.com"
    }
  }'
```

### generate_self_certificate

Generate self-signed certificate.

| Property | Value |
|----------|-------|
| Entity | `certificate` |
| Instance Required | Yes |

**Action Performer:** `self.tls.generate`

### download_certificate

Download certificate PEM file.

| Property | Value |
|----------|-------|
| Entity | `certificate` |
| Instance Required | Yes |

Returns: Certificate PEM file download

### download_public_key

Download public key PEM file.

| Property | Value |
|----------|-------|
| Entity | `certificate` |
| Instance Required | Yes |

Returns: Public key PEM file download

## Email Actions

### mail.send

Send email via SMTP.

**Action Performer:** `mail.send`

**Parameters:**
| Field | Type | Required |
|-------|------|----------|
| to | []string | Yes |
| subject | string | Yes |
| from | string | Yes |
| body | string | Yes |
| mail_server_hostname | string | No |

### aws.mail.send

Send email via AWS SES.

**Action Performer:** `aws.mail.send`

### sync_mail_servers

Sync all mail server configurations.

| Property | Value |
|----------|-------|
| Entity | `mail_server` |
| Instance Required | No |

**Action Performer:** `mail.servers.sync`

## Integration Actions

### install_integration

Install an API integration.

| Property | Value |
|----------|-------|
| Entity | `integration` |
| Instance Required | Yes |

**Action Performer:** `integration.install`

### get_action_schema

Download action schema JSON.

| Property | Value |
|----------|-------|
| Entity | `action` |
| Instance Required | Yes |

Returns: Action schema JSON file download

## Data Exchange Actions

### add_exchange

Add a data exchange for Google Sheets sync.

| Property | Value |
|----------|-------|
| Entity | `world` |
| Instance Required | Yes |

**Input Fields:**
| Field | Type | Required |
|-------|------|----------|
| name | label | Yes |
| sheet_id | alias | Yes |
| app_key | alias | Yes |

## Action Performers (Internal)

These are the internal action executors:

| Performer Name | Description |
|----------------|-------------|
| `site.storage.sync` | Sync site with cloud storage |
| `oauth.token` | Generate OAuth token |
| `world.column.rename` | Rename table column |
| `cloud_store.files.import` | Import files from cloud store |
| `oauth.client.redirect` | Start OAuth redirect flow |
| `response.create` | Create action response |
| `password.reset.verify` | Verify password reset code |
| `cloudstore.folder.create` | Create folder in cloud store |
| `column.storage.sync` | Sync column with storage |
| `command.execute` | Execute system command |
| `mail.servers.sync` | Sync mail servers |
| `password.reset.begin` | Start password reset |
| `cloudstore.site.create` | Create site from cloud store |
| `world.delete` | Delete table |
| `otp.login.verify` | Verify OTP login |
| `__download_cms_config` | Download CMS configuration |
| `site.file.list` | List site files |
| `jwt.token` | Generate JWT token |
| `self.tls.generate` | Generate self-signed certificate |
| `integration.install` | Install integration |
| `$transaction` | Transaction wrapper |
| `__csv_data_export` | Export CSV data |
| `site.file.delete` | Delete site file |
| `aws.mail.send` | Send via AWS SES |
| `world.column.delete` | Delete column |
| `random.generate` | Generate random values |
| `mail.send` | Send email |
| `oauth.login.response` | Handle OAuth response |
| `__restart` | Restart system |
| `__data_export` | Export data |
| `cloudstore.path.move` | Move cloud store path |
| `__upload_xlsx_file_to_entity` | Upload XLSX to entity |
| `__data_import` | Import data |
| `acme.tls.generate` | Generate ACME certificate |
| `__become_admin` | Become administrator |
| `$network.request` | Make network request |
| `__upload_csv_file_to_entity` | Upload CSV to entity |
| `oauth.profile.exchange` | Exchange OAuth profile |
| `site.file.get` | Get site file |
| `__enable_graphql` | Enable GraphQL API |

## Action Request Format

All actions follow this request format:

```bash
curl -X POST http://localhost:6336/action/{entity}/{action_name}[/{reference_id}] \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "field1": "value1",
      "field2": "value2"
    }
  }'
```

- `{entity}` - Target entity name (e.g., `user_account`, `world`)
- `{action_name}` - Action name as defined
- `{reference_id}` - Required if `InstanceOptional: false`

## Action Response Types

Actions can return various response types:

| Type | Description |
|------|-------------|
| `client.notify` | Display notification to user |
| `client.redirect` | Redirect browser |
| `client.file.download` | Trigger file download |
| `jwt.token` | Return JWT authentication token |
