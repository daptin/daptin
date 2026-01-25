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
| Configuration Options | 📝 | Server-Configuration.md - config API tested, env vars/flags NOT tested |
| Database Setup | 📝 | Database-Setup.md - connection strings NOT tested |
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
8. [ ] Configuration - config API tested, env vars/flags/ports NOT tested yet

### Nice to Have (advanced)
9. [ ] GraphQL API
10. [ ] CalDAV/CardDAV
11. [ ] FTP Server
12. [ ] Collaborative Editing (YJS)

---

## Recently Completed

| What | When |
|------|------|
| Server Configuration (config API, monitoring endpoints - partial) | 2026-01-25 |
| Custom Actions (complete performer reference, 40+ performers, tested examples) | 2026-01-25 |
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
