# Certificate Actions

TLS certificate management actions.

## Overview

Daptin provides actions for:
- Generating self-signed certificates
- Obtaining Let's Encrypt ACME certificates
- Downloading certificates and keys

## Certificate Table

Certificates are stored in the `certificate` table:

| Column | Type | Description |
|--------|------|-------------|
| hostname | label | Domain hostname |
| issuer | label | Certificate issuer |
| generated_at | datetime | Generation timestamp |
| certificate_pem | content | PEM-encoded certificate |
| private_key_pem | encrypted | PEM-encoded private key |
| public_key_pem | content | PEM-encoded public key |
| root_certificate | content | Root CA certificate |

## Create Certificate Entry

Before generating a certificate, create an entry:

```bash
curl -X POST http://localhost:6336/api/certificate \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "certificate",
      "attributes": {
        "hostname": "example.com"
      }
    }
  }'
```

## Generate Self-Signed Certificate

Create a self-signed certificate for development/testing.

**Action:** `generate_self_certificate`
**Entity:** `certificate`
**Performer:** `self.tls.generate`

```bash
curl -X POST http://localhost:6336/action/certificate/generate_self_certificate/CERTIFICATE_REFERENCE_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"attributes":{}}'
```

**Response:**

```json
{
  "ResponseType": "client.notify",
  "Attributes": {
    "type": "success",
    "title": "Success",
    "message": "Certificate generated for example.com"
  }
}
```

## Generate ACME Certificate

Obtain a Let's Encrypt certificate for production use.

**Action:** `generate_acme_certificate`
**Entity:** `certificate`
**Performer:** `acme.tls.generate`

### Prerequisites

1. Domain must resolve to Daptin server
2. Port 80 must be accessible for HTTP-01 challenge
3. Valid email address for ACME registration

### Request

```bash
curl -X POST http://localhost:6336/action/certificate/generate_acme_certificate/CERTIFICATE_REFERENCE_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "email": "admin@example.com"
    }
  }'
```

**Input Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| email | label | Yes | Contact email for Let's Encrypt |

### Process

1. Creates ACME client with user email
2. Registers with Let's Encrypt
3. Responds to HTTP-01 challenge at `/.well-known/acme-challenge/`
4. Obtains certificate
5. Stores certificate, private key, and root certificate

### ACME Challenge

Daptin automatically handles the HTTP-01 challenge:
- Challenge endpoint: `http://hostname/.well-known/acme-challenge/{token}`
- Port 80 must be accessible from the internet
- DNS must resolve to Daptin server

## Download Certificate

Download the certificate PEM file.

**Action:** `download_certificate`
**Entity:** `certificate`

```bash
curl -X POST http://localhost:6336/action/certificate/download_certificate/CERTIFICATE_REFERENCE_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"attributes":{}}'
```

**Response:** Downloads `{hostname}.pem.crt`

## Download Public Key

Download the public key PEM file.

**Action:** `download_public_key`
**Entity:** `certificate`

```bash
curl -X POST http://localhost:6336/action/certificate/download_public_key/CERTIFICATE_REFERENCE_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"attributes":{}}'
```

**Response:** Downloads `{hostname}.pem.key.pub`

## List Certificates

```bash
curl http://localhost:6336/api/certificate \
  -H "Authorization: Bearer $TOKEN"
```

## Get Certificate Details

```bash
curl http://localhost:6336/api/certificate/CERTIFICATE_REFERENCE_ID \
  -H "Authorization: Bearer $TOKEN"
```

## Certificate Usage

### HTTPS Configuration

Once generated, certificates are automatically used for HTTPS:

```bash
# Set hostname for HTTPS
curl -X POST 'http://localhost:6336/_config/backend/hostname' \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer $TOKEN' \
  -d '"example.com"'

# Enable HTTPS
curl -X POST 'http://localhost:6336/_config/backend/https.enabled' \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer $TOKEN' \
  -d 'true'
```

### Mail Server TLS

Certificates can be used for mail server TLS:

```bash
curl -X POST http://localhost:6336/api/mail_server \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "mail_server",
      "attributes": {
        "hostname": "mail.example.com",
        "always_on_tls": true
      }
    }
  }'
```

### DKIM Signing

Certificates provide keys for DKIM email signing.

## Certificate Renewal

### ACME Auto-Renewal

ACME certificates expire after 90 days. To renew:

```bash
curl -X POST http://localhost:6336/action/certificate/generate_acme_certificate/CERTIFICATE_REFERENCE_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "email": "admin@example.com"
    }
  }'
```

### Scheduled Renewal

Create a scheduled task for automatic renewal:

```yaml
Tasks:
  - Name: renew_certificates
    Label: Renew TLS Certificates
    Schedule: "0 0 1 * *"  # Monthly
    ActionName: generate_acme_certificate
    EntityName: certificate
    AsUserEmail: admin@example.com
    Attributes:
      email: admin@example.com
```

## Troubleshooting

### ACME Challenge Failed

1. Verify DNS resolves to server
2. Check port 80 is open
3. Verify hostname matches certificate entry
4. Check server logs for challenge details

### Certificate Not Working

1. Verify certificate exists in database
2. Check hostname matches request
3. Restart Daptin after generating certificate
4. Verify certificate not expired

### Self-Signed Certificate Warnings

Self-signed certificates trigger browser warnings. For production:
- Use ACME certificates
- Or import self-signed cert to trust store

## Security Notes

- Private keys are stored encrypted
- ACME registration uses production Let's Encrypt
- Self-signed certificates valid for development only
- Root certificates included for chain verification
