# OTP Authentication and Account Recovery

Daptin uses time-based one-time passwords for authenticated OTP enrollment and for account recovery. The implementation is designed so an anonymous caller cannot enroll an OTP profile or use the passwordless-login actions.

## Security Properties

| Property | Current behavior |
|---|---|
| Code size | 6 decimal digits |
| TOTP period | 120 seconds |
| Accepted clock skew | None; only the current period is accepted |
| Enrollment | Must be initiated and verified by the authenticated account owner |
| Signup | Does not create an OTP profile |
| Login/reset prerequisite | Existing profile with `verified=true` |
| Account attempt allowance | 5 attempts per 15 minutes |
| Source attempt allowance | 50 attempts per 15 minutes |
| Distributed state | Atomic Olric counters shared by the cluster |
| Replay protection | One successful use per account and TOTP period |
| Cache failure | Verification fails closed |
| Password-reset result | Does not issue a session JWT |

The account counter is the primary defense against distributed-source attacks. Changing IP addresses does not give an attacker more attempts against one account.

## Permissions

The built-in `send_otp`, `verify_otp`, and `verify_mobile_number` actions require the `AuthenticatedExecute` permission. They do not grant `GuestExecute`.

The performers also verify that the authenticated user owns the target account for enrollment, sending, and authenticated verification. This ownership check is intentional defense in depth and is not replaced by action-row permission changes.

The `reset-password` and `reset-password-verify` actions remain reachable for account recovery, but they operate only on an already enrolled and verified OTP profile and are protected by the distributed attempt and replay controls.

## Shared Olric State

OTP verification uses Daptin's shared Olric cache. Every Daptin node participating in the same deployment must use the same Olric cluster.

The following logical keys are maintained:

```text
otp-attempt:account:<internal-account-id>
otp-attempt:source:<sha256-source-address>
otp-used:account:<internal-account-id>:counter:<totp-counter>
```

Attempt admission uses atomic increments. At most five concurrent attempts are admitted for an account during the 15-minute window, even when the requests reach different nodes or originate from different addresses.

If Olric cannot initialize or increment the required state, OTP verification returns an error. It does not continue without brute-force protection.

## Enroll OTP

Enrollment is an authenticated, instance-bound action. Supply the current user's account reference ID in `user_account_id`.

```bash
curl -X POST http://localhost:6336/action/user_account/register_otp \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "user_account_id": "USER_ACCOUNT_REFERENCE_ID",
      "mobile_number": "15550000000"
    }
  }'
```

Daptin rejects the operation if the authenticated session does not own the target account. If no profile exists, this operation creates one with `verified=false` and a new encrypted 20-byte TOTP secret.

The generated code must be delivered through a trusted channel configured by the application. Do not expose the OTP performer response directly to an untrusted client.

## Verify Enrollment

Verify the current account's code while authenticated:

```bash
curl -X POST http://localhost:6336/action/user_otp_account/verify_mobile_number \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "email": "user@example.com",
      "mobile_number": "15550000000",
      "otp": "123456"
    }
  }'
```

Successful owner verification changes the profile to `verified=true`. It does not need to mint a replacement session token.

## Send a Code for an Enrolled Account

`send_otp` requires authentication, account ownership, and an existing verified profile:

```bash
curl -X POST http://localhost:6336/action/user_otp_account/send_otp \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "email": "user@example.com",
      "mobile_number": "15550000000"
    }
  }'
```

It does not provision a profile for an arbitrary email address. Accounts must use the enrollment flow first.

## Authenticated OTP Verification

The legacy `verify_otp` action is no longer an anonymous passwordless-login endpoint. It requires an authenticated owner and a verified profile:

```bash
curl -X POST http://localhost:6336/action/user_account/verify_otp \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "email": "user@example.com",
      "otp": "123456"
    }
  }'
```

Applications should normally use password or OAuth authentication to obtain the initial session and treat OTP as an enrolled additional verification mechanism.

## Password Recovery

Start recovery using the normal action:

```bash
curl -X POST http://localhost:6336/action/user_account/reset-password \
  -H "Content-Type: application/json" \
  -d '{"attributes":{"email":"user@example.com"}}'
```

Recovery fails if the account has no verified OTP profile. The generated six-digit code is sent through the configured recovery mail flow.

Submit the code:

```bash
curl -X POST http://localhost:6336/action/user_account/reset-password-verify \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "email": "user@example.com",
      "otp": "123456"
    }
  }'
```

Verification consumes one distributed attempt. A successful code is marked used for its account and TOTP period before the password-reset workflow continues. The verification performer does not return a victim session JWT.

## Lockout and Retry Behavior

An account receives five admitted verification attempts per 15-minute window. Further attempts return an error until the distributed key expires. A source address can make at most 50 admitted attempts across accounts in the same duration.

A successful verification clears the account and source attempt counters, but the successful TOTP counter remains marked as used, preventing replay of the same code.

Clients should not automatically retry an OTP after a protection or replay error. Ask the user to wait for the lockout window or request a code in a later TOTP period as appropriate.

## Cluster Checklist

1. Run Olric on every Daptin node using the deployment's shared cluster configuration.
2. Confirm nodes discover one another before exposing recovery endpoints.
3. Ensure reverse proxies preserve a meaningful source address. The per-account counter remains effective even when all requests appear to come from one proxy.
4. Deliver codes only through a trusted, account-controlled channel.
5. Monitor messages about unavailable OTP protection and excessive attempts.
6. Keep the explicit built-in action permissions intact during schema synchronization.

## Migration Notes

- Existing encrypted TOTP secrets remain usable; TOTP secrets do not encode digit count or period.
- Clients and delivery integrations must switch from four-digit to six-digit codes.
- Codes generated under the former five-minute settings are not accepted after upgrading.
- Users who never completed enrollment have `verified=false` and cannot use login or recovery until they enroll as the authenticated account owner.
- Signup no longer creates an OTP profile from an optional mobile-number field.

## Related Documentation

- [[Authentication]]
- [[Rate-Limiting]]
- [[API-Metering]]
