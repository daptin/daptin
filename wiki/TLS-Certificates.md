# TLS Certificates

HTTPS and certificate management.

## Certificate Actions

| Action | Description |
|--------|-------------|
| `generate_self_tls_certificate` | Self-signed certificate |
| `generate_acme_tls_certificate` | Let's Encrypt certificate |

## Self-Signed Certificate

For development or internal use:

```bash
curl -X POST http://localhost:6336/action/world/generate_self_tls_certificate \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "attributes": {
      "hostname": "example.com"
    }
  }'
```

## Let's Encrypt (ACME)

For production with valid certificates:

```bash
curl -X POST http://localhost:6336/action/world/generate_acme_tls_certificate \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "attributes": {
      "hostname": "example.com",
      "email": "admin@example.com"
    }
  }'
```

### Requirements

- Domain must resolve to server
- Port 80 must be accessible for challenge
- Valid email for notifications

## Certificate Storage

Certificates stored in `certificate` table:

```bash
curl http://localhost:6336/api/certificate \
  -H "Authorization: Bearer $TOKEN"
```

### Certificate Fields

| Field | Description |
|-------|-------------|
| hostname | Domain name |
| certificate_pem | Public certificate |
| private_key_pem | Private key |
| issuer | Certificate authority |
| valid_from | Start date |
| valid_until | Expiration date |

## Enable HTTPS

### Via Environment

```bash
DAPTIN_TLS_CERT_PATH=/path/to/cert.pem \
DAPTIN_TLS_KEY_PATH=/path/to/key.pem \
DAPTIN_PORT=443 \
./daptin
```

### Via Config

```bash
curl -X POST http://localhost:6336/_config/backend/https.port \
  -H "Authorization: Bearer $TOKEN" \
  -d '"443"'
```

## Multi-Domain (SNI)

Store multiple certificates for different domains. Daptin uses SNI to serve the correct certificate.

## Certificate Renewal

### Manual Renewal

```bash
curl -X POST http://localhost:6336/action/world/generate_acme_tls_certificate \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "attributes": {
      "hostname": "example.com",
      "email": "admin@example.com"
    }
  }'
```

### Automated Renewal

Set up a scheduled task:

```bash
curl -X POST http://localhost:6336/api/task \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "task",
      "attributes": {
        "name": "Renew TLS Cert",
        "action_name": "generate_acme_tls_certificate",
        "entity_name": "world",
        "schedule": "0 0 1 * *",
        "attributes": "{\"hostname\":\"example.com\",\"email\":\"admin@example.com\"}"
      }
    }
  }'
```

## View Certificate Info

```bash
curl 'http://localhost:6336/api/certificate?query=[{"column":"hostname","operator":"is","value":"example.com"}]' \
  -H "Authorization: Bearer $TOKEN"
```

## Delete Certificate

```bash
curl -X DELETE http://localhost:6336/api/certificate/CERT_ID \
  -H "Authorization: Bearer $TOKEN"
```

## Nginx Reverse Proxy

If using Nginx for TLS termination:

```nginx
server {
    listen 443 ssl;
    server_name example.com;

    ssl_certificate /etc/letsencrypt/live/example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/example.com/privkey.pem;

    location / {
        proxy_pass http://localhost:6336;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

## Troubleshooting

### Challenge Failed

1. Ensure port 80 is accessible
2. Check DNS resolution
3. Verify firewall rules

### Certificate Not Trusted

1. Use Let's Encrypt for public trust
2. Import self-signed cert to trust store

### Expired Certificate

1. Check certificate validity dates
2. Set up automated renewal
