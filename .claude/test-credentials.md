# Test Credentials for Documentation

Use these credentials for all API testing.

## Admin User

- **Email**: `admin@admin.com`
- **Password**: `adminadmin`

## Get Token

```bash
TOKEN=$(curl -s -X POST http://localhost:6336/action/user_account/signin \
  -H "Content-Type: application/json" \
  -d '{"attributes":{"email":"admin@admin.com","password":"adminadmin"}}' | \
  jq -r '.[] | select(.ResponseType == "client.store.set") | .Attributes.value')

echo $TOKEN
```

## Quick Test

```bash
# Verify admin access
curl -s http://localhost:6336/api/user_account \
  -H "Authorization: Bearer $TOKEN" | jq '.data | length'
```

## Test Users Created

| Email | Password | Groups |
|-------|----------|--------|
| admin@admin.com | adminadmin | administrators, users |
| newuser@example.com | userpassword123 | users, editors |

## Verified Documentation (2026-01-24)

**Users-and-Groups.md:**
- ✅ Sign in
- ✅ List users (admin)
- ✅ Create user (admin)
- ✅ List groups
- ✅ Create group
- ✅ Add user to group (junction table name correct)
- ✅ OTP registration (register_otp)
- ✅ OTP verification (verify_otp)
- ❌ Password reset (guest): 403 - documented as admin-only

**Asset-Columns.md:**
- ✅ Create table with image column
- ✅ Base64 file upload (inline storage)
- ✅ Retrieve file via API (returns base64)
- ❌ Asset endpoint: Only works with cloud storage
- ❌ Multipart form upload: JSON parsing error

**Cloud-Storage.md (2026-01-24):**
- ✅ Create cloud_store record (local filesystem)
- ✅ create_folder action: Works correctly, creates folders
- ✅ upload_file action: Works after bug fix - file created on disk
- ✅ create_site action: Works after bug fix - site record created in DB
- ✅ move_path action: Works - moves/renames files (note: creates directory if destination doesn't exist)
- ✅ delete_path action: Works - deletes files/folders async

**Bug Fixed (commit 9173037e):** Cloud storage actions crashed when cloud_store had no credential_name configured.
The code tried to access `cred.DataMap` without checking if `cred` is nil first.
Fixed in 6 files by adding `cred != nil &&` check.

**Site Actions:**
- ❌ list_files: Returns "site not found" - site cache built at startup, new sites need restart
- ❌ get_file: Not tested (depends on list_files)

**Notes:**
- Site cache is built at server startup - new sites won't appear until restart
- File operations run asynchronously (success returned immediately, operation completes in background)
- GraphQL needs to be enabled via config before use

**Documents (2026-01-24):**
- ✅ Create document: Works (requires document_name, document_path, document_extension, mime_type)
- ✅ YJS WebSocket endpoint: `/live/document/:referenceId/document_content/yjs` exists
- ❌ Collaborative editing: Requires WebSocket client to test

**State Machines (2026-01-24):**
- ✅ Create state machine definition (smd table)
- ✅ Events JSON format: `[{"Name":"event","Src":["state1"],"Dst":"state2"}]`

**Subsites (2026-01-24):**
- ✅ Create site via create_site action
- ✅ Site record created in database
- ❌ Site serving: Cache built at startup, new sites need restart
- ✅ Upload files to site path via upload_file action

**Other Features:**
- OAuth connect: Empty (needs configuration)
- Mail servers: None configured (SMTP disabled by default)
- JSON schemas: Empty
- Scheduled tasks: Empty
- Feeds: Empty (need to create feed linked to stream)
- Streams: 2 default streams (table, transformed_user)
