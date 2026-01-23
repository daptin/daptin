# Encryption

Data encryption at rest and in transit.

## Column Encryption

Encrypt sensitive data at rest using `encrypted` column type:

```yaml
Tables:
  - TableName: user_profile
    Columns:
      - Name: ssn
        DataType: varchar(255)
        ColumnType: encrypted

      - Name: credit_card
        DataType: varchar(255)
        ColumnType: encrypted
```

## How It Works

1. Data encrypted before storage
2. Decrypted when retrieved
3. Uses AES-256-GCM
4. Key derived from master secret

## Encryption Key

### Environment Variable

```bash
DAPTIN_ENCRYPTION_KEY="your-32-byte-encryption-key-here" ./daptin
```

### Generate Key

```bash
openssl rand -base64 32
```

## Encrypted Column Types

| Column Type | Use Case |
|-------------|----------|
| `encrypted` | General encrypted text |
| `bcrypt` | Password hashing (one-way) |

## Password Hashing

Use `bcrypt` for passwords (cannot be decrypted):

```yaml
Columns:
  - Name: password
    DataType: varchar(255)
    ColumnType: bcrypt
```

## API Behavior

### Write

Data automatically encrypted:

```bash
curl -X POST http://localhost:6336/api/user_profile \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "user_profile",
      "attributes": {
        "ssn": "123-45-6789"
      }
    }
  }'
```

### Read

Data automatically decrypted for authorized users:

```bash
curl http://localhost:6336/api/user_profile/ID \
  -H "Authorization: Bearer $TOKEN"
```

**Response:**
```json
{
  "data": {
    "type": "user_profile",
    "attributes": {
      "ssn": "123-45-6789"
    }
  }
}
```

## Database Storage

In database, encrypted data looks like:

```
AES256:iv:ciphertext:tag
```

## Key Rotation

To rotate encryption keys:

1. Export data with old key
2. Update `DAPTIN_ENCRYPTION_KEY`
3. Re-import data

## TLS in Transit

### Enable HTTPS

```bash
DAPTIN_TLS_CERT_PATH=/path/to/cert.pem \
DAPTIN_TLS_KEY_PATH=/path/to/key.pem \
./daptin
```

### Database TLS

MySQL:
```
DAPTIN_DB_CONNECTION_STRING="user:pass@tcp(host:3306)/db?tls=true"
```

PostgreSQL:
```
DAPTIN_DB_CONNECTION_STRING="host=db sslmode=require user=daptin password=pass dbname=daptin"
```

## Sensitive Data Best Practices

1. **Use encrypted columns** for PII, financial data
2. **Use bcrypt** for passwords
3. **Enable TLS** for all connections
4. **Rotate keys** periodically
5. **Audit access** to sensitive columns

## Encryption at Rest

### Full Database Encryption

Use database-level encryption:

**MySQL:**
```sql
ALTER TABLE tablename ENCRYPTION='Y';
```

**PostgreSQL:**
Use pgcrypto or Transparent Data Encryption (TDE).

### Disk Encryption

Use LUKS or similar for disk-level encryption.

## Audit Logging

Track access to encrypted data:

```yaml
Tables:
  - TableName: sensitive_data
    Columns:
      - Name: secret
        ColumnType: encrypted
    Audit: true
```

## Performance

Encryption adds overhead:
- ~5-10% for read/write
- Consider caching for frequently accessed data
