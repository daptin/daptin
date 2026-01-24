# Credentials

Secure storage for sensitive authentication data.

**Related**: [Cloud Storage](Cloud-Storage.md) | [Integrations](Integrations.md)

**Source of truth**: `server/resource/columns.go` (credential table)

---

## Overview

The `credential` table provides encrypted storage for sensitive data like:
- API keys and secrets
- Service account passwords
- SSH keys and tokens
- Database connection strings

Credentials are linked to other entities (like `cloud_store`) to provide authentication without exposing secrets in those records.

---

## Credential Table

| Column | Type | Description |
|--------|------|-------------|
| `name` | label | Human-readable identifier (indexed) |
| `content` | encrypted | Sensitive data (encrypted at rest) |

**Note**: This table has `DefaultGroups: adminsGroup` - only administrators can manage credentials.

---

## Create Credential

**Admin required**.

```bash
curl -X POST http://localhost:6336/api/credential \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "credential",
      "attributes": {
        "name": "aws-s3-access",
        "content": "{\"access_key\": \"AKIAIOSFODNN7EXAMPLE\", \"secret_key\": \"wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY\"}"
      }
    }
  }'
```

**Response** includes the `reference_id` for linking:
```json
{
  "data": {
    "type": "credential",
    "id": "019bec12-3456-7890-abcd-ef1234567890",
    "attributes": {
      "name": "aws-s3-access",
      "reference_id": "019bec12-3456-7890-abcd-ef1234567890"
    }
  }
}
```

**Important**: The `content` field is NOT returned in responses for security.

---

## Link Credential to Cloud Store

Credentials are used with cloud storage for authenticated access:

```bash
# Create cloud store with credential reference
curl -X POST http://localhost:6336/api/cloud_store \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "cloud_store",
      "attributes": {
        "name": "s3-bucket",
        "store_type": "s3",
        "root_path": "my-bucket/data"
      },
      "relationships": {
        "credential_id": {
          "data": {"type": "credential", "id": "CREDENTIAL_REF_ID"}
        }
      }
    }
  }'
```

See [Cloud Storage](Cloud-Storage.md) for complete setup.

---

## Credential Content Formats

### AWS S3

```json
{
  "access_key": "AKIAIOSFODNN7EXAMPLE",
  "secret_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
  "region": "us-east-1"
}
```

### Google Cloud Storage

```json
{
  "type": "service_account",
  "project_id": "my-project",
  "private_key_id": "key-id",
  "private_key": "-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----\n",
  "client_email": "service@my-project.iam.gserviceaccount.com",
  "client_id": "123456789",
  "auth_uri": "https://accounts.google.com/o/oauth2/auth",
  "token_uri": "https://oauth2.googleapis.com/token"
}
```

### Azure Blob Storage

```json
{
  "account_name": "mystorageaccount",
  "account_key": "base64-encoded-key"
}
```

### FTP/SFTP

```json
{
  "host": "ftp.example.com",
  "port": 21,
  "username": "ftpuser",
  "password": "ftppassword"
}
```

### SSH Key

```json
{
  "private_key": "-----BEGIN RSA PRIVATE KEY-----\n...\n-----END RSA PRIVATE KEY-----",
  "passphrase": "optional-passphrase"
}
```

---

## List Credentials

```bash
curl http://localhost:6336/api/credential \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

**Note**: Only `name` and metadata are returned. The `content` field is never exposed.

---

## Update Credential

```bash
curl -X PATCH http://localhost:6336/api/credential/CREDENTIAL_ID \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "credential",
      "id": "CREDENTIAL_ID",
      "attributes": {
        "content": "{\"access_key\": \"NEW_KEY\", \"secret_key\": \"NEW_SECRET\"}"
      }
    }
  }'
```

---

## Delete Credential

```bash
curl -X DELETE http://localhost:6336/api/credential/CREDENTIAL_ID \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

**Warning**: Deleting a credential will break any cloud_store or other entity using it.

---

## Security

### Encryption

- Content is encrypted at rest using AES encryption
- Encryption key is derived from `encryption.secret` config
- Never logged or exposed in API responses

### Best Practices

1. **Use unique credentials** - Don't share credentials across services
2. **Rotate regularly** - Update credentials periodically
3. **Principle of least privilege** - Use credentials with minimal required permissions
4. **Audit access** - Only administrators can access credential table
5. **Backup encryption key** - Store `encryption.secret` securely

---

## Relationships

| Entity | Relationship | FK Column |
|--------|--------------|-----------|
| `cloud_store` | has_one | `credential_id` |

---

## Troubleshooting

### "content" Not Returned

This is by design. The encrypted content is never returned in API responses for security.

### Cloud Store Authentication Fails

1. Verify credential exists and is linked
2. Check credential content format matches provider requirements
3. Verify provider credentials are still valid

### Permission Denied

Only administrators can access the credential table. Ensure your user is in the `administrators` group.

---

## See Also

- [Cloud Storage](Cloud-Storage.md) - Using credentials with storage
- [Integrations](Integrations.md) - OAuth authentication for APIs
- [Encryption](Encryption.md) - Encryption configuration
