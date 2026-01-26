# First Admin Setup

**Prerequisites**: [[Installation]] - Daptin must be running on port 6336
**Related**: [[Getting-Started-Guide]] | [[Common-Errors]]

This guide covers the critical first 5 minutes with a fresh Daptin installation.

---

## Why This Matters

**On a fresh install, Daptin is WIDE OPEN** - anyone can do anything.

The first person to claim admin "locks the door" and becomes the system administrator. After that:
- Public signup is disabled
- Guest permissions are restricted
- Only admins can create new users

**You must do this immediately** or anyone else can claim admin first.

---

## Prerequisites Check

### 1. Kill Stale Processes (CRITICAL)

```bash
# Kill all old Daptin processes
pkill -9 -f daptin 2>/dev/null || true
pkill -9 -f "go run main" 2>/dev/null || true

# Free both ports
lsof -i :6336 -t | xargs kill -9 2>/dev/null || true  # HTTP API
lsof -i :5336 -t | xargs kill -9 2>/dev/null || true  # Olric cache (CRITICAL!)

# Verify ports are free
lsof -i :6336 || echo "✓ Port 6336 free"
lsof -i :5336 || echo "✓ Port 5336 free"

sleep 2
```

**Why port 5336 matters**: Olric distributed cache stores admin reference IDs. Stale cache causes "Unauthorized" errors even with fresh database.

**Related**: [[Common-Errors#unauthorized-on-become_an_administrator]]

### 2. Fresh Database (Optional)

```bash
# If you want to start completely fresh:
cd /path/to/daptin
rm -f daptin.db
```

### 3. Start Daptin

```bash
# Preferred method
./scripts/testing/test-runner.sh start

# OR manual
go run main.go > /tmp/daptin.log 2>&1 &
sleep 10
```

### 4. Verify Server Running

```bash
curl http://localhost:6336/ping
# Expected: pong
```

If you don't get "pong", check logs:
```bash
tail -20 /tmp/daptin.log
```

---

## Step 1: Create First User Account

**Password requirements**:
- Minimum 8 characters
- At least one uppercase letter
- At least one lowercase letter
- At least one number
- At least one special character

Example valid password: `Admin123!@#`

```bash
curl -s -X POST http://localhost:6336/action/user_account/signup \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "name": "Admin",
      "email": "admin@admin.com",
      "password": "adminadmin",
      "passwordConfirm": "adminadmin"
    }
  }'
```

**Expected response**:
```json
[
  {
    "ResponseType": "client.store.set",
    "Attributes": {
      "key": "token",
      "value": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
    }
  },
  {
    "ResponseType": "client.notify",
    "Attributes": {
      "message": "Signed up",
      "type": "success"
    }
  }
]
```

**If you get an error**:
- `{"message": "Password validation failed"}` → Use stronger password
- `{"message": "Email already exists"}` → Admin already claimed, see [Recovery](#recovery-lost-admin-access)
- `{"errors": [{"status": "403"}]}` → Admin already exists, signup locked

---

## Step 2: Sign In and Get Token

```bash
TOKEN=$(curl -s -X POST http://localhost:6336/action/user_account/signin \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "email": "admin@admin.com",
      "password": "adminadmin"
    }
  }' | jq -r '.[] | select(.ResponseType == "client.store.set") | .Attributes.value')

# Save token to file for reuse
echo "$TOKEN" > /tmp/daptin-token.txt

echo "Token: $TOKEN"
```

**Expected**: Long JWT token like `eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJlbWFpbCI6ImFkbWluQGFkbWluLmNvbSIsImV4cCI6MTc2OTYyOTg0OSwiaWF0IjoxNzY5MzcwNjQ5LCJpc3MiOiJkYXB0aW4tMDE5YmY2IiwianRpIjoiMDE5YmY2YjUtODQwMS03YzE1LTljNzktY2NmMzc2MWQxMDA1IiwibmFtZSI6IkFkbWluIiwibmJmIjoxNzY5MzcwNjQ5LCJzdWIiOiIwMTliZjY3OS1jYTZmLTcwNTUtYTY2Yy1iNDZhYmRhZTMzM2QifQ.14aCrLSt7D-jJ39oc3efyzMcOdbux2zgGmgWZXXva_o`

**If empty token**:
```bash
# Check the full response
curl -s -X POST http://localhost:6336/action/user_account/signin \
  -H "Content-Type: application/json" \
  -d '{"attributes":{"email":"admin@admin.com","password":"adminadmin"}}'
```

---

## Step 3: Become Administrator

**CRITICAL**: The action is on `world` entity, not `user_account`.

```bash
TOKEN=$(cat /tmp/daptin-token.txt)

curl -s -X POST http://localhost:6336/action/world/become_an_administrator \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{}'
```

**Expected response**:
```json
[
  {
    "ResponseType": "client.redirect",
    "Attributes": {
      "delay": 7000,
      "location": "/",
      "window": "self"
    }
  },
  {
    "ResponseType": "Restart",
    "Attributes": null
  }
]
```

**Server may restart** - wait 5-10 seconds.

**If you get "Unauthorized"**:

This is the #1 issue with fresh installs. See [[Common-Errors#unauthorized-on-become_an_administrator]].

**Quick fix**:
```bash
# Kill Olric cache on port 5336
lsof -i :5336 -t | xargs kill -9 2>/dev/null || true
sleep 2

# Restart server
./scripts/testing/test-runner.sh start

# Try again after server starts
sleep 10
curl -X POST http://localhost:6336/action/world/become_an_administrator \
  -H "Authorization: Bearer $TOKEN" -d '{}'
```

---

## Step 4: Get Fresh Token After Restart

Server restart invalidates old token. Sign in again:

```bash
sleep 5  # Wait for server to fully restart

TOKEN=$(curl -s -X POST http://localhost:6336/action/user_account/signin \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "email": "admin@admin.com",
      "password": "adminadmin"
    }
  }' | jq -r '.[] | select(.ResponseType == "client.store.set") | .Attributes.value')

echo "$TOKEN" > /tmp/daptin-token.txt

echo "✓ Admin setup complete! Token saved to /tmp/daptin-token.txt"
```

---

## Verification

### Check Admin Status

```bash
TOKEN=$(cat /tmp/daptin-token.txt)

# Method 1: Check administrators group membership
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/usergroup?page%5Bsize%5D=100" | \
  jq '.data[] | select(.attributes.name == "administrators")'

# Method 2: Check database directly
sqlite3 daptin.db "
  SELECT ua.name, ua.email, ug.name as group_name
  FROM user_account ua
  JOIN user_account_user_account_id_has_usergroup_usergroup_id j
    ON ua.id = j.user_account_id
  JOIN usergroup ug
    ON j.usergroup_id = ug.id
  WHERE ug.name = 'administrators';
"
```

**Expected**: Your user shown in administrators group.

### Check Token Works

```bash
TOKEN=$(cat /tmp/daptin-token.txt)

# Should see multiple tables
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/world?page%5Bsize%5D=100" | \
  jq '.data | length'

# Should be > 10
```

### Decode Token (Optional)

```bash
TOKEN=$(cat /tmp/daptin-token.txt)

# View token contents
echo $TOKEN | cut -d. -f2 | base64 -d 2>/dev/null | jq .
```

**Expected fields**:
- `email`: Your email
- `name`: Your name
- `sub`: User reference_id
- `exp`: Expiration timestamp (3 days from issue)

---

## What Changed After Admin Setup

### Before Admin Setup

| Who | Can Do |
|-----|--------|
| Anyone | Everything - create users, delete tables, access all data |
| Guest | Create, read, update, delete on all tables |

### After Admin Setup

| Who | Can Do |
|-----|--------|
| Guest | Only peek at public data, signin action |
| Regular users | Only their own data + shared group data |
| Administrator | Everything |

**Key changes**:
- ✅ Signup action is disabled (permission changed to guest=0)
- ✅ New administrator usergroup created
- ✅ Your user added to administrators group
- ✅ System tables locked down
- ✅ Default permissions enforced

---

## Next Steps

Now that you're admin, you can:

1. **Create your data schema** → [[Schema-Definition]]
2. **Create additional users** → [[Users-and-Groups#creating-users]]
3. **Set up permissions** → [[Permissions]]
4. **Configure cloud storage** → [[Cloud-Storage]]

**Recommended next read**: [[Create-Your-First-Table]] (when available)

---

## Recovery: Lost Admin Access

### If You Forgot Admin Password

**Option 1: Reset via database** (requires database access)

```bash
# Generate new bcrypt hash for password "password123"
HASH=$(htpasswd -bnBC 10 "" password123 | tr -d ':\n')

# Update admin password in database
sqlite3 daptin.db "
  UPDATE user_account
  SET password = '$HASH'
  WHERE email = 'admin@admin.com';
"

# Now signin with new password
curl -X POST http://localhost:6336/action/user_account/signin \
  -d '{"attributes":{"email":"admin@admin.com","password":"password123"}}'
```

**Option 2: Start fresh** (wipes all data)

```bash
# Stop server
./scripts/testing/test-runner.sh stop

# Delete database
rm daptin.db

# Start server
./scripts/testing/test-runner.sh start

# Follow Steps 1-4 again
```

### If Someone Else Claimed Admin

If you're setting up a server and someone else already claimed admin, you have two options:

1. **Ask them to create an account for you** (recommended)
2. **Start with fresh database** (if appropriate)

**There is no backdoor** - this is by design for security.

---

## Common Issues

### "Unauthorized" on become_an_administrator

**Cause**: Stale Olric cache on port 5336

**Solution**: Kill port 5336, restart server, try again

**Full details**: [[Common-Errors#unauthorized-on-become_an_administrator]]

### Signup returns 403 Forbidden

**Cause**: Admin already exists, signup is locked

**Solution**: Ask existing admin to create account for you, or start with fresh database

### Token is empty

**Cause**: Sign response parsing failed

**Solution**:
```bash
# Debug - view full response
curl -s -X POST http://localhost:6336/action/user_account/signin \
  -H "Content-Type: application/json" \
  -d '{"attributes":{"email":"admin@admin.com","password":"adminadmin"}}' | jq .

# Check for error messages
```

### Server won't start

**Cause**: Port already in use or database corruption

**Solution**:
```bash
# Check what's using ports
lsof -i :6336
lsof -i :5336

# Kill processes
pkill -9 -f daptin

# Check logs
tail -50 /tmp/daptin.log

# Try fresh database if corrupted
rm daptin.db
```

---

## Using test-runner.sh (Recommended)

The test-runner script handles server lifecycle correctly:

```bash
# Check if running
./scripts/testing/test-runner.sh check

# Start (kills old processes first, including port 5336)
./scripts/testing/test-runner.sh start

# Stop (kills both 6336 and 5336)
./scripts/testing/test-runner.sh stop

# Get token
./scripts/testing/test-runner.sh token

# Make API call
./scripts/testing/test-runner.sh get /api/world

# View logs
./scripts/testing/test-runner.sh logs

# View errors only
./scripts/testing/test-runner.sh errors
```

**Advantages**:
- Automatically kills Olric cache (port 5336)
- Manages token file
- Provides convenient API access
- Shows useful logs

---

## Security Note

**Change default password immediately in production**:

The example uses `admin@admin.com` / `adminadmin` for documentation clarity.

In production:
- Use strong password (16+ characters, random)
- Use real email address
- Consider [[Two-Factor-Auth]]
- Review [[Security-Checklist]] (when available)

---

**Last Updated**: 2026-01-26
**Testing Status**: ✅ All commands verified working
**Based On**: Walkthrough testing Steps 0.1-0.4
