# Daptin Documentation Testing Methodology

## Quick Start

```bash
# 1. Check server
.claude/testing/test-runner.sh check

# 2. Start if needed
.claude/testing/test-runner.sh start

# 3. Get token
.claude/testing/test-runner.sh token

# 4. Test API
.claude/testing/test-runner.sh get /api/user_account
```

---

## Configuration

### Environment
| Variable | Default | Description |
|----------|---------|-------------|
| `DAPTIN_HOST` | `http://localhost:6336` | API endpoint |
| Database | `daptin.db` | SQLite file in project root |
| Log file | `/tmp/daptin.log` | Server logs |
| Token cache | `/tmp/daptin-token.txt` | Cached auth token |

### Default Admin Credentials
```
Email:    admin@admin.com
Password: adminadmin
```

These are created automatically on first signup if no admin exists.

### Database Location
- **Development**: `./daptin.db` (SQLite)
- **Fresh start**: Delete `daptin.db` and restart server

### Ports
| Port | Service |
|------|---------|
| 6336 | HTTP API |
| 5336 | Olric cache |
| 21 | FTP (if enabled) |
| 993 | IMAP (if enabled) |

---

## Testing Rules

### 1. Always Use Test Runner
```bash
# GOOD
.claude/testing/test-runner.sh action cloud_store upload_file '{"cloud_store_id":"..."}'

# BAD - complex one-liners cause parsing issues
TOKEN=$(curl...) && curl ... | jq ...
```

### 2. Use --max-time for All Curls
All curl commands must have timeout:
```bash
curl -s --max-time 5 --connect-timeout 5 ...
```

### 3. Check Server Before Testing
```bash
.claude/testing/test-runner.sh check
# If "stopped", run:
.claude/testing/test-runner.sh start
```

### 4. One Command Per Line
```bash
# GOOD
.claude/testing/test-runner.sh check
.claude/testing/test-runner.sh token

# BAD
.claude/testing/test-runner.sh check && .claude/testing/test-runner.sh token
```

---

## Test Runner Commands

### Server Management
```bash
.claude/testing/test-runner.sh check    # Check if running
.claude/testing/test-runner.sh start    # Start server (waits for ready)
.claude/testing/test-runner.sh stop     # Stop server
```

### Authentication
```bash
.claude/testing/test-runner.sh token    # Get fresh JWT token
```

### API Calls
```bash
# GET request
.claude/testing/test-runner.sh get /api/user_account

# POST request
.claude/testing/test-runner.sh post /api/entity '{"data":{...}}'

# Action call
.claude/testing/test-runner.sh action cloud_store upload_file '{"cloud_store_id":"..."}'
```

### Debugging
```bash
.claude/testing/test-runner.sh logs      # Last 20 lines
.claude/testing/test-runner.sh logs 50   # Last 50 lines
.claude/testing/test-runner.sh errors    # Show errors only
```

---

## Testing Workflow

### Phase 1: Discover Feature

1. **Find action definition** in `server/resource/columns.go`:
   ```bash
   grep -n "upload_file" server/resource/columns.go
   ```

2. **Identify parameters** - Look for `InFields` array

3. **Find entity type** - Look for `OnType` field

### Phase 2: Test Feature

1. **Start server**
   ```bash
   .claude/testing/test-runner.sh start
   ```

2. **Run test**
   ```bash
   .claude/testing/test-runner.sh action entity action_name '{"param":"value"}'
   ```

3. **Record result** in `.claude/test-credentials.md`:
   - ✅ Working
   - ❌ Broken (include error)
   - 📝 Partial

### Phase 3: Document Feature

1. **Update wiki page** with correct examples

2. **Update** `wiki/Documentation-TODO.md`

3. **Commit**
   ```bash
   git add wiki/ .claude/
   git commit -m "docs: document feature X"
   ```

---

## API Patterns

### Action Endpoints
```
POST /action/{entity}/{action_name}
Body: {"attributes": {...}}
```

### CRUD Endpoints
```
GET    /api/{entity}           # List
GET    /api/{entity}/{id}      # Read
POST   /api/{entity}           # Create
PATCH  /api/{entity}/{id}      # Update
DELETE /api/{entity}/{id}      # Delete
```

### Content Types
| Endpoint | Content-Type |
|----------|--------------|
| `/action/*` | `application/json` |
| `/api/*` (CRUD) | `application/vnd.api+json` |

---

## Common Actions by Entity

### cloud_store
| Action | Description |
|--------|-------------|
| `upload_file` | Upload file to storage |
| `create_folder` | Create folder |
| `delete_path` | Delete file/folder |
| `move_path` | Move/rename |
| `create_site` | Create subsite |

### site
| Action | Description |
|--------|-------------|
| `list_files` | List files |
| `get_file` | Get file content |
| `sync_storage` | Sync from cloud |

### user_account
| Action | Description |
|--------|-------------|
| `signin` | Login |
| `signup` | Register |
| `register_otp` | Enable 2FA |
| `verify_otp` | Verify 2FA |

---

## File Locations

| File | Purpose |
|------|---------|
| `.claude/testing/test-runner.sh` | Test automation |
| `.claude/testing/METHODOLOGY.md` | This document |
| `.claude/test-credentials.md` | Test results |
| `wiki/Documentation-TODO.md` | Doc status |
| `/tmp/daptin.log` | Server logs |
| `daptin.db` | SQLite database |

---

## Troubleshooting

### Server Won't Start
```bash
# Kill stale processes
pkill -9 -f daptin
lsof -i :5336 | awk 'NR>1 {print $2}' | xargs kill -9

# Check errors
.claude/testing/test-runner.sh errors
```

### Request Hangs
```bash
# Kill stale curls
pkill -9 -f "curl.*6336"

# Restart server
.claude/testing/test-runner.sh stop
.claude/testing/test-runner.sh start
```

### Database Locked
```bash
# Stop server and wait
.claude/testing/test-runner.sh stop
sleep 5
.claude/testing/test-runner.sh start
```

### Fresh Database
```bash
.claude/testing/test-runner.sh stop
rm daptin.db
.claude/testing/test-runner.sh start
```

---

## Commit Format

```
<type>: <description>

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
```

Types: `fix:`, `docs:`, `feat:`, `test:`
