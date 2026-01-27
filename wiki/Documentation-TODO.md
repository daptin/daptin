# Documentation Status

Track what's documented and what users still need.

**Rule**: Only mark complete after testing the documented workflow yourself.

---

## Before Testing: Fresh Database

**ALWAYS start with a fresh database when testing documentation.**

```bash
# 1. Stop Daptin (Ctrl+C)
# 2. Delete the database
rm daptin.db

# 3. Restart Daptin
go run main.go
```

This gives you a clean system where:
- No admin exists yet (system is wide open)
- Signup works for guests
- You can test the full user journey

---

## How to Document Properly

**Every wiki page must be based on actual testing, not assumptions.**

### The Process

1. **Test First, Write Second**
   - Run the actual commands yourself
   - Verify the response matches what you document
   - If something doesn't work, investigate the code - don't guess

2. **Include Real Examples**
   - Copy actual curl commands you ran
   - Copy actual responses you received
   - Include error messages users might encounter

3. **When Stuck, Read the Code**
   - Don't document features you haven't verified
   - If the API behaves differently than expected, check the handler
   - Action names, column names, and endpoints must match the code

4. **Mark Tested Examples**
   - Add "**Tested ✓**" at the top of pages with verified examples
   - Include the Daptin version or date of testing
   - Note any prerequisites (admin access, SMTP config, etc.)

### What Makes Bad Documentation

- Documenting features that don't exist
- Copying theoretical examples without testing
- Wrong action names (e.g., `generate_password_reset_otp` vs `generate_password_reset`)
- Wrong table/column names (e.g., `user_usergroup` vs `user_account_user_account_id_has_usergroup_usergroup_id`)
- Claiming capabilities the system doesn't have
- Omitting critical steps (like server restart after schema changes)

### What Makes Good Documentation

- Every example was actually run and verified
- Error scenarios are documented with real error messages
- Caching behavior and restart requirements are noted
- Prerequisites are listed upfront
- The user can follow step-by-step and succeed

### When Features Don't Work

If you find a feature that doesn't work as expected:
1. Check if it's a bug or intentional behavior
2. Document the actual behavior, not the expected behavior
3. Add a troubleshooting section with workarounds
4. File an issue if it's a bug

---

## Status

- ✅ Complete - Tested, works as documented
- 📝 Needs Work - Exists but incomplete or untested
- ❌ Missing - No documentation

---

## Getting Started

*"How do I start using Daptin?"*

| Guide | Status | Notes |
|-------|--------|-------|
| Installation | ✅ | THOROUGHLY TESTED 2026-01-27: Build from source (197MB binary), all command flags, SQLite/MySQL/PostgreSQL, Docker (SQLite/persistent/MySQL/PostgreSQL), Docker Compose, environment variables (PORT, LOG_LEVEL, TZ, DB_TYPE, DB_CONNECTION_STRING). Fixed: Docker port mapping (6336:8080 not 6336:6336), persistent storage path (/data not /opt/daptin), health endpoint docs, added image tag requirement (v0.9.82), MySQL 8.0 issues (use MariaDB 10.11) |
| First Admin Setup | ✅ | In Getting-Started-Guide.md |
| Create Your First Table | ✅ | In Getting-Started-Guide.md |
| CRUD Operations | ✅ | Rewritten, removed false claims (transactions, wrong action names) |

---

## Build Your Data Model

*"How do I define my data?"*

| Guide | Status | Notes |
|-------|--------|-------|
| Define Tables (Schema) | ✅ | Schema-Definition.md |
| Column Types | ✅ | Column-Types.md, Column-Type-Reference.md |
| Link Tables (Relationships) | ✅ | Relationships.md |
| Filter & Sort Data | ✅ | Filtering-and-Pagination.md - Complete with all operators, sorting, pagination, fuzzy search tested |
| Aggregate Data | ✅ | Aggregation-API.md - All basic features tested. Known issues: HAVING clause and POST method not working |

---

## Control Access

*"Who can see and edit my data?"*

| Guide | Status | Notes |
|-------|--------|-------|
| Understanding Permissions | ✅ | Permissions.md |
| Managing Users | ✅ | Users-and-Groups.md - fixed junction table names, action names |
| Creating User Groups | ✅ | Users-and-Groups.md |
| Sharing Records with Groups | ✅ | In Permissions.md |
| Re-enabling Public Signup | ✅ | In Getting-Started-Guide.md |

---

## User Authentication

*"How do users log in?"*

| Guide | Status | Notes |
|-------|--------|-------|
| Email/Password Login | ✅ | Authentication.md |
| JWT Tokens | ✅ | Authentication.md |
| Two-Factor Auth (2FA) | ✅ | Two-Factor-Auth.md |
| OAuth (Google, GitHub, etc) | ✅ | Authentication.md |
| Password Reset | ✅ | Users-and-Groups.md - requires admin access, SMTP |

---

## Add Business Logic

*"How do I add custom behavior?"*

| Guide | Status | Notes |
|-------|--------|-------|
| What Are Actions? | ✅ | Actions-Overview.md - rewritten for end users |
| Create Custom Actions | ✅ | Custom-Actions.md - complete performer reference, tested examples |
| Make HTTP Requests | ✅ | In Custom-Actions.md ($network.request) |
| Validate Data | ✅ | In Custom-Actions.md (Validations section) |
| State Machines | ✅ | State-Machines.md |
| Scheduled Tasks | ✅ | Task-Scheduling.md |

---

## Send Email

*"How do I send and receive email?"*

| Guide | Status | Notes |
|-------|--------|-------|
| Configure SMTP | ✅ | SMTP-Server.md |
| Send Email from Actions | ✅ | Email-Actions.md - Complete rewrite with tested examples. Bug fixed in mail.send performer (type assertion). Documented performers vs actions, credential storage pattern, working examples for mail.send and aws.mail.send |
| Receive Email (IMAP) | ✅ | IMAP-Support.md |

---

## Handle Files

*"How do I upload and store files?"*

| Guide | Status | Notes |
|-------|--------|-------|
| File Columns | ✅ | Asset-Columns.md - inline and cloud storage tested |
| Cloud Storage (S3, GCS, etc) | ✅ | Cloud-Storage.md - CRUD + file operations working with correct URL format |
| Serve Static Sites | ✅ | Subsites.md - site creation tested, cache needs restart |

---

## Connect External Services

*"How do I integrate with other APIs?"*

| Guide | Status | Notes |
|-------|--------|-------|
| Install OpenAPI Integration | ✅ | Integrations.md |
| Store API Credentials | ✅ | Credentials.md |
| Import/Export Data | ✅ | Data-Actions.md |

---

## Real-Time Features

*"How do I get live updates?"*

| Guide | Status | Notes |
|-------|--------|-------|
| WebSocket API | ✅ | Tested 2026-01-26 - All methods working, 69 topics available |
| YJS Collaboration | ✅ | Tested 2026-01-26 - Both direct and file column endpoints working |
| GraphQL API | ✅ | Tested 2026-01-26 - Works correctly including subscriptions |
| FTP Server | ✅ | Tested 2026-01-26 - Site-based access, FTPS/TLS, all operations verified |

---

## Deploy & Operate

*"How do I run Daptin in production?"*

| Guide | Status | Notes |
|-------|--------|-------|
| Configuration Options | ✅ | All command-line flags, environment variables, and runtime config tested. MySQL/PostgreSQL noted as requiring external setup |
| Database Setup | ✅ | MySQL/MariaDB and PostgreSQL tested with Docker. Connection strings verified. Documented in Server-Configuration.md |
| TLS/HTTPS | ✅ | Tested 2026-01-26 - Complete with self-signed and ACME workflows |
| Monitoring | ✅ | Tested 2026-01-26 - All endpoints verified, profiling documented |

---

## Priority

### Must Have (blocks users)
1. [x] CRUD Operations - rewritten, verified against code
2. [x] Users & Groups - fixed junction tables, action names, removed fake features
3. [x] Password Reset - documented in Users-and-Groups.md
4. [x] File Upload - inline (Asset-Columns) and cloud (Cloud-Storage) tested

### Should Have (common use cases)
5. [x] Actions Overview - rewritten for end users
6. [x] Cloud Storage - S3/GCS setup (all actions tested after bug fix)
7. [x] WebSocket - tested and verified working (2026-01-26)
8. [x] YJS - tested and verified working (2026-01-26)
9. [x] Configuration - all flags, env vars, and runtime config tested (2026-01-26)

### Nice to Have (advanced)
9. [x] GraphQL API - tested 2026-01-26
10. [x] CalDAV/CardDAV - tested 2026-01-26
11. [x] FTP Server - tested 2026-01-26

---

## Recently Completed

| What | When | Key Learnings |
|------|------|---------------|
| Installation (comprehensive testing and documentation) | 2026-01-27 | **Docker Port Mapping Critical**: Container listens on 8080 internally, must map `6336:8080` NOT `6336:6336` or API won't be accessible. **Docker Image Tag Required**: No `latest` tag exists, must use `daptin/daptin:v0.9.82` explicitly. **Persistent Storage Path Wrong**: Docs showed `-v /path:/opt/daptin` which overwrites binary and fails. Correct: `-v /path:/data` with `DAPTIN_DB_CONNECTION_STRING=/data/daptin.db`. **MySQL 8.0 Fails**: OOM errors during container init. Use MariaDB 10.11 instead (fully compatible). **All Databases Work**: SQLite, MySQL/MariaDB, PostgreSQL all tested successfully with Docker. **Docker Compose Works**: Tested with corrected port mapping and volume path. **Build from Source**: Binary ~200MB, Go 1.24.3, no errors. **Storage Directory Required**: Must create `./storage/yjs-documents` before first run. **Health Endpoint Misleading**: `/health` returns HTML dashboard not health check - use `/statistics` for monitoring. **Olric Port 5336**: Must be free or fatal "failed to create olric topic" error. **Environment Variables**: DAPTIN_PORT, LOG_LEVEL, TZ, DB_TYPE, DB_CONNECTION_STRING all verified working. |
| Cloud Storage Actions (create_folder, upload_file, move/delete) | 2026-01-27 | **URL Format Critical**: GitHub #166 was about documentation error, not broken code. Actions work with correct format: `/action/{type}/{action_name}?{type}_id={id}` NOT `/action/{type}/{id}/{action_name}`. **Query Parameter Required**: Instance ID must be passed as query parameter (e.g., `?cloud_store_id=$ID`), not in URL path or body. **Async Operations**: create_folder and upload_file execute asynchronously in goroutines. **Bugs Found**: delete_path returns success but doesn't delete; move_path creates directory instead of renaming file. **Testing Confirms Code Works**: All performers execute correctly when called with proper URL format. |
| CalDAV/CardDAV (basic WebDAV file storage for calendars/contacts) | 2026-01-26 | **NOT Full Protocol**: Implements basic WebDAV file storage only, NOT full CalDAV/CardDAV (missing REPORT, calendar-query). **Storage Directories Required**: Must create `./storage/caldav/` and `./storage/carddav/` or get 404 errors. **Configuration**: Disabled by default, enable via `_config/backend/caldav.enable`, requires restart. **Both Auth Methods**: Supports Bearer token (JWT) and Basic auth (email:password). **WebDAV Methods**: All core WebDAV methods work (PROPFIND, GET, PUT, DELETE, MKCOL, COPY, MOVE, PROPPATCH). **File Formats**: Stores .ics (iCalendar) and .vcf (vCard) files. **Client Compatibility**: May NOT work with standard clients (Apple Calendar, Thunderbird) that expect full CalDAV/CardDAV protocol features. **Use Cases**: Good for simple file storage/backup, NOT suitable for production calendar server. **No Multi-User Isolation**: All users share same storage directory. **Code**: Uses `github.com/emersion/go-webdav` library, `webdav.LocalFileSystem("./storage")` backend. |
| FTP Server (site-based file access with FTPS/TLS) | 2026-01-26 | **Conditional Startup**: FTP server ONLY starts if sites with `ftp_enabled=true` exist. No sites = no FTP port listening. **Configuration**: Requires `ftp.enable=true` in _config AND at least one FTP-enabled site. **Site Directory Structure**: Root directory lists sites as subdirectories; each maps to `{cloud_store.root_path}/{site.path}/`. **LIST Quirk**: Directory listings may appear empty but files are accessible via direct RETR. **Authentication**: Uses Daptin user accounts (email/password). **FTPS/TLS**: Automatic using site certificates. **Port**: Default 2121 (non-standard). **Restart Required**: After creating FTP-enabled sites or changing ftp.enable config. **Dependencies**: Requires cloud_store → site → ftp_enabled chain. |
| Monitoring (health checks, statistics, profiling) | 2026-01-26 | **Structure Corrections**: CPU uses `counts` not `count`, process.count is integer not object, disk.io not disk.ioCounters. **HTTP pprof doesn't exist**: Daptin uses file-based profiling via `-runtime=profile` flag, not HTTP endpoints. **Endpoint Clarifications**: /health returns admin HTML UI not simple health check, /meta returns empty body, /api/_config returns HTML not config data. **Statistics are comprehensive**: 8 sections including detailed temperature sensors (29 on macOS M1!), per-core CPU utilization, database connection pool stats, web server metrics. |
| Email Actions (mail.send, aws.mail.send, custom actions) | 2026-01-26 | **Critical Bug Fixed**: mail.send had hard type assertion causing panic with YAML arrays. Used `GetValueAsArrayString()` helper. **Performers vs Actions**: mail.send/aws.mail.send are NOT direct REST endpoints - must use in custom actions' OutFields. **Credential Pattern**: aws.mail.send uses credential NAME to lookup stored credentials, not inline keys. **No fake features**: mail.send doesn't support contentType, attachments, cc, bcc (docs claimed this). |
| Aggregation API (count, sum, avg, min, max, GROUP BY, filters, ORDER BY) | 2026-01-26 | All basic aggregations work correctly via GET method. All filter operators tested and working (eq, not, lt, lte, gt, gte, in, notin). GROUP BY and ORDER BY work perfectly. **Known bugs**: HAVING clause generates correct SQL but returns empty results (bug in result processing). POST method fails with "empty identifier" error. Use GET method for all queries. |
| Server Configuration (env vars, flags, HTTPS, MySQL, PostgreSQL, Olric) | 2026-01-26 | Tested all flags/env vars. MySQL (MariaDB 10.11) and PostgreSQL 15 fully working with 50 concurrent connections. Olric clustering has bug: PubSub topic creation fails with "no available client found". Single-node Olric works. HTTPS requires cert generation + enable_https config. |
| Documentation Process Meta-Guide | 2026-01-26 | **CRITICAL**: Always check server logs vs client errors; use protocol-appropriate testing tools; search git history for usage examples; read auth middleware for each protocol; don't assume features are broken - verify testing approach first |
| WebSocket API (tested and verified working, all 6 methods documented) | 2026-01-26 | Use proper clients for protocol, check server logs for actual responses, auth mechanisms vary by protocol, found examples in dadadash repo |
| YJS Collaboration (tested and verified working, both endpoints documented) | 2026-01-26 | Check dadadash git history for usage examples including commented-out code |
| GraphQL API (tested from previous session) | 2026-01-26 | Real-time features may use different auth mechanisms |
| Asset Columns (inline and cloud storage, correct array format) | 2026-01-25 |
| Server Configuration (config API, port, log_level, runtime, schema folder) | 2026-01-25 |
| Custom Actions (complete performer reference, 40+ performers, tested examples) | 2026-01-25 |
| Actions Overview (E2E permission testing, restart requirement documented) | 2026-01-25 |
| Subsites (site creation, file upload) | 2026-01-24 |
| Users & Groups (fixed junction tables, removed fake features) | 2026-01-24 |
| CRUD Operations (removed false claims) | 2026-01-24 |
| Getting Started (user journeys) | 2026-01-24 |
| Permissions (admin-first locking) | 2026-01-24 |
| State Machines | 2026-01-24 |
| Task Scheduling | 2026-01-24 |
| Authentication (JWT, OAuth, 2FA) | 2026-01-24 |
| Email (SMTP/IMAP) | 2026-01-24 |
| Integrations | 2026-01-24 |
