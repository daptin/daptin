# Documentation TODO

Track documentation progress for all Daptin features.

---

## Status Legend

- ✅ Complete - Full lifecycle documented and verified
- 🔄 In Progress - Being documented
- 📝 Partial - Exists but needs verification/depth
- ❌ Missing - No documentation
- ⏸️ Blocked - Needs investigation first

---

## Features

### Core Data & API

| Feature | Wiki File | Status | Notes |
|---------|-----------|--------|-------|
| Core Concepts | Core-Concepts.md | ✅ | Foundation doc with interlinking |
| Column Types | Column-Types.md, Column-Type-Reference.md | ✅ | Verified against column_types.go |
| Schema Definition | Schema-Definition.md | ✅ | Fixed standard columns |
| CRUD Operations | CRUD-Operations.md | 📝 | Needs verification |
| API Overview | API-Overview.md, API-Reference.md | 📝 | Reference exists |
| Filtering/Pagination | Filtering-and-Pagination.md | 📝 | Needs verification |
| Relationships | Relationships.md | ✅ | Verified with API testing |
| Aggregation | Aggregation-API.md | 📝 | Needs verification |

### Authentication & Authorization

| Feature | Wiki File | Status | Notes |
|---------|-----------|--------|-------|
| Authentication | Authentication.md | ✅ | JWT, OAuth, 2FA overview documented |
| Users & Groups | Users-and-Groups.md | 📝 | Needs verification |
| Permissions | Permissions.md | ✅ | Verified: bit structure, check order, join table permissions |
| OAuth Providers | Authentication.md | ✅ | Documented oauth_connect/oauth_token tables, flow, allow_login |
| 2FA/OTP | Two-Factor-Auth.md | ✅ | Complete - verified and corrected |
| JWT Tokens | Authentication.md | ✅ | Documented jwt.token action and config |

### Mail System

| Feature | Wiki File | Status | Notes |
|---------|-----------|--------|-------|
| SMTP Server | SMTP-Server.md | ✅ | Complete |
| IMAP Support | IMAP-Support.md | ✅ | Complete |
| Email Actions | Email-Actions.md | 📝 | Needs sync with SMTP doc |

### Storage & Files

| Feature | Wiki File | Status | Notes |
|---------|-----------|--------|-------|
| Cloud Storage | Cloud-Storage.md | 📝 | Has code, needs testing |
| Asset Columns | Asset-Columns.md | 📝 | Needs verification |
| FTP Server | FTP-Server.md | 📝 | `endpoint_ftp.go` exists |

### Actions System

| Feature | Wiki File | Status | Notes |
|---------|-----------|--------|-------|
| Actions Overview | Actions-Overview.md | 📝 | Needs verification |
| Custom Actions | Custom-Actions.md | 📝 | Needs verification |
| Action Reference | Action-Reference.md | 📝 | Large file, needs audit |
| Admin Actions | Admin-Actions.md | 📝 | Needs verification |
| Data Actions | Data-Actions.md | 📝 | Needs verification |
| Certificate Actions | Certificate-Actions.md | 📝 | Needs verification |
| Cloud Actions | Cloud-Actions.md | 📝 | Needs verification |
| User Actions | User-Actions.md | 📝 | Needs verification |

### Advanced Features

| Feature | Wiki File | Status | Notes |
|---------|-----------|--------|-------|
| GraphQL | GraphQL-API.md | 📝 | Endpoint exists, needs testing |
| State Machines | State-Machines.md | ✅ | Verified against code - smd table, looplab/fsm, state endpoints |
| Subsites | Subsites.md | 📝 | Complex, needs testing |
| Task Scheduling | Task-Scheduling.md | ✅ | Verified against code - robfig/cron, task table, execution flow |
| WebSocket | WebSocket-API.md | 📝 | Needs verification |
| Event System | Event-System.md | 📝 | Needs verification |

### Collaboration & Sync

| Feature | Wiki File | Status | Notes |
|---------|-----------|--------|-------|
| YJS Collaboration | YJS-Collaboration.md | 📝 | `endpoint_yjs.go` exists |
| CalDAV/CardDAV | CalDAV-CardDAV.md | 📝 | `endpoint_caldav.go` exists |

### Infrastructure

| Feature | Wiki File | Status | Notes |
|---------|-----------|--------|-------|
| Installation | Installation.md | 📝 | Needs verification |
| Configuration | Configuration.md | 📝 | Needs verification |
| Database Setup | Database-Setup.md | 📝 | Needs verification |
| TLS Certificates | TLS-Certificates.md | 📝 | Needs verification |
| Clustering | Clustering.md | 📝 | Needs verification |
| Monitoring | Monitoring.md | 📝 | Needs verification |
| Rate Limiting | Rate-Limiting.md | 📝 | Needs verification |
| Caching | Caching.md | 📝 | Needs verification |
| Encryption | Encryption.md | 📝 | Needs verification |

### Meta Documentation

| Feature | Wiki File | Status | Notes |
|---------|-----------|--------|-------|
| Documentation Guidelines | Documentation-Guidelines.md | ✅ | Admin setup, testing rules |
| Getting Started | Getting-Started-Guide.md | ✅ | Fixed admin bootstrapping |

---

## Undocumented System Tables

| Table | Purpose | Wiki Status |
|-------|---------|-------------|
| `integration` | External service integrations | ✅ Verified against code |
| `marketplace` | Plugin/extension marketplace | ❌ None |
| `data_exchange` | Import/export jobs | ✅ Data-Exchange.md + Data-Actions.md verified |
| `template` | Rendering templates | ❌ None |
| `feed` | RSS/Atom feeds | 📝 RSS-Atom-Feeds.md exists |
| `collection` | Grouped items | ❌ None |
| `document` | Document storage | ❌ None |
| `timeline` | Activity timelines | ❌ None |
| `stream` | Data streams | ❌ None |
| `calendar` | Calendar entries | 📝 Only in CalDAV doc |
| `json_schema` | Schema definitions | ❌ None |
| `credential` | Stored credentials | ✅ Credentials.md created |

---

## Undocumented Performers

| Performer | Purpose | Priority |
|-----------|---------|----------|
| `$network.request` | Make HTTP requests from actions | ✅ Documented in Custom-Actions.md |
| `$transaction` | Database transactions in actions | Medium |
| `command.execute` | Execute system commands | Medium |
| `generate.random.data` | Generate test data | Low |
| `response.create` | Create custom action responses | Medium |
| `cloudstore.file.upload` | Upload to cloud storage | High |
| `cloudstore.folder.create` | Create cloud folder | Medium |
| `cloudstore.path.move` | Move cloud files | Medium |
| `cloudstore.site.create` | Create site from cloud | Medium |
| `site.file.get` | Get site file | Medium |
| `site.file.list` | List site files | Medium |
| `site.file.delete` | Delete site file | Medium |
| `site.storage.sync` | Sync site storage | Medium |
| `oauth.client.redirect` | OAuth redirect | ✅ Documented in Authentication.md |
| `oauth.login.response` | OAuth callback | ✅ Documented in Authentication.md |
| `oauth.profile.exchange` | Get OAuth profile | ✅ Documented in Authentication.md |
| `oauth.token` | Generate OAuth token | ✅ Documented in Authentication.md |
| `world.column.delete` | Delete table column | Low |
| `world.column.rename` | Rename table column | Low |
| `world.delete` | Delete table | Low |
| `column.storage.sync` | Sync column storage | Low |
| `template.render` | Render template | Medium |

---

## Priority Queue

### High Priority
1. [x] OAuth providers (login with Google/GitHub/etc) - Documented in Authentication.md
2. [x] Integrations system - Verified against code, install_integration action
3. [x] Data import/export lifecycle - Fixed action URLs, removed non-existent actions
4. [x] Credentials management - Created Credentials.md
5. [x] `$network.request` performer - Documented in Custom-Actions.md

### Medium Priority
6. [ ] Templates and rendering
7. [x] Task scheduling verification - Complete
8. [x] State machines verification - Complete
9. [ ] Feeds (RSS/Atom)
10. [ ] Cloud storage performers

### Low Priority
11. [ ] YJS collaboration
12. [ ] CalDAV/CardDAV
13. [ ] FTP server
14. [ ] Marketplace
15. [ ] Schema modification performers

---

## Completed

| Feature | Date | Commit |
|---------|------|--------|
| Mail (SMTP/IMAP) | 2026-01-24 | Full lifecycle |
| 2FA/OTP | 2026-01-24 | Verified and corrected |
| Core Concepts | 2026-01-24 | Foundation doc |
| Column Types | 2026-01-24 | Verified against code |
| Schema Definition | 2026-01-24 | Fixed standard columns |
| Documentation Guidelines | 2026-01-24 | Admin setup, testing rules |
| Getting Started (admin) | 2026-01-24 | Fixed bootstrapping workflow |
| Relationships | 2026-01-24 | Verified with API, fixed FK column names |
| Permissions | 2026-01-24 | Verified bit structure, check order, join table permissions |
| OAuth Providers | 2026-01-24 | Documented tables, flow, allow_login in Authentication.md |
| Authentication | 2026-01-24 | JWT, OAuth, 2FA overview, WebSocket auth, password reset |
| Integrations | 2026-01-24 | OpenAPI install, auth types, dynamic actions |
| Data Actions | 2026-01-24 | Fixed action URLs, removed non-existent actions |
| Credentials | 2026-01-24 | New wiki page for secure credential storage |
| $network.request | 2026-01-24 | HTTP request performer in Custom-Actions.md |
| Wiki Audit Report | 2026-01-24 | All critical issues resolved (90% complete) |
| State Machines | 2026-01-24 | Complete rewrite - smd table, endpoints, looplab/fsm |
| Task Scheduling | 2026-01-24 | Complete rewrite - task table, robfig/cron, execution flow |

---

## Notes

*Add observations during documentation here*

### Process Learnings
- Check `columns.go` for table definitions AND action definitions (SystemActions array)
- Check `server/actions/` for performers (internal executors)
- **Critical distinction:**
  - **Action names** = REST API endpoints (e.g., `register_otp`, `generate_self_certificate`)
  - **Performer names** = Internal executors used in OutFields (e.g., `otp.generate`, `self.tls.generate`)
- Performers are internal - not directly callable via REST
- Actions link tables to performers via OutFields
- Some features need server restart to take effect
- Certificate actions have OnType=`certificate`, not `world`

### Critical: Admin Setup for Documentation Sessions
**ALWAYS set up admin FIRST before testing protected features**

1. Sign up: `POST /action/user_account/signup`
2. Sign in: `POST /action/user_account/signin`
3. Become admin: `POST /action/world/become_an_administrator` (NOT user_account!)
4. Wait for restart, re-signin

See [Documentation-Guidelines.md](Documentation-Guidelines.md) for full setup script.

**Common mistake**: `/action/user_account/become_an_administrator` fails with "no reference id" - the action is on `world` table, not `user_account`.

### Common Gaps Found
- Wiki says action name X, code says Y
- Parameters documented incorrectly
- Missing prerequisites
- Undocumented error conditions
