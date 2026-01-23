# Actions Overview

Actions are Daptin's business logic layer for custom operations beyond CRUD.

## Executing Actions

```bash
curl -X POST http://localhost:6336/action/{entity}/{action_name} \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"attributes": {...}}'
```

## Action Response Types

| ResponseType | Description |
|--------------|-------------|
| `client.notify` | Show notification message |
| `client.redirect` | Redirect to URL |
| `client.store.set` | Store value (localStorage) |
| `client.cookie.set` | Set browser cookie |
| `client.file.download` | Trigger file download |

### Example Response

```json
[
  {
    "ResponseType": "client.notify",
    "Attributes": {
      "type": "success",
      "title": "Success",
      "message": "Operation completed"
    }
  },
  {
    "ResponseType": "client.redirect",
    "Attributes": {
      "location": "/dashboard",
      "delay": 2000
    }
  }
]
```

## Built-in Actions (46 Total)

### User Management

| Action | Entity | Description |
|--------|--------|-------------|
| `signup` | user_account | Register new user |
| `signin` | user_account | Authenticate user |
| `generate_password_reset_flow` | user_account | Request password reset |
| `generate_password_reset_verify_flow` | user_account | Complete password reset |
| `otp_generate` | user_account | Setup 2FA |
| `otp_login_verify` | user_account | Verify 2FA code |
| `switch_session_user` | user_account | Admin impersonation |
| `generate_jwt_token` | user_account | Create JWT token |

### OAuth

| Action | Entity | Description |
|--------|--------|-------------|
| `oauth_login_begin` | oauth_connect | Start OAuth flow |
| `oauth_login_response` | oauth_connect | Handle OAuth callback |
| `oauth_profile_exchange` | oauth_connect | Get OAuth profile |
| `generate_oauth2_token` | oauth_token | Generate OAuth token |

### Administration

| Action | Entity | Description |
|--------|--------|-------------|
| `become_an_administrator` | world | First user becomes admin |
| `restart_daptin` | world | Restart server |
| `enable_graphql` | world | Enable GraphQL API |
| `download_cms_config` | world | Export system config |

### Data Import/Export

| Action | Entity | Description |
|--------|--------|-------------|
| `import_data` | * | Import data (JSON/CSV/XLSX) |
| `export_data` | * | Export data |
| `export_csv_data` | * | Export as CSV |
| `csv_to_entity` | world | Create table from CSV |
| `xls_to_entity` | world | Create table from XLSX |

### Schema Operations

| Action | Entity | Description |
|--------|--------|-------------|
| `delete_table` | world | Drop table |
| `rename_column` | world | Rename column |
| `delete_column` | world | Drop column |

### Cloud Storage

| Action | Entity | Description |
|--------|--------|-------------|
| `cloudstore_file_upload` | cloud_store | Upload file |
| `cloudstore_file_delete` | cloud_store | Delete file |
| `cloudstore_folder_create` | cloud_store | Create folder |
| `cloudstore_path_move` | cloud_store | Move file/folder |
| `cloudstore_site_create` | cloud_store | Create subsite |
| `column_sync_storage` | * | Sync asset column |
| `import_cloudstore_files` | cloud_store | Import from storage |
| `site_file_get` | site | Get subsite file |
| `site_file_list` | site | List subsite files |
| `site_sync_storage` | site | Sync subsite storage |

### Email

| Action | Entity | Description |
|--------|--------|-------------|
| `mail.send` | world | Send email via SMTP |
| `aws.mail.send` | world | Send email via AWS SES |
| `mail_servers_sync` | mail_server | Sync mail config |

### Certificates

| Action | Entity | Description |
|--------|--------|-------------|
| `generate_self_tls_certificate` | world | Generate self-signed cert |
| `generate_acme_tls_certificate` | world | Get Let's Encrypt cert |
| `download_certificate` | certificate | Export certificate |

### Utilities

| Action | Entity | Description |
|--------|--------|-------------|
| `network_request` | integration | HTTP request |
| `render_template` | * | Template rendering |
| `transaction` | world | Transaction control |
| `execute_process` | world | Run external process |
| `random_value_generate` | * | Generate random value |
| `generate_random_data` | * | Generate test data |
| `make_response` | * | Custom response |
| `get_action_schema` | action | Export action schema |
| `integration_install` | integration | Install integration |
| `integration_execute` | integration | Run integration |

## List Available Actions

```bash
# All actions
curl http://localhost:6336/api/action \
  -H "Authorization: Bearer $TOKEN"

# Actions for specific entity
curl 'http://localhost:6336/api/action?query=[{"column":"on_type","operator":"is","value":"todo"}]' \
  -H "Authorization: Bearer $TOKEN"
```

## Action Definition

Actions defined in schema:

```yaml
Actions:
  - Name: publish_article
    Label: Publish Article
    OnType: article
    InFields:
      - Name: article_id
        ColumnType: id
        ColumnName: reference_id
    OutFields:
      - Type: client.notify
        Attributes:
          type: success
          message: Article published
    Validations: []
    Conformations: []
```

### Action Properties

| Property | Description |
|----------|-------------|
| `Name` | Unique action identifier |
| `Label` | Display name |
| `OnType` | Entity this action belongs to |
| `InFields` | Input parameters |
| `OutFields` | Output responses |
| `Validations` | Input validation rules |
| `Conformations` | Data transformations |
| `InstanceOptional` | Don't require record ID |

## InField Types

```yaml
InFields:
  - Name: title
    ColumnType: label
    IsNullable: false

  - Name: category
    ColumnType: enum
    Values: [news, blog, review]

  - Name: file
    ColumnType: file.document

  - Name: publish_date
    ColumnType: datetime
```

## Permission Control

```yaml
Actions:
  - Name: admin_action
    OnType: world
    RequiredPermission: Execute
    # Only users with Execute permission can run
```

## Custom Actions

See [[Custom-Actions]] for creating your own actions.
