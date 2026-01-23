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
| CRUD Operations | CRUD-Operations.md | 📝 | Needs verification |
| API Overview | API-Overview.md, API-Reference.md | 📝 | Reference exists |
| Filtering/Pagination | Filtering-and-Pagination.md | 📝 | Needs verification |
| Relationships | Relationships.md | 📝 | Needs verification |
| Column Types | Column-Types.md, Column-Type-Reference.md | 📝 | Reference exists |
| Aggregation | Aggregation-API.md | 📝 | Needs verification |
| Schema Definition | Schema-Definition.md | 📝 | Needs verification |

### Authentication & Authorization

| Feature | Wiki File | Status | Notes |
|---------|-----------|--------|-------|
| Basic Auth | Authentication.md | 📝 | Needs verification |
| Users & Groups | Users-and-Groups.md | 📝 | Needs verification |
| Permissions | Permissions.md | 📝 | Needs verification |
| OAuth Providers | - | ❌ | 4 performers exist, no dedicated doc |
| 2FA/OTP | Two-Factor-Auth.md | ✅ | Complete - verified and corrected |
| JWT Tokens | - | ❌ | `jwt.token` performer undocumented |

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
| State Machines | State-Machines.md | 📝 | `smd` table exists |
| Subsites | Subsites.md | 📝 | Complex, needs testing |
| Task Scheduling | Task-Scheduling.md | 📝 | `task` table exists |
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

---

## Undocumented System Tables

| Table | Purpose | Wiki Status |
|-------|---------|-------------|
| `integration` | External service integrations | ❌ Integrations.md is stub |
| `marketplace` | Plugin/extension marketplace | ❌ None |
| `data_exchange` | Import/export jobs | 📝 Data-Exchange.md shallow |
| `template` | Rendering templates | ❌ None |
| `feed` | RSS/Atom feeds | 📝 RSS-Atom-Feeds.md exists |
| `collection` | Grouped items | ❌ None |
| `document` | Document storage | ❌ None |
| `timeline` | Activity timelines | ❌ None |
| `stream` | Data streams | ❌ None |
| `calendar` | Calendar entries | 📝 Only in CalDAV doc |
| `json_schema` | Schema definitions | ❌ None |
| `credential` | Stored credentials | ❌ Mentioned only |

---

## Undocumented Performers

| Performer | Purpose | Priority |
|-----------|---------|----------|
| `$network.request` | Make HTTP requests from actions | High |
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
| `oauth.client.redirect` | OAuth redirect | High |
| `oauth.login.response` | OAuth callback | High |
| `oauth.profile.exchange` | Get OAuth profile | High |
| `oauth.token` | Generate OAuth token | High |
| `world.column.delete` | Delete table column | Low |
| `world.column.rename` | Rename table column | Low |
| `world.delete` | Delete table | Low |
| `column.storage.sync` | Sync column storage | Low |
| `template.render` | Render template | Medium |

---

## Priority Queue

### High Priority
1. [ ] OAuth providers (login with Google/GitHub/etc)
2. [ ] Integrations system
3. [ ] Data import/export lifecycle
4. [ ] Credentials management
5. [ ] `$network.request` performer

### Medium Priority
6. [ ] Templates and rendering
7. [ ] Task scheduling verification
8. [ ] State machines verification
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

---

## Notes

*Add observations during documentation here*

### Process Learnings
- Check `columns.go` for table definitions
- Check `server/actions/` for performers
- Performers are internal - not directly callable via REST
- Actions link tables to performers via OutFields
- Some features need server restart to take effect

### Common Gaps Found
- Wiki says action name X, code says Y
- Parameters documented incorrectly
- Missing prerequisites
- Undocumented error conditions
