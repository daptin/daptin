# Two-Factor Authentication

TOTP-based two-factor authentication for OTP-based login.

## Overview

Daptin supports TOTP (Time-based One-Time Password) for OTP-based authentication. This is a **separate authentication flow** from password-based signin.

**Note:** The current implementation generates OTP codes but does not return them to the client. The system is designed to send OTP via SMS (integration currently disabled). For development/testing, you may need to check the database directly for the OTP secret.

## TOTP Parameters

| Parameter | Value |
|-----------|-------|
| Algorithm | SHA1 |
| Digits | 4 |
| Period | 300 seconds (5 minutes) |
| Issuer | site.daptin.com |
| Secret Size | 10 bytes |

## Enable OTP for User

### Step 1: Register OTP

This creates a `user_otp_account` record with the OTP secret.

```bash
# Get user reference ID first
USER_REF=$(curl -s http://localhost:6336/api/user_account \
  -H "Authorization: Bearer $TOKEN" | jq -r '.data[0].id')

# Register OTP (requires user_account_id in body)
curl -X POST http://localhost:6336/action/user_account/register_otp \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"attributes\": {
      \"mobile_number\": \"1234567890\",
      \"user_account_id\": \"$USER_REF\"
    }
  }"
```

**Response:** Empty array `[]` (OTP secret is stored internally, not returned)

**Important:** This action requires `InstanceOptional: false`, meaning you must provide the `user_account_id` in the request body.

### Step 2: Send OTP (Optional)

Generate and send a new OTP code:

```bash
curl -X POST http://localhost:6336/action/user_otp_account/send_otp \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "mobile_number": "1234567890",
      "email": "user@example.com"
    }
  }'
```

**Note:** The OTP code is generated but SMS delivery is currently disabled. In production, configure an SMS provider integration.

## Sign In with OTP

Use `verify_otp` to authenticate with an OTP code and receive a JWT token:

```bash
curl -X POST http://localhost:6336/action/user_account/verify_otp \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "otp": "1234",
      "email": "user@example.com"
    }
  }'
```

**Response:**
```json
[
  {
    "ResponseType": "client.store.set",
    "Attributes": {
      "key": "token",
      "value": "eyJhbGciOiJIUzI1NiIs..."
    }
  }
]
```

**Note:** You can use either `email` or `mobile_number` to identify the user.

## OTP Actions Reference

| Action | OnType | InstanceOptional | Description |
|--------|--------|------------------|-------------|
| `register_otp` | user_account | false | Create OTP profile with secret |
| `send_otp` | user_otp_account | true | Generate new OTP code |
| `verify_otp` | user_account | true | Verify OTP and get JWT |
| `verify_mobile_number` | user_otp_account | true | Verify OTP and get JWT |

## Check OTP Status

```bash
curl http://localhost:6336/api/user_otp_account \
  -H "Authorization: Bearer $TOKEN"
```

**Response includes:**
- `mobile_number`: Registered phone number
- `verified`: 0 (unverified) or 1 (verified)

## OTP Tables

| Table | Purpose |
|-------|---------|
| user_otp_account | Stores OTP secrets and verification status |

## Security Best Practices

1. **Rate limiting** - Prevent brute force on OTP verification
2. **Short validity** - OTP codes expire after 5 minutes
3. **Secure storage** - OTP secrets are encrypted in database
4. **SMS delivery** - Configure a trusted SMS provider for production

## Development/Testing

For testing without SMS delivery, you can generate OTP codes using the stored secret:

### Python (pyotp)

```python
import pyotp

# Use the decrypted secret from database
totp = pyotp.TOTP('SECRET_HERE', digits=4, interval=300)
code = totp.now()
```

### JavaScript (otplib)

```javascript
const { totp } = require('otplib');

totp.options = { digits: 4, step: 300 };
const code = totp.generate('SECRET_HERE');
```

## Known Limitations

1. OTP codes are not returned to the client (designed for SMS delivery)
2. No built-in `disable_otp` action - reset OTP by clearing `user_otp_account`
3. Standard signin does not require OTP - use `verify_otp` for OTP-based login
