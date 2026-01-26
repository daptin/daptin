# TLS Certificates

**Tested ✓ 2026-01-26**

Secure your Daptin instance with TLS/SSL certificates. Supports both self-signed certificates (development) and ACME/Let's Encrypt certificates (production).

## Quick Start (5 minutes)

Generate a self-signed certificate for development:

```bash
# Get authentication token
TOKEN=$(cat /tmp/daptin-token.txt)

# Step 1: Create certificate record
curl -X POST http://localhost:6336/api/certificate \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "certificate",
      "attributes": {
        "hostname": "localhost"
      }
    }
  }'

# Step 2: Get certificate ID
CERT_ID=$(curl -s -H "Authorization: Bearer $TOKEN" http://localhost:6336/api/certificate | jq -r '.data[0].id')

# Step 3: Generate self-signed certificate
curl -X POST http://localhost:6336/action/certificate/generate_self_certificate \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"certificate_id\": \"$CERT_ID\"}"
```

**Expected Response:**
```json
[{
  "ResponseType": "client.notify",
  "Attributes": {
    "message": "Certificate generated for localhost",
    "title": "Success",
    "type": "message"
  }
}]
```

**Note:** Self-signed certificates will show browser warnings in production. Use ACME for production deployments.

---

## Core Concepts

### Certificate Types

**Self-Signed Certificates**
- Generated locally by Daptin
- Perfect for development and testing
- No external dependencies
- Browser will show security warnings
- No cost

**ACME Certificates (Let's Encrypt)**
- Industry-standard trusted certificates
- No browser warnings
- Free from Let's Encrypt
- Requires:
  - Real domain with public DNS
  - Port 80 accessible from internet
  - Valid email address in database
- Auto-renewal supported

### How It Works

1. **Certificate Record**: Create a record in the `certificate` table with hostname
2. **Generation**: Call action to generate certificate (self-signed or ACME)
3. **Storage**: Certificate stored encrypted in database
4. **SNI Support**: Multiple certificates for different hostnames
5. **HTTPS Server**: Daptin serves certificates based on requested hostname

### Certificate Table Schema

| Column | Type | Description |
|--------|------|-------------|
| hostname | varchar(100) | Domain name (e.g., "api.example.com") |
| issuer | varchar(100) | "self" or "acme" |
| generated_at | timestamp | Certificate generation time |
| certificate_pem | text | Public certificate (PEM format) |
| private_key_pem | text | Private key (PEM format) |
| public_key_pem | text | Public key (PEM format) |
| root_certificate | text | Root CA certificate (ACME only) |

---

## Self-Signed Certificates (Development)

### Complete Workflow

**Step 1: Create Certificate Record**
```bash
TOKEN=$(cat /tmp/daptin-token.txt)

curl -X POST http://localhost:6336/api/certificate \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "certificate",
      "attributes": {
        "hostname": "localhost"
      }
    }
  }'
```

**Response:**
```json
{
  "data": {
    "type": "certificate",
    "id": "019bf97f-ce74-7534-93fa-13cb6039efba",
    "attributes": {
      "hostname": "localhost",
      "issuer": null,
      "generated_at": null,
      "certificate_pem": null,
      "private_key_pem": null,
      "public_key_pem": null
    }
  }
}
```

**Step 2: Generate Self-Signed Certificate**
```bash
CERT_ID="019bf97f-ce74-7534-93fa-13cb6039efba"  # Use ID from Step 1

curl -X POST http://localhost:6336/action/certificate/generate_self_certificate \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"certificate_id\": \"$CERT_ID\"}"
```

**Success Response:**
```json
[{
  "ResponseType": "client.notify",
  "Attributes": {
    "message": "Certificate generated for localhost",
    "title": "Success",
    "type": "message"
  }
}]
```

**Step 3: Verify Certificate**
```bash
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/certificate/$CERT_ID" | \
  jq '.data.attributes | {hostname, issuer, generated_at}'
```

**Response:**
```json
{
  "hostname": "localhost",
  "issuer": "self",
  "generated_at": "2026-01-26T14:23:30+05:30"
}
```

### Multiple Domains (SNI)

Create certificates for multiple hostnames:

```bash
TOKEN=$(cat /tmp/daptin-token.txt)

# Create certificate records for multiple domains
for hostname in api.example.com web.example.com admin.example.com; do
  curl -s -X POST http://localhost:6336/api/certificate \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/vnd.api+json" \
    -d "{\"data\":{\"type\":\"certificate\",\"attributes\":{\"hostname\":\"$hostname\"}}}"
  sleep 1
done

# Generate certificates for all
curl -s -H "Authorization: Bearer $TOKEN" "http://localhost:6336/api/certificate" | \
  jq -r '.data[].id' | \
  while read CERT_ID; do
    curl -s -X POST http://localhost:6336/action/certificate/generate_self_certificate \
      -H "Authorization: Bearer $TOKEN" \
      -H "Content-Type: application/json" \
      -d "{\"certificate_id\": \"$CERT_ID\"}"
  done
```

### Regenerating Certificates

Calling `generate_self_certificate` on an existing certificate will regenerate it:

```bash
# Regenerate certificate (same certificate_id)
curl -X POST http://localhost:6336/action/certificate/generate_self_certificate \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"certificate_id\": \"$CERT_ID\"}"
```

**Note:** The action succeeds but may not update the timestamp if the certificate is already valid.

---

## ACME Certificates (Production)

### Prerequisites

Before generating ACME certificates:

1. **Real Domain**: Must own domain with public DNS (e.g., "api.example.com")
2. **DNS Configuration**: Domain must resolve to your Daptin server's public IP
3. **Port 80 Access**: Let's Encrypt requires port 80 accessible from internet for HTTP-01 challenge
4. **Valid Email**: User email must exist in `user_account` table (used for Let's Encrypt notifications)

### Complete Workflow

**Step 1: Verify Prerequisites**
```bash
# Check email exists
TOKEN=$(cat /tmp/daptin-token.txt)
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/user_account?filter=email||eq||admin@example.com" | \
  jq '.data[].attributes.email'

# Verify DNS resolution (from external machine)
nslookup api.example.com

# Verify port 80 accessible (from external machine)
curl -I http://api.example.com
```

**Step 2: Create Certificate Record**
```bash
curl -X POST http://localhost:6336/api/certificate \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "certificate",
      "attributes": {
        "hostname": "api.example.com"
      }
    }
  }'
```

**Step 3: Generate ACME Certificate**
```bash
CERT_ID="<certificate-id-from-step-2>"

curl -X POST http://localhost:6336/action/certificate/generate_acme_certificate \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"certificate_id\": \"$CERT_ID\",
    \"attributes\": {
      \"email\": \"admin@example.com\"
    }
  }"
```

**Step 4: Monitor Challenge**

During ACME generation, Daptin:
1. Contacts Let's Encrypt production API
2. Creates HTTP-01 challenge endpoint: `http://api.example.com/.well-known/acme-challenge/{token}`
3. Let's Encrypt verifies ownership by fetching the challenge
4. Certificate issued and stored in database

This process takes 30-60 seconds.

**Success Response:**
```json
[{
  "ResponseType": "client.notify",
  "Attributes": {
    "message": "Certificate generated for api.example.com",
    "title": "Success",
    "type": "message"
  }
}]
```

**Step 5: Verify Certificate**
```bash
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/certificate/$CERT_ID" | \
  jq '.data.attributes | {hostname, issuer, generated_at, root_certificate: (.root_certificate[:50])}'
```

**Response:**
```json
{
  "hostname": "api.example.com",
  "issuer": "acme",
  "generated_at": "2026-01-26T14:30:00+05:30",
  "root_certificate": "-----BEGIN CERTIFICATE-----\nMIIEkjCCA3qgAwIB..."
}
```

### ACME Technical Details

**Let's Encrypt API:**
- Production API: https://acme-v02.api.letsencrypt.org
- Certificate validity: 90 days
- Rate limits: 50 certificates per domain per week

**Private Key Storage:**
- ACME account private key stored in `_config` table
- Key name: `letsencrypt-user-private-key-{email-hash}`
- Encrypted with `encryption.secret`
- RSA 2048-bit key

**Challenge Endpoint:**
- Path: `/.well-known/acme-challenge/{token}`
- Must be accessible on port 80
- Temporary (active only during verification)

---

## Downloading Certificates

### Download Certificate PEM

```bash
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/action/certificate/{cert_id}/download_certificate" \
  -o certificate.pem
```

### Download Public Key PEM

```bash
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/action/certificate/{cert_id}/download_public_key" \
  -o public_key.pem
```

### Download via API

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/certificate/$CERT_ID" | \
  jq -r '.data.attributes.certificate_pem' > certificate.pem

curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/certificate/$CERT_ID" | \
  jq -r '.data.attributes.private_key_pem' > private_key.pem
```

---

## Configuration

### Server Configuration

Certificates are automatically loaded from the database at server startup. To use new certificates:

**Option 1: Restart Daptin**
```bash
curl -X POST http://localhost:6336/action/world/restart_daptin \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"attributes":{}}'
```

**Option 2: Manual Restart**
```bash
./scripts/testing/test-runner.sh stop
./scripts/testing/test-runner.sh start
```

### HTTPS Configuration

Once certificates are generated, Daptin automatically serves HTTPS on the configured port (default: 6443).

**Test HTTPS:**
```bash
# Self-signed (will show warning)
curl -k https://localhost:6443/api/world

# ACME (trusted)
curl https://api.example.com:6443/api/world
```

---

## Error Handling

### Common Errors

**Error: "email or mobile missing"**
```json
{
  "ResponseType": "client.notify",
  "Attributes": {
    "message": "email or mobile missing",
    "title": "failed",
    "type": "error"
  }
}
```
**Cause:** ACME certificate requires `email` parameter
**Solution:** Add email to request:
```bash
curl -X POST http://localhost:6336/action/certificate/generate_acme_certificate \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"certificate_id\": \"$CERT_ID\", \"attributes\": {\"email\": \"admin@example.com\"}}"
```

---

**Error: "invalid email"**
```json
{
  "ResponseType": "client.notify",
  "Attributes": {
    "message": "invalid email",
    "title": "failed",
    "type": "error"
  }
}
```
**Cause:** Email does not exist in `user_account` table
**Solution:** Use email of existing user:
```bash
# Check existing users
curl -s -H "Authorization: Bearer $TOKEN" "http://localhost:6336/api/user_account" | jq '.data[].attributes.email'
```

---

**Error: "Domain name does not end with a valid public suffix (TLD)"**
```json
{
  "ResponseType": "client.notify",
  "Attributes": {
    "message": "acme: error: 400 :: Cannot issue for \"api.local\": Domain name does not end with a valid public suffix (TLD)",
    "title": "failed",
    "type": "error"
  }
}
```
**Cause:** Let's Encrypt requires real public domains (not .local, .test, localhost)
**Solution:**
- Use real domain with valid TLD (.com, .net, .org, etc.)
- For development, use self-signed certificates instead

---

**Error: "Cannot communicate with Let's Encrypt"**
**Cause:** Port 80 not accessible from internet
**Solution:**
1. Check firewall allows port 80
2. Verify DNS points to correct server
3. Test from external machine:
```bash
curl http://your-domain.com/.well-known/acme-challenge/test
```

---

### Debugging

**Check Certificate Status:**
```bash
sqlite3 daptin.db "SELECT hostname, issuer, length(certificate_pem), generated_at FROM certificate;"
```

**View Certificate Details:**
```bash
# Get certificate from database
TOKEN=$(cat /tmp/daptin-token.txt)
curl -s -H "Authorization: Bearer $TOKEN" "http://localhost:6336/api/certificate" | \
  jq -r '.data[0].attributes.certificate_pem' > cert.pem

# View certificate info
openssl x509 -in cert.pem -text -noout
```

**Check ACME Private Key:**
```bash
sqlite3 daptin.db "SELECT name FROM _config WHERE name LIKE 'letsencrypt%';"
```

**Monitor Server Logs:**
```bash
./scripts/testing/test-runner.sh logs | grep -i "certificate\|acme\|tls"
```

---

## Security Considerations

### Self-Signed Certificates
- **Development Only**: Do not use in production
- **Browser Warnings**: Users will see security warnings
- **No External Validation**: No verification of domain ownership
- **Trust Management**: Must manually add to trusted certificates

### ACME Certificates
- **Production Ready**: Industry-standard trusted certificates
- **Auto-Renewal**: Implement renewal before 90-day expiry
- **Rate Limits**: Max 50 certs per domain per week
- **Domain Validation**: Requires actual domain control
- **Private Key Security**: Keys stored encrypted in database

### Private Key Storage
- All private keys stored in database (encrypted)
- Encryption key: `encryption.secret` from `_config` table
- Algorithm: AES-256
- Backup database includes private keys - secure backups appropriately

### SNI (Server Name Indication)
- Daptin serves different certificates based on requested hostname
- Requires client SNI support (all modern browsers)
- Default certificate served if hostname not found

---

## Production Deployment Checklist

For production HTTPS deployment:

- [ ] Real domain with public DNS configured
- [ ] Port 80 open and accessible from internet (ACME validation)
- [ ] Port 443 (or custom HTTPS port) open for secure traffic
- [ ] Valid email in `user_account` table
- [ ] Certificate record created with production hostname
- [ ] ACME certificate generated successfully
- [ ] Certificate expiry monitoring configured (90-day validity)
- [ ] Auto-renewal implemented (recommended: 30 days before expiry)
- [ ] Backup database secured (contains private keys)
- [ ] HTTPS redirect configured (optional)
- [ ] HTTP Strict Transport Security (HSTS) configured (optional)

---

## Certificate Renewal

ACME certificates expire after 90 days. Implement renewal:

**Manual Renewal:**
```bash
# Regenerate ACME certificate
TOKEN=$(cat /tmp/daptin-token.txt)
CERT_ID="<your-cert-id>"

curl -X POST http://localhost:6336/action/certificate/generate_acme_certificate \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"certificate_id\": \"$CERT_ID\",
    \"attributes\": {
      \"email\": \"admin@example.com\"
    }
  }"

# Restart to load new certificate
curl -X POST http://localhost:6336/action/world/restart_daptin \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"attributes":{}}'
```

**Automated Renewal (Recommended):**

Create scheduled task or cron job:
```bash
#!/bin/bash
# Run daily, renews if < 30 days remaining

TOKEN=$(curl -s -X POST http://localhost:6336/action/user_account/signin \
  -H "Content-Type: application/json" \
  -d '{"attributes":{"email":"admin@example.com","password":"your-password"}}' | \
  jq -r '.[] | select(.ResponseType == "client.store.set") | .Attributes.value')

# Get certificates expiring soon
# Check generated_at timestamp + 90 days

# Renew each certificate
curl -X POST http://localhost:6336/action/certificate/generate_acme_certificate \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"certificate_id\": \"$CERT_ID\", \"attributes\": {\"email\": \"admin@example.com\"}}"
```

---

## Troubleshooting

### Certificate Not Loading

**Symptom:** Generated certificate but HTTPS not working

**Solution:**
1. Verify certificate in database:
```bash
sqlite3 daptin.db "SELECT hostname, length(certificate_pem) FROM certificate;"
```

2. Restart Daptin to load certificates:
```bash
./scripts/testing/test-runner.sh stop && ./scripts/testing/test-runner.sh start
```

3. Check HTTPS port is accessible:
```bash
curl -k https://localhost:6443/api/world
```

### ACME Challenge Fails

**Symptom:** Let's Encrypt cannot verify domain

**Diagnostic Steps:**
1. Verify DNS resolution:
```bash
nslookup api.example.com
dig api.example.com
```

2. Test port 80 from external machine:
```bash
curl http://api.example.com/.well-known/acme-challenge/test
```

3. Check firewall rules:
```bash
# Allow port 80
sudo ufw allow 80/tcp
sudo iptables -A INPUT -p tcp --dport 80 -j ACCEPT
```

4. Verify domain points to correct IP:
```bash
curl ifconfig.me  # Your server's public IP
```

### Multiple Certificates Conflict

**Symptom:** Wrong certificate served for hostname

**Solution:** Daptin uses SNI - ensure client supports SNI (all modern browsers do):
```bash
# Test with SNI
curl --resolve api.example.com:443:127.0.0.1 https://api.example.com

# Check certificate served
openssl s_client -connect api.example.com:443 -servername api.example.com < /dev/null
```

### Cannot Delete Certificate

**Symptom:** Want to remove old certificate

**Solution:**
```bash
TOKEN=$(cat /tmp/daptin-token.txt)
CERT_ID="<certificate-id>"

curl -X DELETE "http://localhost:6336/api/certificate/$CERT_ID" \
  -H "Authorization: Bearer $TOKEN"
```

**Note:** Deleting a certificate does not immediately stop HTTPS server from using cached version. Restart server after deletion.

---

## Reference Links

- [Let's Encrypt Documentation](https://letsencrypt.org/docs/)
- [ACME Protocol RFC 8555](https://tools.ietf.org/html/rfc8555)
- [OpenSSL Certificate Commands](https://www.openssl.org/docs/man1.1.1/man1/openssl-x509.html)
- [SNI Support](https://en.wikipedia.org/wiki/Server_Name_Indication)
- [TLS Best Practices](https://wiki.mozilla.org/Security/Server_Side_TLS)

---

## Related Documentation

- [Authentication](Authentication.md) - User authentication with JWT
- [OAuth Authentication](OAuth-Authentication.md) - Social login setup
- [Production Deployment](Production-Deployment.md) - Production best practices
- [Common Errors](Common-Errors.md) - Troubleshooting guide
- [Action Reference](Action-Reference.md) - All available actions
