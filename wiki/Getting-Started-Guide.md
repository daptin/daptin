# Getting Started with Daptin

Get up and running with Daptin in minutes.

---

## 1. Start Daptin

```bash
# Default (SQLite)
go run main.go

# With PostgreSQL
go run main.go -db_type postgres -db_connection_string "host=localhost port=5432 user=postgres password=secret dbname=daptin sslmode=disable"
```

Open `http://localhost:6336` to access the dashboard.

---

## 2. Set Up Your First Admin

On a fresh install, **anyone can do anything**. You need to claim admin immediately.

```bash
# Sign up
curl -X POST http://localhost:6336/action/user_account/signup \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "name": "Admin",
      "email": "admin@example.com",
      "password": "yourpassword",
      "passwordConfirm": "yourpassword"
    }
  }'

# Sign in (save the token from response)
curl -X POST http://localhost:6336/action/user_account/signin \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "email": "admin@example.com",
      "password": "yourpassword"
    }
  }'

# Become admin (NOTE: action is on "world", not "user_account")
curl -X POST http://localhost:6336/action/world/become_an_administrator \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{}'
```

The server restarts. Sign in again - you're now admin.

**After this**: The system locks down. Public signup is disabled. Only you can create new users.

---

## 3. What If Signup Fails?

If you get a **403 error** on signup, someone already claimed admin. You have two options:

### Option A: Contact the Admin

Ask them to create an account for you (see "Admin: Create a User" below).

### Option B: Reset the Database

If this is your own server and you lost access:

```bash
# Stop Daptin
# Delete the database file (default: daptin.db)
rm daptin.db
# Restart Daptin
```

This wipes everything. Start fresh with step 2.

---

## 4. Admin: Create a User

Since signup is disabled after admin setup, create users directly:

```bash
# As admin, create a new user
curl -X POST http://localhost:6336/api/user_account \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "user_account",
      "attributes": {
        "name": "New User",
        "email": "user@example.com",
        "password": "userpassword"
      }
    }
  }'
```

The user can now sign in with these credentials.

---

## 5. Admin: Re-enable Public Signup

If you want anyone to sign up:

```bash
# Find the signup action
curl "http://localhost:6336/api/action?filter[action_name]=signup" \
  -H "Authorization: Bearer $ADMIN_TOKEN"

# Note the action ID, then update permission to allow guest execute
curl -X PATCH "http://localhost:6336/api/action/ACTION_ID" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "action",
      "id": "ACTION_ID",
      "attributes": {
        "permission": 2085152
      }
    }
  }'
```

---

## 6. Create Your Data

### Define a Table

Create a file `schema.yaml`:

```yaml
Tables:
  - TableName: todo
    Columns:
      - Name: title
        DataType: varchar(500)
        ColumnType: label
      - Name: completed
        DataType: bool
        ColumnType: truefalse
        DefaultValue: "false"
```

Upload it via dashboard or restart Daptin with schema file.

### Use the API

```bash
# Create
curl -X POST http://localhost:6336/api/todo \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "todo",
      "attributes": {"title": "My first task"}
    }
  }'

# List
curl http://localhost:6336/api/todo \
  -H "Authorization: Bearer $TOKEN"

# Update
curl -X PATCH http://localhost:6336/api/todo/RECORD_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "todo",
      "id": "RECORD_ID",
      "attributes": {"completed": true}
    }
  }'

# Delete
curl -X DELETE http://localhost:6336/api/todo/RECORD_ID \
  -H "Authorization: Bearer $TOKEN"
```

---

## 7. Filter and Sort

```bash
# Filter
curl 'http://localhost:6336/api/todo?query=[{"column":"completed","operator":"is","value":"false"}]' \
  -H "Authorization: Bearer $TOKEN"

# Sort (descending with -)
curl 'http://localhost:6336/api/todo?sort=-created_at' \
  -H "Authorization: Bearer $TOKEN"

# Paginate
curl 'http://localhost:6336/api/todo?page[number]=1&page[size]=10' \
  -H "Authorization: Bearer $TOKEN"
```

### Filter Operators

| Operator | Meaning |
|----------|---------|
| `is` | Equals |
| `is not` | Not equals |
| `contains` | Substring match |
| `begins with` | Starts with |
| `ends with` | Ends with |
| `any of` | In list |
| `is empty` | Is null |

---

## Troubleshooting

| Problem | Solution |
|---------|----------|
| **403 on signup** | Admin exists. Contact admin or reset database. |
| **401 Unauthorized** | Token expired. Sign in again. |
| **API returns HTML** | Add header: `Accept: application/vnd.api+json` |
| **"become_an_administrator" fails** | Admin already exists. Only first user can claim. |

---

## Next Steps

- [Permissions](Permissions.md) - Control who can access what
- [Relationships](Relationships.md) - Link tables together
- [Actions Overview](Actions-Overview.md) - Add business logic
- [Schema Definition](Schema-Definition.md) - Full schema options
