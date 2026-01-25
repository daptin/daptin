# Walkthrough: Building a Product Catalog with Cloud Storage and Permissions

**A complete, step-by-step guide for beginners** to build a real-world product catalog API with Daptin.

By the end of this walkthrough, you'll have:
- ✅ A working REST API for managing products
- ✅ Product images stored in cloud storage (not in the database)
- ✅ Three user roles with different permission levels
- ✅ Custom actions for publishing/unpublishing products
- ✅ A deep understanding of Daptin's permission system

**Time Required**: 30-45 minutes
**Difficulty**: Beginner (no prior Daptin experience needed)

---

## What You'll Learn

This walkthrough teaches you:

1. **Schema Definition**: How to create tables using YAML files
2. **Cloud Storage**: How to connect to S3/Minio or local storage for file uploads
3. **User Management**: Creating users and organizing them into groups
4. **Permission System**: Daptin's three-tier permission model (guest/owner/group)
5. **Relationships**: Linking users, groups, and records together
6. **Custom Actions**: Creating executable actions beyond basic CRUD
7. **Testing**: Verifying permissions work correctly

---

## The Scenario

**Company**: TechGear Inc. - An electronics e-commerce company

**Team Structure**:
- **Admin**: You (full access to everything)
- **Marketing Team**: Can view products and upload/update photos
- **Sales Team**: Can only view products (read-only)
- **Guests**: Public users who can only see published products

**What We're Building**:
1. A `product` table with name, price, description, published flag, and photo
2. Photos stored in cloud storage (S3/Minio/local filesystem)
3. Permission controls so each team has appropriate access
4. A custom action to toggle the published status of products

**API Endpoints You'll Create**:
- `GET /api/product` - List products
- `POST /api/product` - Create product
- `PATCH /api/product/{id}` - Update product
- `POST /action/product/toggle_publish` - Custom action

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                         USERS                                    │
├──────────────┬──────────────┬──────────────┬───────────────────┤
│    Admin     │  Marketing   │    Sales     │      Guest        │
│  (full)      │ (upload)     │ (view only)  │  (public only)    │
└──────┬───────┴──────┬───────┴──────┬───────┴───────┬───────────┘
       │              │              │               │
       ▼              ▼              ▼               ▼
┌─────────────────────────────────────────────────────────────────┐
│                      product TABLE                               │
│  ┌──────┬────────┬───────────┬──────────┬────────────────────┐  │
│  │ name │ price  │ published │ photo    │ permission         │  │
│  │      │        │ (boolean) │ (file)   │ (per-record)       │  │
│  └──────┴────────┴───────────┴────┬─────┴────────────────────┘  │
└───────────────────────────────────┼─────────────────────────────┘
                                    │
                    ForeignKeyData.Namespace = "product-images"
                                    │
┌───────────────────────────────────▼─────────────────────────────┐
│                      cloud_store                                 │
│  name: "product-images"                                          │
│  root_path: "product-images:techgear-bucket/products"           │
│  credential_id: → minio-creds                                    │
└─────────────────────────────────────────────────────────────────┘
```

---

## Before You Begin

### Prerequisites Check

Make sure you have these installed:

```bash
# Check Go version (need 1.19 or higher)
go version
# Expected: go version go1.24.3 darwin/arm64 (or similar)

# Check jq (JSON parser)
jq --version
# Expected: jq-1.6 (or similar)

# Check you're in the Daptin directory
pwd
# Expected: .../daptin

# Check source code exists
ls main.go
# Expected: main.go
```

If anything is missing:
- **Go**: Install from https://go.dev/dl/
- **jq**: `brew install jq` (Mac) or `apt-get install jq` (Linux)
- **Daptin source**: Clone from https://github.com/daptin/daptin

### What This Walkthrough Uses

- **Database**: SQLite (created automatically as `daptin.db`)
- **Storage**: Local filesystem at `/tmp/product-images` (easiest for beginners)
- **Port**: 6336 (HTTP API)
- **Port**: 5336 (Internal Olric cache)

**Note**: This walkthrough uses local filesystem storage for simplicity. For production, you'd use S3/Minio/Google Cloud Storage (explained in Step 1).

---

## Understanding Daptin Basics

Before we start, here's what you need to know:

**What is Daptin?**
Daptin is a backend server that:
1. Reads table schemas from YAML files
2. Automatically creates REST APIs for those tables
3. Provides built-in user authentication and permissions
4. Handles file uploads to cloud storage
5. Supports custom actions and workflows

**How Daptin Works**:
```
schema_product.yaml → Daptin reads → Creates product table → Exposes /api/product
```

**Key Concepts**:
- **World**: A table definition (e.g., "product" table)
- **Entity**: A record in a table (e.g., one product)
- **Usergroup**: A group of users (e.g., "marketing" group)
- **Permission**: Who can do what (read, update, delete, etc.)
- **Action**: A custom operation (e.g., "toggle_publish")

---

## Step 0: Initial Setup (Fresh Database)

**What we're doing**: Setting up a clean Daptin instance with our product table.

**Why start fresh**: Using an existing database can cause permission issues due to cached data. Starting fresh ensures everything works correctly.

---

### 0.1 Create Product Schema File

**What is a schema file?**
A schema file tells Daptin what tables to create. It's like a blueprint for your database.

**Why YAML?**
YAML is easy to read and write. Daptin reads all `schema_*.yaml` files on startup and creates tables automatically.

Create the file:

```bash
cat > schema_product.yaml << 'EOF'
Tables:
  - TableName: product
    # DefaultPermission controls who can do what with new products
    # 704385 = Calculated below (don't worry, we'll explain permissions later!)
    DefaultPermission: 704385
    Columns:
      - Name: name
        DataType: varchar(500)
        ColumnType: name
        IsNullable: false           # Required field

      - Name: price
        DataType: float
        ColumnType: measurement
        DefaultValue: "0"           # Defaults to 0 if not provided

      - Name: description
        DataType: text
        ColumnType: content
        IsNullable: true            # Optional field

      - Name: published
        DataType: bool
        ColumnType: truefalse
        DefaultValue: "false"       # New products are unpublished by default

      - Name: photo
        DataType: text
        ColumnType: file            # Special type for file uploads
        IsNullable: true            # Optional - not all products need photos
        IsForeignKey: true          # Links to cloud storage
        ForeignKeyData:
          DataSource: cloud_store   # Where to store files
          Namespace: product-images # Which cloud store to use (we'll create this in Step 1)
          KeyName: photos           # Subfolder within cloud store
EOF
```

**What just happened?**
- Created a file named `schema_product.yaml`
- Defined a table named `product` with 5 columns
- The `photo` column is special - it stores files in cloud storage, not the database

**Verify the file was created:**

```bash
cat schema_product.yaml
# You should see the YAML content above

ls -lh schema_product.yaml
# Expected: -rw-r--r--  1 user  staff   XXX bytes  schema_product.yaml
```

### 0.2 Start Daptin Fresh

**What we're doing**: Starting Daptin server with a clean database.

**Why kill existing processes?**
Old Daptin processes keep permissions in cache (Olric). If you don't kill them, you'll get confusing "403 Forbidden" errors later.

```bash
# Step 1: Kill any existing Daptin processes
pkill -9 -f daptin 2>/dev/null || true
pkill -9 -f "go run main" 2>/dev/null || true
sleep 2
echo "✓ Killed existing processes"

# Step 2: Free the ports Daptin uses
lsof -i :6336 -t | xargs kill -9 2>/dev/null || true  # HTTP API port
lsof -i :5336 -t | xargs kill -9 2>/dev/null || true  # Olric cache port
echo "✓ Freed ports 6336 and 5336"

# Step 3: Remove old database for clean start
rm -f daptin.db
echo "✓ Removed old database"

# Step 4: Start server in background
nohup go run main.go > /tmp/daptin.log 2>&1 &
echo "✓ Started Daptin server"
echo "Waiting 20 seconds for initialization..."
sleep 20

# Step 5: Verify server is running
curl -s http://localhost:6336/api/world | head -c 50
echo ""
echo "✓ Server is running!"
```

**Expected output**: You should see some JSON starting with `{"data":[{...`

**If it doesn't work:**
- Check the log: `tail -f /tmp/daptin.log`
- Look for errors in red
- Make sure ports 6336 and 5336 are free: `lsof -i :6336`

---

### 0.3 Create Admin Account

**What we're doing**: Creating the first user account, which becomes the admin.

**Why "adminadmin"?**
This is just a simple password for testing. In production, use a strong password.

```bash
curl -s -X POST http://localhost:6336/action/user_account/signup \
  -H "Content-Type: application/json" \
  -d '{"attributes":{"name":"Admin","email":"admin@admin.com","password":"adminadmin","passwordConfirm":"adminadmin"}}' | jq
```

**Expected output:**
```json
[
  {
    "ResponseType": "client.notify",
    "Attributes": {
      "message": "Created",
      "title": "Success",
      "type": "success"
    }
  }
]
```

**What just happened?**
- Created a user with email `admin@admin.com`
- Password is `adminadmin` (8 characters minimum)
- This user is NOT an administrator yet (we'll do that next)

---

### 0.4 Get Admin Token and Become Administrator

**What we're doing**:
1. Sign in to get an authentication token (like a temporary password)
2. Use the token to become an administrator
3. Sign in again to get a fresh token

**Why do we need a token?**
The token proves you're authenticated. Every API call (except signup/signin) needs this token.

```bash
# Step 1: Sign in and get token
echo "Signing in as admin..."
TOKEN=$(curl -s -X POST http://localhost:6336/action/user_account/signin \
  -H "Content-Type: application/json" \
  -d '{"attributes":{"email":"admin@admin.com","password":"adminadmin"}}' | \
  jq -r '.[] | select(.ResponseType == "client.store.set") | .Attributes.value')

# Save token to file for later use
echo "$TOKEN" > /tmp/daptin-token.txt
echo "✓ Got token: ${TOKEN:0:20}..."

# Step 2: Become administrator
echo "Becoming administrator..."
curl -s -X POST http://localhost:6336/action/world/become_an_administrator \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"attributes":{}}' | jq

# Step 3: Server may restart - wait and get fresh token
echo "Waiting 5 seconds for server to stabilize..."
sleep 5

TOKEN=$(curl -s -X POST http://localhost:6336/action/user_account/signin \
  -H "Content-Type: application/json" \
  -d '{"attributes":{"email":"admin@admin.com","password":"adminadmin"}}' | \
  jq -r '.[] | select(.ResponseType == "client.store.set") | .Attributes.value')

echo "$TOKEN" > /tmp/daptin-token.txt
echo "✓ Admin token ready and saved to /tmp/daptin-token.txt"
```

**What is a Bearer token?**
When you make API calls, you include the token like this:
```
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

**IMPORTANT**: After `become_an_administrator`, the public signup is locked for security. From now on, only admins can create new users via the API.

**Verify you're an admin:**
```bash
TOKEN=$(cat /tmp/daptin-token.txt)
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:6336/api/usergroup | \
  jq '.data[] | select(.attributes.name == "administrators")'
# You should see the administrators group
```

---

## Step 1: Set Up Cloud Storage

**What we're doing**: Configuring where product photos will be stored.

**Why cloud storage?**
Storing files (images, PDFs, etc.) in the database makes it slow and bloated. Cloud storage is:
- **Faster**: Optimized for file serving
- **Cheaper**: Pay only for what you use
- **Scalable**: Handle millions of files

**Options**:
1. **Local Filesystem** (easiest for beginners) - stores files in `/tmp/product-images`
2. **Minio** (S3-compatible, free, local) - like AWS S3 but runs on your machine
3. **AWS S3** (production-ready cloud storage)
4. **Google Cloud Storage** (Google's cloud storage)

**We'll use**: Local filesystem for this walkthrough (simplest, no setup needed).

---

### 1.1 Create Local Storage Directory

**What we're doing**: Creating a folder where product photos will be saved.

```bash
# Create the storage directory
mkdir -p /tmp/product-images
echo "✓ Created /tmp/product-images directory"

# Verify it exists
ls -ld /tmp/product-images
# Expected: drwxr-xr-x  2 user  wheel  64 Jan 25 12:00 /tmp/product-images
```

**What is /tmp?**
`/tmp` is a temporary directory on your system. Files here might be deleted on reboot, but it's perfect for testing.

---

### 1.2 Create Cloud Store Record

**What is a cloud_store?**
A `cloud_store` record tells Daptin:
- Where to store files (local filesystem, S3, etc.)
- How to connect (credentials, paths)
- What to call this storage (name: "product-images")

**Important**: The `name` field must match the `Namespace` in your schema file (`product-images` in our case).

```bash
TOKEN=$(cat /tmp/daptin-token.txt)

curl -X POST http://localhost:6336/api/cloud_store \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "cloud_store",
      "attributes": {
        "name": "product-images",
        "store_type": "local",
        "store_provider": "localstore",
        "root_path": "/tmp/product-images",
        "store_parameters": "{}"
      }
    }
  }' | jq '.data.attributes | {name, store_type, root_path}'
```

**Expected output:**
```json
{
  "name": "product-images",
  "store_type": "local",
  "root_path": "/tmp/product-images"
}
```

**What each field means**:
- `name`: Identifier that matches schema Namespace
- `store_type`: Type of storage (local, s3, gcs)
- `store_provider`: Provider name (localstore, Minio, AWS)
- `root_path`: Where files are stored
- `store_parameters`: Extra config (empty for local)

**Verify it was created:**
```bash
curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:6336/api/cloud_store | \
  jq '.data[] | {name: .attributes.name, type: .attributes.store_type}'
```

---

### 1.3 Restart Server to Load Cloud Storage

**Why restart?**
Daptin loads cloud storage configuration on startup. After creating a `cloud_store`, you must restart for it to take effect.

```bash
# Kill current server
pkill -f "go run main"
sleep 2

# Start fresh
nohup go run main.go > /tmp/daptin.log 2>&1 &
echo "Waiting 20 seconds for server to restart..."
sleep 20

# Get new token (old one may be invalid)
TOKEN=$(curl -s -X POST http://localhost:6336/action/user_account/signin \
  -H "Content-Type: application/json" \
  -d '{"attributes":{"email":"admin@admin.com","password":"adminadmin"}}' | \
  jq -r '.[] | select(.ResponseType == "client.store.set") | .Attributes.value')

echo "$TOKEN" > /tmp/daptin-token.txt
echo "✓ Server restarted, cloud storage loaded"
```

**Verify cloud storage is working:**
Check the logs for cloud storage initialization:
```bash
grep -i "Sync table column" /tmp/daptin.log
# Expected: [71] Sync table column [product][photo] at /tmp/product-images
```

---

<details>
<summary><strong>Alternative: Using Minio (S3-Compatible)</strong></summary>

If you want to use Minio instead of local filesystem:

### 1.1 Create Credential (Minio Version)

**What is a credential?**
A credential stores authentication info (API keys, passwords) for cloud services.

```bash
TOKEN=$(cat /tmp/daptin-token.txt)

# Create credential with Minio access keys
curl -X POST http://localhost:6336/api/credential \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "credential",
      "attributes": {
        "name": "minio-creds",
        "content": "{\"type\":\"s3\",\"provider\":\"Minio\",\"env_auth\":\"false\",\"access_key_id\":\"minioadmin\",\"secret_access_key\":\"minioadmin123\",\"endpoint\":\"http://localhost:9000\",\"region\":\"us-east-1\"}"
      }
    }
  }'

# Create cloud store pointing to Minio
curl -X POST http://localhost:6336/api/cloud_store \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "cloud_store",
      "attributes": {
        "name": "product-images",
        "store_type": "s3",
        "store_provider": "s3",
        "root_path": "product-images:techgear-bucket",
        "store_parameters": "{}"
      }
    }
  }'

# Link credential to cloud store
CRED_ID=$(curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:6336/api/credential | jq -r '.data[0].id')
STORE_ID=$(curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:6336/api/cloud_store | jq -r '.data[0].id')

curl -X PATCH "http://localhost:6336/api/cloud_store/$STORE_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d "{\"data\":{\"type\":\"cloud_store\",\"id\":\"$STORE_ID\",\"relationships\":{\"credential_id\":{\"data\":{\"type\":\"credential\",\"id\":\"$CRED_ID\"}}}}}"

# Restart server
pkill -f "go run main" && sleep 2 && nohup go run main.go > /tmp/daptin.log 2>&1 &
sleep 20
TOKEN=$(curl -s -X POST http://localhost:6336/action/user_account/signin \
  -H "Content-Type: application/json" \
  -d '{"attributes":{"email":"admin@admin.com","password":"adminadmin"}}' | \
  jq -r '.[] | select(.ResponseType == "client.store.set") | .Attributes.value')
echo "$TOKEN" > /tmp/daptin-token.txt
```

</details>

---

## Step 2: Create User Groups

**What we're doing**: Creating groups to organize users by role.

**Why use groups?**
Instead of setting permissions for each individual user, we:
1. Create groups (marketing, sales)
2. Add users to groups
3. Set permissions for the groups

This makes it easy to manage many users with the same role.

**Groups in Daptin**:
- **administrators**: Created automatically, has full access
- **users**: Created automatically, all users belong here
- **marketing**: We'll create this (can upload photos)
- **sales**: We'll create this (read-only)

---

### 2.1 Create Marketing Group

```bash
TOKEN=$(cat /tmp/daptin-token.txt)

curl -X POST http://localhost:6336/api/usergroup \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "usergroup",
      "attributes": {
        "name": "marketing"
      }
    }
  }' | jq '.data.attributes.name'
```

**Expected output:** `"marketing"`

---

### 2.2 Create Sales Group

```bash
curl -X POST http://localhost:6336/api/usergroup \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "usergroup",
      "attributes": {
        "name": "sales"
      }
    }
  }' | jq '.data.attributes.name'
```

**Expected output:** `"sales"`

---

### 2.3 Verify All Groups Exist

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:6336/api/usergroup | \
  jq '.data[] | {id, name: .attributes.name}'
```

**Expected output:**
```json
{"id": "...", "name": "administrators"}
{"id": "...", "name": "guests"}
{"id": "...", "name": "users"}
{"id": "...", "name": "marketing"}
{"id": "...", "name": "sales"}
```

**What each group means**:
- **administrators**: Full system access
- **guests**: Public users (not logged in)
- **users**: All registered users
- **marketing**: Our custom group for marketing team
- **sales**: Our custom group for sales team

---

## Step 3: Understanding the Product Table and Permissions

**What we're doing**: Verifying our product table was created and understanding its permission system.

**Good news**: The table was already created automatically when Daptin started (from the `schema_product.yaml` file in Step 0).

**In this step, we'll**:
1. Verify the table exists
2. Understand how permissions work
3. Learn to calculate permission values

---

### 3.1 Understanding Permission Values (Simplified)

**What are permissions?**
Permissions control who can do what with your data:
- **Peek**: See that records exist in lists (but can't read details)
- **Read**: View full record details
- **Create**: Make new records
- **Update**: Modify existing records
- **Delete**: Remove records
- **Execute**: Run custom actions
- **Refer**: Use records in relationships

**The three permission levels**:
1. **Guest** permissions: What unauthenticated users can do
2. **Owner** permissions: What the creator of a record can do
3. **Group** permissions: What group members can do

**Permission values (simplified table)**:

| Permission | Value |
|------------|-------|
| None       | 0     |
| Peek       | 1     |
| Read       | 2     |
| Create     | 4     |
| Update     | 8     |
| Delete     | 16    |
| Execute    | 32    |
| Refer      | 64    |
| **Full**   | **127** (all of the above) |

**Combine permissions by adding**:
- Read + Update = 2 + 8 = 10
- Read + Update + Execute = 2 + 8 + 32 = 42

---

### 3.2 How Permission Calculation Works

Permissions use a three-tier bit-shift model (don't worry if this sounds complex, we'll make it simple):

**The formula**:
```
Permission = Guest + (Owner × 128) + (Group × 16384)
```

**Why these multipliers?**
- Guest: No multiplier (positions 0-6)
- Owner: × 128 (or << 7, shifts to positions 7-13)
- Group: × 16384 (or << 14, shifts to positions 14-20)

This is called "bit-shifting" - it puts each role's permissions in different "slots" so they don't interfere.

**For our product table (DefaultPermission: 704385)**:

Let's break down what `704385` means:

1. **Guest**: Peek only (1)
   - Can see products exist in lists
   - CANNOT read full details

2. **Owner**: Full access (127)
   - The person who created the product has full control
   - 127 × 128 = 16,256

3. **Group**: Read + Update + Execute (42)
   - Group members can view, edit, and run actions
   - 42 = Read(2) + Update(8) + Execute(32)
   - 42 × 16,384 = 688,128

**Total**: 1 + 16,256 + 688,128 = **704,385**

**Visual breakdown**:
```
Permission 704385 means:
├─ Guest: Peek (1)                    → Can see products in lists
├─ Owner: Full (127)                  → Creator has full control
└─ Group: Read+Update+Execute (42)    → Team can view & edit
```

**Common permission values you'll use**:
```javascript
// For our walkthrough:
704385 = Default product permission (guest peek, owner full, group can update)
704387 = Published product (guest can read, owner full, group can update)
688128 = Group permission for join tables (42 << 14)
32768  = Read-only group permission (2 << 14)
```

**Quick calculator** (if you want to create your own):
```bash
# Formula: guest + (owner * 128) + (group * 16384)
# Example: Guest=Peek(1), Owner=Full(127), Group=Read+Update(10)
echo $((1 + (127 * 128) + (10 * 16384)))
# Output: 180097
```

### 3.3 Verify Table Was Created

Let's confirm the product table exists and check its default permission.

```bash
TOKEN=$(cat /tmp/daptin-token.txt)

# Check if table is empty (should be 0 products)
echo "Number of products:"
curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:6336/api/product | jq '.data | length'

# Check table configuration
echo ""
echo "Product table configuration:"
curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:6336/api/world | \
  jq '.data[] | select(.attributes.table_name == "product") | {
    table_name: .attributes.table_name,
    default_permission: .attributes.default_permission
  }'
```

**Expected output:**
```
Number of products:
0

Product table configuration:
{
  "table_name": "product",
  "default_permission": 704385
}
```

**What this means**:
- Table exists ✓
- No products yet (count = 0) ✓
- Default permission is 704385 (matches our schema) ✓

**Verify the columns exist:**
```bash
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/world?filter[table_name]=product&include=columns" | \
  jq '.included[] | select(.type == "column") | .attributes.column_name'
```

**Expected output:**
```
"name"
"price"
"description"
"published"
"photo"
```

---

<details>
<summary><strong>Troubleshooting: If table doesn't exist</strong></summary>

If the product table wasn't created:

1. Check the schema file exists:
   ```bash
   cat schema_product.yaml
   ```

2. Check server logs for errors:
   ```bash
   grep -i "product" /tmp/daptin.log | grep -i "error"
   ```

3. Restart server to reload schema:
   ```bash
   pkill -f "go run main"
   sleep 2
   nohup go run main.go > /tmp/daptin.log 2>&1 &
   sleep 20
   ```

</details>

---

## Step 4: Create Test Users

**What we're doing**: Creating two test users and adding them to their respective groups.

**Why create users via API?**
After becoming an administrator, the public signup is locked for security. Only admins can create new users.

**About the password hash**:
The hash `$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy` = "password123"

This is a bcrypt hash. Daptin never stores plain-text passwords - only hashes.

**Users we'll create**:
1. **Marketing Mary** (mary@techgear.com) - Marketing team member
2. **Sales Sam** (sam@techgear.com) - Sales team member

---

### 4.1 Create Marketing User

```bash
TOKEN=$(cat /tmp/daptin-token.txt)

echo "Creating Marketing Mary..."
curl -s -X POST http://localhost:6336/api/user_account \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "user_account",
      "attributes": {
        "name": "Marketing Mary",
        "email": "mary@techgear.com",
        "password": "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"
      }
    }
  }' | jq '{id: .data.id, name: .data.attributes.name, email: .data.attributes.email}'
```

**Expected output:**
```json
{
  "id": "019bf54b-...",
  "name": "Marketing Mary",
  "email": "mary@techgear.com"
}
```

---

### 4.2 Add Mary to Marketing Group

**What is a join table?**
To link Mary to the marketing group, we create a record in a "join table" named:
`user_account_user_account_id_has_usergroup_usergroup_id`

This long name follows the pattern: `{table1}_{fk1}_has_{table2}_{fk2}`

```bash
# Get Mary's ID
MARY_ID=$(curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:6336/api/user_account | \
  jq -r '.data[] | select(.attributes.email == "mary@techgear.com") | .id')

# Get Marketing group ID
MARKETING_GROUP_ID=$(curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:6336/api/usergroup | \
  jq -r '.data[] | select(.attributes.name == "marketing") | .id')

echo "Mary ID: $MARY_ID"
echo "Marketing Group ID: $MARKETING_GROUP_ID"

# Add Mary to marketing group
curl -s -X POST http://localhost:6336/api/user_account_user_account_id_has_usergroup_usergroup_id \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d "{
    \"data\": {
      \"type\": \"user_account_user_account_id_has_usergroup_usergroup_id\",
      \"attributes\": {
        \"user_account_id\": \"$MARY_ID\",
        \"usergroup_id\": \"$MARKETING_GROUP_ID\"
      }
    }
  }" | jq '{id: .data.id, user: .data.attributes.user_account_id, group: .data.attributes.usergroup_id}'
```

**Expected output:**
```json
{
  "id": "...",
  "user": "019bf54b-...",
  "group": "..."
}
```

### 4.3 Create Sales User and Add to Sales Group

Same process for Sam:

```bash
echo "Creating Sales Sam..."
curl -s -X POST http://localhost:6336/api/user_account \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "user_account",
      "attributes": {
        "name": "Sales Sam",
        "email": "sam@techgear.com",
        "password": "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"
      }
    }
  }' | jq '{id: .data.id, name: .data.attributes.name, email: .data.attributes.email}'

# Get Sam's ID
SAM_ID=$(curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:6336/api/user_account | \
  jq -r '.data[] | select(.attributes.email == "sam@techgear.com") | .id')

# Get Sales group ID
SALES_GROUP_ID=$(curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:6336/api/usergroup | \
  jq -r '.data[] | select(.attributes.name == "sales") | .id')

echo "Sam ID: $SAM_ID"
echo "Sales Group ID: $SALES_GROUP_ID"

# Add Sam to sales group
curl -s -X POST http://localhost:6336/api/user_account_user_account_id_has_usergroup_usergroup_id \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d "{
    \"data\": {
      \"type\": \"user_account_user_account_id_has_usergroup_usergroup_id\",
      \"attributes\": {
        \"user_account_id\": \"$SAM_ID\",
        \"usergroup_id\": \"$SALES_GROUP_ID\"
      }
    }
  }" | jq '{id: .data.id, user: .data.attributes.user_account_id, group: .data.attributes.usergroup_id}'
```

---

### 4.4 Verify Group Membership

Let's verify all users are in their correct groups:

```bash
# Using the API (cleaner output)
echo "User-Group Memberships:"
curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:6336/api/user_account_user_account_id_has_usergroup_usergroup_id?include=user_account_id,usergroup_id | \
  jq -r '.data[] | "\(.attributes.user_account_id) → \(.attributes.usergroup_id)"'
```

Or using SQLite directly:

```bash
echo "User-Group Memberships (readable):"
sqlite3 daptin.db "
SELECT u.name as User, ug.name as UserGroup
FROM user_account_user_account_id_has_usergroup_usergroup_id j
JOIN user_account u ON j.user_account_id = u.id
JOIN usergroup ug ON j.usergroup_id = ug.id
ORDER BY u.name, ug.name;
"
```

**Expected output:**
```
User              | UserGroup
------------------|-----------------
Admin             | administrators
Admin             | users
Marketing Mary    | marketing
Sales Sam         | sales
```

**What this means**:
- Admin is in administrators and users groups ✓
- Mary is in marketing group ✓
- Sam is in sales group ✓

**Quick test - sign in as Mary:**
```bash
MARY_TOKEN=$(curl -s -X POST http://localhost:6336/action/user_account/signin \
  -H "Content-Type: application/json" \
  -d '{"attributes":{"email":"mary@techgear.com","password":"password123"}}' | \
  jq -r '.[] | select(.ResponseType == "client.store.set") | .Attributes.value')

echo "Mary's token (first 30 chars): ${MARY_TOKEN:0:30}..."
```

If you see a token, Mary can sign in successfully! ✓

---

## Step 5: Create Products and Upload Photos

### 5.1 Create a Product with Photo (as Admin)

```bash
# Create a sample product image (base64 encoded)
PHOTO_BASE64=$(echo -n "Product Image Placeholder" | base64)

curl -X POST http://localhost:6336/api/product \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "product",
      "attributes": {
        "name": "Wireless Headphones Pro",
        "price": 149.99,
        "description": "Premium noise-canceling wireless headphones",
        "published": false,
        "photo": [
          {
            "name": "headphones-main.jpg",
            "file": "data:image/jpeg;base64,'$PHOTO_BASE64'",
            "type": "image/jpeg"
          }
        ]
      }
    }
  }' | jq '.data.attributes | {name, price, published, photo}'
```

### 5.2 Share Product Table with Groups (CRITICAL)

**IMPORTANT**: Daptin checks permissions at TWO levels:
1. **Table-level** (world record) - Can the group access this table at all?
2. **Record-level** - Can the group access this specific record?

Both levels must be configured. The table (world) must be shared with groups first.

```bash
# Get the product ID from the response above, or query:
PRODUCT_ID=$(curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:6336/api/product | \
  jq -r '.data[0].id')

echo "Product ID: $PRODUCT_ID"

# Get the world ID for product table
PRODUCT_WORLD_ID=$(curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/world?filter[table_name]=product" | \
  jq -r '.data[0].id')

echo "Product World ID: $PRODUCT_WORLD_ID"

# CRITICAL STEP: Add product TABLE to marketing group
# This allows the marketing group to access the product table at all
# Permission uses bit-shifted format: Group permission at bits 14-20
# 688128 = 42 << 14 (where 42 = Read + Update + Execute)
curl -s -X POST http://localhost:6336/api/world_world_id_has_usergroup_usergroup_id \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d "{
    \"data\": {
      \"type\": \"world_world_id_has_usergroup_usergroup_id\",
      \"attributes\": {
        \"world_id\": \"$PRODUCT_WORLD_ID\",
        \"usergroup_id\": \"$MARKETING_GROUP_ID\"
      }
    }
  }" | jq '.data.id'

# Set permission on the join table (POST ignores permission, must PATCH)
WORLD_MARKETING_JOIN_ID=$(curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:6336/api/world_world_id_has_usergroup_usergroup_id | \
  jq -r ".data[] | select(.attributes.world_id == \"$PRODUCT_WORLD_ID\" and .attributes.usergroup_id == \"$MARKETING_GROUP_ID\") | .id")

curl -s -X PATCH "http://localhost:6336/api/world_world_id_has_usergroup_usergroup_id/$WORLD_MARKETING_JOIN_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d "{
    \"data\": {
      \"type\": \"world_world_id_has_usergroup_usergroup_id\",
      \"id\": \"$WORLD_MARKETING_JOIN_ID\",
      \"attributes\": {
        \"permission\": 688128
      }
    }
  }"

# Add product TABLE to sales group (read-only)
# 32768 = 2 << 14 (where 2 = Read only)
curl -s -X POST http://localhost:6336/api/world_world_id_has_usergroup_usergroup_id \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d "{
    \"data\": {
      \"type\": \"world_world_id_has_usergroup_usergroup_id\",
      \"attributes\": {
        \"world_id\": \"$PRODUCT_WORLD_ID\",
        \"usergroup_id\": \"$SALES_GROUP_ID\"
      }
    }
  }" | jq '.data.id'

WORLD_SALES_JOIN_ID=$(curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:6336/api/world_world_id_has_usergroup_usergroup_id | \
  jq -r ".data[] | select(.attributes.world_id == \"$PRODUCT_WORLD_ID\" and .attributes.usergroup_id == \"$SALES_GROUP_ID\") | .id")

curl -s -X PATCH "http://localhost:6336/api/world_world_id_has_usergroup_usergroup_id/$WORLD_SALES_JOIN_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d "{
    \"data\": {
      \"type\": \"world_world_id_has_usergroup_usergroup_id\",
      \"id\": \"$WORLD_SALES_JOIN_ID\",
      \"attributes\": {
        \"permission\": 32768
      }
    }
  }"

# CRITICAL: Restart server to clear Olric permission cache
./scripts/testing/test-runner.sh stop && ./scripts/testing/test-runner.sh start
./scripts/testing/test-runner.sh token
TOKEN=$(cat /tmp/daptin-token.txt)
```

### 5.3 Share Individual Products with Groups (Optional)

Optionally, you can also set record-level permissions on individual products:

```bash
# Add product record to marketing group
curl -s -X POST http://localhost:6336/api/product_product_id_has_usergroup_usergroup_id \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d "{
    \"data\": {
      \"type\": \"product_product_id_has_usergroup_usergroup_id\",
      \"attributes\": {
        \"product_id\": \"$PRODUCT_ID\",
        \"usergroup_id\": \"$MARKETING_GROUP_ID\"
      }
    }
  }" | jq '.data.id'

# IMPORTANT: POST ignores permission attribute - must PATCH to set it
PRODUCT_MARKETING_JOIN_ID=$(curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:6336/api/product_product_id_has_usergroup_usergroup_id | \
  jq -r ".data[] | select(.attributes.product_id == \"$PRODUCT_ID\" and .attributes.usergroup_id == \"$MARKETING_GROUP_ID\") | .id")

# Set permission: 688128 = 42 << 14 (Read + Update + Execute for group)
curl -s -X PATCH "http://localhost:6336/api/product_product_id_has_usergroup_usergroup_id/$PRODUCT_MARKETING_JOIN_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d "{
    \"data\": {
      \"type\": \"product_product_id_has_usergroup_usergroup_id\",
      \"id\": \"$PRODUCT_MARKETING_JOIN_ID\",
      \"attributes\": {
        \"permission\": 688128
      }
    }
  }"

# Add product record to sales group (read-only: 32768 = 2 << 14)
curl -s -X POST http://localhost:6336/api/product_product_id_has_usergroup_usergroup_id \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d "{
    \"data\": {
      \"type\": \"product_product_id_has_usergroup_usergroup_id\",
      \"attributes\": {
        \"product_id\": \"$PRODUCT_ID\",
        \"usergroup_id\": \"$SALES_GROUP_ID\"
      }
    }
  }" | jq '.data.id'

PRODUCT_SALES_JOIN_ID=$(curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:6336/api/product_product_id_has_usergroup_usergroup_id | \
  jq -r ".data[] | select(.attributes.product_id == \"$PRODUCT_ID\" and .attributes.usergroup_id == \"$SALES_GROUP_ID\") | .id")

curl -s -X PATCH "http://localhost:6336/api/product_product_id_has_usergroup_usergroup_id/$PRODUCT_SALES_JOIN_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d "{
    \"data\": {
      \"type\": \"product_product_id_has_usergroup_usergroup_id\",
      \"id\": \"$PRODUCT_SALES_JOIN_ID\",
      \"attributes\": {
        \"permission\": 32768
      }
    }
  }"
```

---

## Step 6: Create a Custom Action (Publish/Unpublish)

**What we're doing**: Creating a custom action that toggles the `published` status of products.

**Why use actions?**
Actions let you define business logic beyond basic CRUD (Create, Read, Update, Delete). They're perfect for:
- Bulk operations
- Multi-step workflows
- Calculations and transformations
- Triggering external services

**Two ways to create actions**:
1. **Schema file** (recommended) - Loaded on server start
2. **API** (runtime) - Requires server restart after creation

We'll use the **schema file approach** since it's more reliable.

### 6.1 Add Action to Schema File

Actions defined in schema files are loaded automatically when Daptin starts. Let's add our toggle_publish action to the schema file.

```bash
cat >> schema_product.yaml << 'EOF'

Actions:
  - Name: toggle_publish
    Label: Toggle Publish Status
    OnType: product
    InstanceOptional: false
    InFields: []
    OutFields:
      - Type: product
        Method: PATCH
        Attributes:
          reference_id: $subject.reference_id
          published: '!subject.published ? 0 : 1'
EOF
```

**What this does**:
- **Name**: URL identifier (`toggle_publish`)
- **Label**: Display name for UI
- **OnType**: Works on `product` table
- **InstanceOptional**: `false` means it needs a specific product ID
- **InFields**: No input parameters needed (empty array)
- **OutFields**: What the action does (PATCH the product record)

**Understanding the PATCH operation**:
- **Type**: `product` - Update a product record
- **Method**: `PATCH` - Update operation
- **reference_id**: `$subject.reference_id` - Gets the target product's reference ID
- **published**: `'!subject.published ? 0 : 1'` - Toggle logic:
  - If `subject.published` is 1 (truthy), return 0
  - If `subject.published` is 0 (falsy), return 1

**Value substitution syntax** (CRITICAL):
- `$variable_name` - Direct value from previous OutFields or special variables
- `$subject.field_name` - Access fields from the target record (for instance actions)
- `!expression` - JavaScript expression evaluation
  - Inside `!` expressions, use `subject.field_name` to access record fields
  - Use `!subject.price * 1.1` to increase price by 10%
  - Use `!subject.published ? 0 : 1` to toggle boolean values
- `~input_field` - Access values from InFields parameters

### 6.2 Verify Schema File

```bash
cat schema_product.yaml
```

**Expected output**: You should see both the Tables section (from Step 0) and the Actions section you just added.

### 6.3 Restart Server to Load Action

```bash
echo "Stopping server..."
pkill -9 -f "go run main"
sleep 2

echo "Starting server..."
nohup go run main.go > /tmp/daptin.log 2>&1 &
sleep 20

# Get new token
TOKEN=$(curl -s -X POST http://localhost:6336/action/user_account/signin \
  -H "Content-Type: application/json" \
  -d '{"attributes":{"email":"admin@admin.com","password":"adminadmin"}}' | \
  jq -r '.[] | select(.ResponseType == "client.store.set") | .Attributes.value')

echo "$TOKEN" > /tmp/daptin-token.txt
echo "✓ Server restarted and action loaded"
```

### 6.4 Verify Action Was Loaded

```bash
TOKEN=$(cat /tmp/daptin-token.txt)

echo "Checking if toggle_publish action exists:"
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/action?page%5Bsize%5D=100" | \
  jq '.data[] | select(.attributes.action_name == "toggle_publish") | {
    id,
    action_name: .attributes.action_name,
    label: .attributes.label,
    on_type: .attributes.on_type
  }'
```

**Expected output:**
```json
{
  "id": "...",
  "action_name": "toggle_publish",
  "label": "Toggle Publish Status",
  "on_type": "product"
}
```

### 6.5 Test the Action

**Important**: Instance actions (InstanceOptional: false) require the record ID in URL attributes.

```bash
TOKEN=$(cat /tmp/daptin-token.txt)

# Get a product to test with
PRODUCT_ID=$(curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product" | \
  jq -r '.data[0].id')

echo "Testing with product: $PRODUCT_ID"
echo ""

# Check current status
echo "Before toggle:"
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product/$PRODUCT_ID" | \
  jq '{name: .data.attributes.name, published: .data.attributes.published}'

echo ""
echo "Executing toggle_publish..."
curl -s -X POST "http://localhost:6336/action/product/toggle_publish" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"attributes\":{\"product_id\":\"$PRODUCT_ID\"}}" | jq

echo ""
echo "After toggle:"
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/product/$PRODUCT_ID" | \
  jq '{name: .data.attributes.name, published: .data.attributes.published}'
```

**Expected behavior:**
- If `published` was 0, it becomes 1
- If `published` was 1, it becomes 0

---

## Step 7: Test Permissions

### 7.1 Sign In as Marketing User

```bash
# Password is "password123" (the bcrypt hash used during creation)
MARY_TOKEN=$(curl -s -X POST http://localhost:6336/action/user_account/signin \
  -H "Content-Type: application/json" \
  -d '{"attributes":{"email":"mary@techgear.com","password":"password123"}}' | \
  jq -r '.[] | select(.ResponseType == "client.store.set") | .Attributes.value')

echo "Mary's token: ${MARY_TOKEN:0:40}..."
```

### 7.2 Marketing User: Can View Products

```bash
curl -s -H "Authorization: Bearer $MARY_TOKEN" \
  http://localhost:6336/api/product | \
  jq '.data[] | {name: .attributes.name, published: .attributes.published}'
```

### 7.3 Marketing User: Can Update Photos

**Prerequisites**: This works only after completing Step 5.2 (sharing the product TABLE with marketing group) and restarting the server.

```bash
NEW_PHOTO=$(echo -n "Updated Product Photo" | base64)

curl -X PATCH "http://localhost:6336/api/product/$PRODUCT_ID" \
  -H "Authorization: Bearer $MARY_TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "product",
      "id": "'$PRODUCT_ID'",
      "attributes": {
        "description": "Updated by Marketing Mary!"
      }
    }
  }' | jq '.data.attributes.description'
# Expected: "Updated by Marketing Mary!"

# Also test photo update
curl -X PATCH "http://localhost:6336/api/product/$PRODUCT_ID" \
  -H "Authorization: Bearer $MARY_TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "product",
      "id": "'$PRODUCT_ID'",
      "attributes": {
        "photo": [
          {
            "name": "headphones-updated.jpg",
            "file": "data:image/jpeg;base64,'$NEW_PHOTO'",
            "type": "image/jpeg"
          }
        ]
      }
    }
  }' | jq '.data.attributes.photo'
```

### 7.4 Sign In as Sales User

```bash
SAM_TOKEN=$(curl -s -X POST http://localhost:6336/action/user_account/signin \
  -H "Content-Type: application/json" \
  -d '{"attributes":{"email":"sam@techgear.com","password":"password123"}}' | \
  jq -r '.[] | select(.ResponseType == "client.store.set") | .Attributes.value')

echo "Sam's token: ${SAM_TOKEN:0:40}..."
```

### 7.5 Sales User: Can View Products

```bash
curl -s -H "Authorization: Bearer $SAM_TOKEN" \
  http://localhost:6336/api/product | \
  jq '.data[] | {name: .attributes.name, price: .attributes.price}'
```

### 7.6 Sales User: CANNOT Update Products (Permission Denied)

```bash
curl -s -X PATCH "http://localhost:6336/api/product/$PRODUCT_ID" \
  -H "Authorization: Bearer $SAM_TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "product",
      "id": "'$PRODUCT_ID'",
      "attributes": {
        "price": 199.99
      }
    }
  }'
# Expected: 403 Forbidden
```

### 7.7 Guest User: Access Control

**IMPORTANT**: With the default schema (DefaultPermission: 704385), guests have only **Peek** permission (value 1). Peek allows seeing that records exist in lists but NOT reading their full content.

To allow guests to actually read published products, you need to update the product permission to include **Read** (value 2) for guests.

First, publish a product and give it guest read permission:

```bash
# The permission calculation:
# Guest: 3 (Peek + Read)
# Owner: 127 (Full)
# Group: 42 (Read + Update + Execute)
# Formula: 3 | (127 << 7) | (42 << 14) = 3 | 16256 | 688128 = 704387

curl -X PATCH "http://localhost:6336/api/product/$PRODUCT_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "product",
      "id": "'$PRODUCT_ID'",
      "attributes": {
        "published": true,
        "permission": 704387
      }
    }
  }'
```

Now test as guest (no token):

```bash
# Guest can see products with Read permission
curl -s http://localhost:6336/api/product | \
  jq '.data[] | {name: .attributes.name, published: .attributes.published}'

# Guest CANNOT update
curl -s -X PATCH "http://localhost:6336/api/product/$PRODUCT_ID" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{"data":{"type":"product","id":"'$PRODUCT_ID'","attributes":{"price":1}}}'
# Expected: 403 Forbidden
```

**Note**: The `published` field is for your application logic (e.g., filtering in UI). Daptin does NOT automatically filter by this field. To truly hide unpublished products from guests, set their permission to 704385 (Peek only) or 0 (no access), and only give 704387 (with Read) to published products.

---

## Step 8: Verify Files in Cloud Storage

```bash
# Check Minio bucket
docker exec minio mc ls local/techgear-bucket/photos/

# Expected output:
# [2026-01-25 12:00:00 UTC]    26B STANDARD headphones-main.jpg
# [2026-01-25 12:05:00 UTC]    22B STANDARD headphones-updated.jpg
```

---

## Permission Reference Table

| Role | View Products | Upload Photos | Update Price | Execute Actions | Delete |
|------|---------------|---------------|--------------|-----------------|--------|
| Admin | Yes | Yes | Yes | Yes | Yes |
| Marketing | Yes | Yes | No | Yes | No |
| Sales | Yes | No | No | No | No |
| Guest | Published only | No | No | No | No |

---

## Complete Permission Calculation

```javascript
// Permission bits (used for Guest, Owner, and Group)
const PEEK = 1, READ = 2, CREATE = 4, UPDATE = 8, DELETE = 16, EXECUTE = 32, REFER = 64;
const FULL = 127;

// Calculate combined permission value
function permission(guest, owner, group) {
  return guest | (owner << 7) | (group << 14);
}

// For join table permissions (group position only)
function groupPermission(group) {
  return group << 14;
}

// Examples for this walkthrough:
permission(1, 127, 42);   // 704385 - Default product (group can update)
permission(3, 127, 42);   // 704387 - Published product (guest can read)
groupPermission(42);      // 688128 - Marketing group permission on join table
groupPermission(2);       // 32768  - Sales group permission on join table

// Decode a permission value
function decode(perm) {
  return {
    guest: perm & 127,
    owner: (perm >> 7) & 127,
    group: (perm >> 14) & 127
  };
}

decode(704385);  // { guest: 1, owner: 127, group: 42 }
decode(688128);  // { guest: 0, owner: 0, group: 42 }
```

**Key Insight**: When setting permissions on join tables (like `world_world_id_has_usergroup_usergroup_id`), use `groupPermission()` format since only the group bits matter in that context.

---

## Troubleshooting

### "403 Forbidden" When User Should Have Access

**Most Common Cause**: The **world (table)** is not shared with the user's group. Daptin checks TWO levels:
1. **Table-level**: Can this group access the `product` table at all?
2. **Record-level**: Can this group access this specific product?

**Solution**: Add the world record to the group:

```bash
# 1. Check if world-group relationship exists
curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:6336/api/world_world_id_has_usergroup_usergroup_id | \
  jq '.data[] | select(.attributes.world_id == "'$PRODUCT_WORLD_ID'")'

# 2. If missing, add it (see Step 5.2)

# 3. CRITICAL: Restart server to clear Olric permission cache
./scripts/testing/test-runner.sh stop && ./scripts/testing/test-runner.sh start
```

**Other checks:**

1. Check user is in the correct group:
   ```bash
   curl -s -H "Authorization: Bearer $TOKEN" \
     "http://localhost:6336/api/user_account/$USER_ID?include=usergroup_id" | jq
   ```

2. Check record-group relationship:
   ```bash
   curl -s -H "Authorization: Bearer $TOKEN" \
     "http://localhost:6336/api/product_product_id_has_usergroup_usergroup_id" | jq
   ```

3. Check record permission value:
   ```bash
   curl -s -H "Authorization: Bearer $TOKEN" \
     "http://localhost:6336/api/product/$PRODUCT_ID" | jq '.data.attributes.permission'
   ```

### Permission Ignored on POST to Join Tables

**Problem**: When creating a join table record (e.g., `product_product_id_has_usergroup_usergroup_id`), the `permission` attribute is ignored and defaults to 2097151 (full access).

**Solution**: Create the record first, then PATCH to set the permission:

```bash
# Step 1: Create the join record
curl -s -X POST http://localhost:6336/api/product_product_id_has_usergroup_usergroup_id \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{"data":{"type":"product_product_id_has_usergroup_usergroup_id","attributes":{"product_id":"...","usergroup_id":"..."}}}'

# Step 2: Get the join record ID
JOIN_ID=$(curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:6336/api/product_product_id_has_usergroup_usergroup_id | jq -r '.data[0].id')

# Step 3: PATCH to set permission
curl -s -X PATCH "http://localhost:6336/api/product_product_id_has_usergroup_usergroup_id/$JOIN_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{"data":{"type":"product_product_id_has_usergroup_usergroup_id","id":"'$JOIN_ID'","attributes":{"permission":688128}}}'
```

### Permission Values: Bit-Shift Format

**Problem**: Using raw values like 42 or 2 for group permissions doesn't work.

**Solution**: Permissions must be bit-shifted to the correct position:
- Guest permissions: bits 0-6 (no shift needed)
- Owner permissions: bits 7-13 (shift left by 7)
- Group permissions: bits 14-20 (shift left by 14)

```bash
# Examples:
# Group = Read + Update + Execute (42) → 42 << 14 = 688128
# Group = Read only (2) → 2 << 14 = 32768
# Group = Full (127) → 127 << 14 = 2080768
```

### Olric Cache Stale After Permission Changes

**Problem**: After changing permissions, users still get 403 errors.

**Solution**: Restart the server to clear the Olric distributed cache:

```bash
./scripts/testing/test-runner.sh stop && ./scripts/testing/test-runner.sh start
```

The Olric cache has a 10-minute TTL, so alternatively you can wait for it to expire.

### Photo Upload Fails

See the [Cloud Storage Complete Guide](cloud-storage-complete-guide.md) for credential format and linking requirements.

### Action Not Visible or Returns HTML

1. Check action permission includes Execute for the user's role
2. Verify action is linked to the correct world_id
3. **Use correct URL format**: `/action/{table}/{action_name}` with `{table}_id` in attributes
4. Restart server after creating actions

```bash
# CORRECT format:
curl -X POST "http://localhost:6336/action/product/toggle_publish" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"attributes":{"product_id":"PRODUCT_UUID"}}'

# WRONG format (returns HTML):
curl -X POST "http://localhost:6336/action/product/PRODUCT_UUID/toggle_publish"
```

---

## Summary

This walkthrough demonstrated:

1. **Cloud Storage Setup**: Credential + Cloud Store + Relationship Link
2. **User Groups**: Marketing (upload access) and Sales (view only)
3. **Table Permissions**: Default permissions on table creation
4. **Record-Level Permissions**: Sharing specific records with groups
5. **Custom Actions**: Creating executable actions with permission control
6. **Permission Testing**: Verifying access control works correctly

The key insight is that Daptin uses a **three-tier permission model**:
- **Table-level**: Default permissions for new records
- **Record-level**: Override via permission field on each record
- **Group-level**: Fine-grained access via record-group relationships

Files in cloud storage inherit the permission of their parent record - if a user can read the product, they can access its photos.

---

## Critical Learnings

These issues were discovered during testing and are essential for success:

1. **World (table) must be shared with groups**: Before a group can access any records in a table, the table's world record must be added to that group. This is the most common cause of 403 errors.

2. **POST ignores permission on join tables**: When creating join table records (like `product_product_id_has_usergroup_usergroup_id`), the `permission` attribute is ignored. You must PATCH after creation to set permissions.

3. **Bit-shifted permission format**: Group permissions on join tables must use bit-shifted values (e.g., 688128 = 42 << 14), not raw values (e.g., 42).

4. **Server restart clears Olric cache**: After changing permissions, restart the server to clear the Olric distributed cache. Otherwise, stale permissions may cause 403 errors.

5. **Action schema is a single JSON field**: Actions require `action_schema` as a complete JSON object, not separate `in_fields`/`out_fields` attributes.

6. **Action URL format**: Use `/action/{table}/{action_name}` with `{table}_id` in attributes, not `/action/{table}/{record_id}/{action_name}`.

---

## Quick Reference

### Common Commands

```bash
# Get auth token
TOKEN=$(cat /tmp/daptin-token.txt)

# Sign in (get new token)
curl -s -X POST http://localhost:6336/action/user_account/signin \
  -H "Content-Type: application/json" \
  -d '{"attributes":{"email":"admin@admin.com","password":"adminadmin"}}' | \
  jq -r '.[] | select(.ResponseType == "client.store.set") | .Attributes.value'

# List resources
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:6336/api/{resource}
# Examples: /api/product, /api/usergroup, /api/user_account

# Get single resource
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:6336/api/{resource}/{id}

# Create resource
curl -X POST http://localhost:6336/api/{resource} \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{"data":{"type":"{resource}","attributes":{...}}}'

# Update resource
curl -X PATCH http://localhost:6336/api/{resource}/{id} \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{"data":{"type":"{resource}","id":"{id}","attributes":{...}}}'

# Delete resource
curl -X DELETE http://localhost:6336/api/{resource}/{id} \
  -H "Authorization: Bearer $TOKEN"

# Execute custom action
curl -X POST http://localhost:6336/action/{table}/{action_name} \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"attributes":{"{table}_id":"{id}"}}'
```

### Permission Cheat Sheet

```javascript
// Permission bits
Peek    = 1
Read    = 2
Create  = 4
Update  = 8
Delete  = 16
Execute = 32
Refer   = 64
Full    = 127 (all)

// Calculate combined permission
Permission = guest + (owner × 128) + (group × 16384)

// Examples
Guest Peek only:           1 + (127 × 128) + (42 × 16384) = 704385
Guest can Read:            3 + (127 × 128) + (42 × 16384) = 704387
Group Read+Update (42):    42 × 16384 = 688128 (for join tables)
Group Read only (2):       2 × 16384 = 32768 (for join tables)
```

### File Upload Format

```json
{
  "photo": [
    {
      "name": "filename.jpg",
      "file": "data:image/jpeg;base64,/9j/4AAQSkZJRg...",
      "type": "image/jpeg"
    }
  ]
}
```

### Useful Database Queries

```bash
# List all tables
sqlite3 daptin.db ".tables"

# Check users
sqlite3 daptin.db "SELECT id, name, email FROM user_account;"

# Check groups
sqlite3 daptin.db "SELECT id, name FROM usergroup;"

# Check user-group memberships
sqlite3 daptin.db "
SELECT u.name, ug.name
FROM user_account_user_account_id_has_usergroup_usergroup_id j
JOIN user_account u ON j.user_account_id = u.id
JOIN usergroup ug ON j.usergroup_id = ug.id;
"

# Check product permissions
sqlite3 daptin.db "SELECT name, permission FROM product;"

# Check world (table) permissions
sqlite3 daptin.db "SELECT table_name, default_permission FROM world;"
```

### Server Management

```bash
# Start server
nohup go run main.go > /tmp/daptin.log 2>&1 &

# Stop server
pkill -9 -f "go run main"
pkill -9 -f daptin

# View logs
tail -f /tmp/daptin.log

# View errors only
tail -f /tmp/daptin.log | grep -i error

# Check if running
lsof -i :6336
curl -s http://localhost:6336/api/world | head -c 50

# Restart (clean)
pkill -9 -f daptin && \
  pkill -9 -f "go run main" && \
  sleep 2 && \
  nohup go run main.go > /tmp/daptin.log 2>&1 & && \
  sleep 20
```

### Common Errors and Solutions

| Error | Cause | Solution |
|-------|-------|----------|
| 403 Forbidden | World not shared with group | Add world to group (Step 5.2), restart server |
| 403 Forbidden | Stale Olric cache | Restart server |
| NOT NULL constraint | Missing required field | Add all non-nullable columns |
| Permission ignored on POST | Join table limitation | Use POST then PATCH to set permission |
| Action returns HTML | Wrong URL format | Use `/action/{table}/{action}` not `/action/{table}/{id}/{action}` |
| Cloud storage not found | Server not restarted | Restart after creating cloud_store |

### Project Structure

```
daptin/
├── main.go                      # Entry point
├── schema_product.yaml          # Your table schema
├── daptin.db                    # SQLite database (created automatically)
├── /tmp/daptin.log             # Server logs
├── /tmp/daptin-token.txt       # Auth token (for easy reuse)
└── /tmp/product-images/        # Local file storage
    └── photos/                 # Product photos
```

### Next Steps

After completing this walkthrough, you can:

1. **Add more tables**: Create `schema_category.yaml`, `schema_order.yaml`, etc.
2. **Add relationships**: Link products to categories with foreign keys
3. **Create more actions**: Bulk update, import/export, etc.
4. **Use real cloud storage**: Switch from local to S3/GCS for production
5. **Build a frontend**: Use the REST API with React/Vue/Angular
6. **Add webhooks**: Trigger external services on data changes
7. **Explore GraphQL**: Daptin also supports GraphQL queries

### Helpful Resources

- **Daptin Docs**: https://docs.daptin.com
- **API Reference**: http://localhost:6336/api/ (when running)
- **Cloud Storage Guide**: See `cloud-storage-complete-guide.md` in this directory
- **Permission System**: See `wiki/Permissions.md`
- **Actions**: See `wiki/Action-Reference.md`
- **Relationships**: See `wiki/Relationships.md`
