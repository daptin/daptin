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

## Status

- ✅ Complete - Tested, works as documented
- 📝 Needs Work - Exists but incomplete or untested
- ❌ Missing - No documentation

---

## Getting Started

*"How do I start using Daptin?"*

| Guide | Status | Notes |
|-------|--------|-------|
| Installation | 📝 | Basic instructions exist |
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
| Filter & Sort Data | 📝 | In Getting Started, needs own doc |
| Aggregate Data | 📝 | Aggregation-API.md needs testing |

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
| Create Custom Actions | 📝 | Custom-Actions.md needs testing |
| Make HTTP Requests | ✅ | In Custom-Actions.md ($network.request) |
| Validate Data | 📝 | Not documented |
| State Machines | ✅ | State-Machines.md |
| Scheduled Tasks | ✅ | Task-Scheduling.md |

---

## Send Email

*"How do I send and receive email?"*

| Guide | Status | Notes |
|-------|--------|-------|
| Configure SMTP | ✅ | SMTP-Server.md |
| Send Email from Actions | 📝 | Email-Actions.md needs sync |
| Receive Email (IMAP) | ✅ | IMAP-Support.md |

---

## Handle Files

*"How do I upload and store files?"*

| Guide | Status | Notes |
|-------|--------|-------|
| File Columns | ✅ | Asset-Columns.md - base64 inline storage tested |
| Cloud Storage (S3, GCS, etc) | ✅ | Cloud-Storage.md - all actions tested after bug fix |
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
| WebSocket API | 📝 | WebSocket-API.md needs testing |
| GraphQL API | 📝 | GraphQL-API.md needs testing |

---

## Deploy & Operate

*"How do I run Daptin in production?"*

| Guide | Status | Notes |
|-------|--------|-------|
| Configuration Options | 📝 | Configuration.md needs testing |
| Database Setup | 📝 | Database-Setup.md needs testing |
| TLS/HTTPS | 📝 | TLS-Certificates.md needs testing |
| Monitoring | 📝 | Monitoring.md needs testing |

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
7. [ ] WebSocket - real-time subscriptions
8. [ ] Configuration - all env vars/flags

### Nice to Have (advanced)
9. [ ] GraphQL API
10. [ ] CalDAV/CardDAV
11. [ ] FTP Server
12. [ ] Collaborative Editing (YJS)

---

## Recently Completed

| What | When |
|------|------|
| Actions Overview (E2E permission testing, restart requirement documented) | 2026-01-25 |
| Cloud Storage (all actions tested after bug fix) | 2026-01-24 |
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
