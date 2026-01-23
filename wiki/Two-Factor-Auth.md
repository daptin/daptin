# Two-Factor Authentication

TOTP-based two-factor authentication.

## Overview

Daptin supports TOTP (Time-based One-Time Password) for 2FA using apps like:
- Google Authenticator
- Authy
- 1Password
- Microsoft Authenticator

## Enable 2FA for User

### Step 1: Generate OTP Secret

```bash
curl -X POST http://localhost:6336/action/user_account/generate_otp \
  -H "Authorization: Bearer $TOKEN"
```

**Response:**
```json
{
  "secret": "JBSWY3DPEHPK3PXP",
  "qr_code": "data:image/png;base64,..."
}
```

### Step 2: Verify OTP

After user scans QR code:

```bash
curl -X POST http://localhost:6336/action/user_account/verify_otp \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "attributes": {
      "otp": "123456"
    }
  }'
```

## Sign In with 2FA

When 2FA is enabled, sign-in requires OTP:

```bash
curl -X POST http://localhost:6336/action/user_account/signin \
  -d '{
    "attributes": {
      "email": "user@example.com",
      "password": "password123",
      "otp": "123456"
    }
  }'
```

## Disable 2FA

```bash
curl -X POST http://localhost:6336/action/user_account/disable_otp \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "attributes": {
      "otp": "123456"
    }
  }'
```

## Check 2FA Status

```bash
curl http://localhost:6336/api/user_account/USER_ID?fields=otp_enabled \
  -H "Authorization: Bearer $TOKEN"
```

## Admin: View 2FA Status

Admins can check if users have 2FA enabled:

```bash
curl http://localhost:6336/api/user_account?fields=email,otp_enabled \
  -H "Authorization: Bearer $TOKEN"
```

## Force 2FA

To require 2FA for all users, add validation:

```yaml
Tables:
  - TableName: user_account
    Validations:
      - ColumnName: otp_secret
        Tags: required
```

## Recovery

If user loses authenticator access:

### Admin Reset

```bash
curl -X PATCH http://localhost:6336/api/user_account/USER_ID \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "user_account",
      "id": "USER_ID",
      "attributes": {
        "otp_secret": null
      }
    }
  }'
```

### Recovery Codes (Manual)

Store backup codes during setup and verify manually.

## TOTP Parameters

| Parameter | Value |
|-----------|-------|
| Algorithm | SHA1 |
| Digits | 6 |
| Period | 30 seconds |
| Issuer | Daptin |

## Security Best Practices

1. **Always verify OTP** - Don't skip verification step
2. **Backup codes** - Provide recovery options
3. **Rate limiting** - Prevent brute force on OTP
4. **Secure secret** - OTP secret column should be protected

## Client Implementation

### JavaScript (speakeasy)

```javascript
const speakeasy = require('speakeasy');

const verified = speakeasy.totp.verify({
  secret: 'JBSWY3DPEHPK3PXP',
  encoding: 'base32',
  token: '123456'
});
```

### Python (pyotp)

```python
import pyotp

totp = pyotp.TOTP('JBSWY3DPEHPK3PXP')
verified = totp.verify('123456')
```
