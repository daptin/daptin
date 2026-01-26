# Wiki Reorganization Plan

**Date**: 2026-01-26
**Based on**: Walkthrough testing verification (Steps 0-7)
**Goal**: Transform 60+ fragmented wiki pages into predictable end-user manual

---

## Problems Identified

### 1. Organization is Feature-First, Not User-First
**Current**: Organized by technology (REST API, GraphQL, WebSocket, FTP, etc.)
**Problem**: Users don't think "I need WebSocket" - they think "I need real-time updates"

### 2. Critical Troubleshooting Information is Scattered
From testing, these issues are EVERYWHERE in different pages:
- Olric cache (port 5336) mentioned in Getting-Started-Guide.md
- API filtering mentioned nowhere
- Two-level permissions buried in Permissions.md
- Password handling not consolidated
- Server restart requirements scattered

### 3. No Clear "How Do I..." Section
Users ask:
- "How do I upload files to S3?"
- "How do I restrict access to a table?"
- "How do I send an email?"

But must piece together from Schema-Definition.md + Cloud-Storage.md + Asset-Columns.md + restart notes

### 4. Reference vs Guide Confusion
**Column-Types.md** vs **Column-Type-Reference.md** vs **Schema-Definition.md**
All cover similar ground. Which do users read first?

### 5. 60+ Files is Too Many
Users can't remember what's where. Navigation requires constant sidebar reference.

---

## New Structure: User Journey-Based

### Tier 1: Getting Started (Must Read, 5 pages)
```
1. Installation.md           → "Download and run Daptin"
2. First-Admin-Setup.md      → "Claim admin on fresh install" (NEW - extract from Getting-Started-Guide.md)
3. Create-Your-First-Table.md → "Define a simple schema, create records" (NEW - beginner-focused)
4. Understanding-Permissions.md → "Who can see what" (NEW - simplified from Permissions.md)
5. Common-Errors.md          → "Troubleshooting 90% of issues" (NEW)
```

### Tier 2: Essential Guides (Common Tasks, 8 pages)
```
6. Working-With-Users.md     → Users, groups, authentication (merge Users-and-Groups.md + Authentication.md basics)
7. File-Uploads.md           → Cloud storage end-to-end (merge Cloud-Storage.md + Asset-Columns.md)
8. Controlling-Access.md     → Permissions deep dive (expand Permissions.md with two-level system)
9. Custom-Logic.md           → Actions overview (simplify Actions-Overview.md)
10. Filtering-Data.md        → Queries, sorts, pagination (consolidate Filtering-and-Pagination.md)
11. Table-Relationships.md   → Foreign keys, joins (keep Relationships.md)
12. Sending-Email.md         → SMTP setup + email actions (merge SMTP-Server.md + Email-Actions.md)
13. Real-Time-Updates.md     → WebSocket + YJS basics (merge WebSocket-API.md + YJS-Collaboration.md)
```

### Tier 3: Advanced Features (8 pages)
```
14. State-Machines.md        → FSM workflows (keep)
15. Task-Scheduling.md       → Cron jobs (keep)
16. GraphQL.md               → GraphQL API (simplify GraphQL-API.md)
17. Two-Factor-Auth.md       → 2FA/OTP (keep)
18. Data-Import-Export.md    → Bulk operations (keep Data-Actions.md)
19. External-Integrations.md → Third-party APIs (keep Integrations.md)
20. Multi-Tenancy.md         → Subsites (keep Subsites.md)
21. TLS-Setup.md             → HTTPS certificates (merge TLS-Certificates.md + Certificate-Actions.md)
```

### Tier 4: Complete Reference (5 pages)
```
22. API-Complete-Reference.md     → All endpoints (merge API-Overview.md + API-Reference.md + CRUD-Operations.md + GET-API-Complete-Reference.md)
23. Schema-Complete-Reference.md  → All TableInfo properties (merge Schema-Definition.md + Schema-Reference-Complete.md + Schema-Examples.md)
24. Actions-Complete-Reference.md → All built-in actions (merge Action-Reference.md + User-Actions.md + Admin-Actions.md + Cloud-Actions.md)
25. Column-Types-Reference.md     → All 41 types (merge Column-Types.md + Column-Type-Reference.md)
26. Configuration-Reference.md    → Env vars, flags (merge Configuration.md + Server-Configuration.md)
```

### Tier 5: Operations (5 pages)
```
27. Monitoring-and-Stats.md  → Health checks, metrics (keep Monitoring.md)
28. Database-Setup.md        → SQLite, MySQL, PostgreSQL (keep)
29. Clustering.md            → Multi-node (keep)
30. Performance-Tuning.md    → Caching, rate limiting (merge Caching.md + Rate-Limiting.md)
31. Security-Checklist.md    → Production hardening (NEW - consolidate security best practices)
```

### Tier 6: Specialized Protocols (Keep but De-Emphasize, 5 pages)
```
32. IMAP-Email.md            → Email retrieval (keep IMAP-Support.md)
33. CalDAV-CardDAV.md        → Calendar/contacts (keep)
34. FTP-Server.md            → File transfer (keep)
35. RSS-Feeds.md             → Feed generation (keep RSS-Atom-Feeds.md)
36. Event-System.md          → Database events (keep)
```

### Tier 7: Meta/Internal
```
37. Contributing.md                → For contributors
38. Documentation-Guide.md         → Doc standards (keep)
39. WIKI_AUDIT_REPORT.md          → Internal tracking (keep)
```

**Total: 39 pages (down from 60+)**
**Reduction: 21 pages eliminated via consolidation**

---

## Critical New Pages to Create

### 1. Common-Errors.md (CRITICAL)

Based on walkthrough testing, this page addresses 90% of user issues:

**Must Include**:
- Olric cache (port 5336) - "Unauthorized" on become_an_administrator
- API filtering behavior - "Why don't I see all tables?"
- Two-level permissions - "Why 403 even after setting permissions?"
- Password handling - API auto-hashes, don't pre-hash
- Server restart requirements - When it's needed
- Filter query syntax - JSON array, not filter[field]
- POST ignores permission on joins - Must PATCH after
- Token expiration - How to refresh
- Credential linking - Must use relationship PATCH
- File upload format - Array of objects with name, file, type

**Format**:
```markdown
## Symptom → Root Cause → Solution

### "Unauthorized" on become_an_administrator (Fresh Database)

**Symptom**:
```json
{"ResponseType": "client.notify", "Attributes": {"message": "Unauthorized"}}
```

**Root Cause**: Stale Olric cache on port 5336 from old Daptin process

**Solution**:
```bash
lsof -i :5336 -t | xargs kill -9 2>/dev/null || true
./scripts/testing/test-runner.sh start
```

**Prevention**: Always use test-runner.sh for server lifecycle
```

### 2. First-Admin-Setup.md

Extract from Getting-Started-Guide.md, focus ONLY on:
- Why first user is special
- Signup → Signin → become_an_administrator flow
- What changes after admin exists
- How to verify admin status
- Recovery if you lose admin access

### 3. Create-Your-First-Table.md

Beginner-focused walkthrough:
- Simplest possible schema (1 table, 3 columns)
- How to load schema (file vs API)
- Verify table creation
- Create a record via curl
- Read it back
- Update it
- Delete it
- Link to deeper docs for more

### 4. Understanding-Permissions.md

Simplified intro (save deep dive for Controlling-Access.md):
- Three roles: Guest, Owner, Group
- Seven permissions: Peek, Read, Create, Update, Delete, Execute, Refer
- Default behaviors (before/after admin)
- How to check permissions
- Two sentences: "For production use, read Controlling-Access.md"

### 5. Security-Checklist.md

Production deployment checklist:
- Disable public signup (or don't)
- Set up TLS certificates
- Configure rate limiting
- Set strong admin password
- Restrict guest permissions
- Enable audit logging
- Configure CORS properly
- Review action permissions
- Set up monitoring

---

## Migration Plan

### Phase 1: Create Critical New Pages (Immediate)
1. Write Common-Errors.md (90% of support issues)
2. Write First-Admin-Setup.md
3. Write Create-Your-First-Table.md
4. Write Understanding-Permissions.md
5. Write Security-Checklist.md

**Why first**: These fill the biggest gaps found during testing

### Phase 2: Consolidate References (Week 1)
1. Merge Column-Types.md + Column-Type-Reference.md → Column-Types-Reference.md
2. Merge API-Overview.md + API-Reference.md + CRUD-Operations.md → API-Complete-Reference.md
3. Merge Schema files → Schema-Complete-Reference.md
4. Merge Action files → Actions-Complete-Reference.md
5. Update all internal links to point to new files

### Phase 3: Create Essential Guides (Week 2)
1. Working-With-Users.md (merge Users-and-Groups.md + auth basics)
2. File-Uploads.md (merge Cloud-Storage.md + Asset-Columns.md)
3. Controlling-Access.md (expand Permissions.md with two-level system)
4. Custom-Logic.md (simplify Actions-Overview.md)
5. Filtering-Data.md (consolidate query docs)
6. Sending-Email.md (merge SMTP + Email-Actions)
7. Real-Time-Updates.md (merge WebSocket + YJS)

### Phase 4: Archive Old Pages (Week 3)
1. Create `wiki/archive/` directory
2. Move old fragmented pages to archive
3. Keep redirects in place (stub files with "→ See New-Page.md")
4. Update all links
5. Test all examples

### Phase 5: Update Navigation (Week 3)
1. Rewrite _Sidebar.md with new structure
2. Update Home.md "I want to..." section with new page links
3. Update all cross-references
4. Add "Prerequisites" section to advanced pages

---

## New _Sidebar.md Structure

```markdown
## Getting Started
- [[Installation]]
- [[First-Admin-Setup]] ⭐
- [[Create-Your-First-Table]] ⭐
- [[Understanding-Permissions]]
- [[Common-Errors]] 🔧

## Essential Guides
- [[Working-With-Users]]
- [[File-Uploads]]
- [[Controlling-Access]]
- [[Custom-Logic]]
- [[Filtering-Data]]
- [[Table-Relationships]]
- [[Sending-Email]]
- [[Real-Time-Updates]]

## Advanced Features
- [[State-Machines]]
- [[Task-Scheduling]]
- [[GraphQL]]
- [[Two-Factor-Auth]]
- [[Data-Import-Export]]
- [[External-Integrations]]
- [[Multi-Tenancy]]
- [[TLS-Setup]]

## Complete Reference
- [[API-Complete-Reference]]
- [[Schema-Complete-Reference]]
- [[Actions-Complete-Reference]]
- [[Column-Types-Reference]]
- [[Configuration-Reference]]

## Operations
- [[Monitoring-and-Stats]]
- [[Database-Setup]]
- [[Clustering]]
- [[Performance-Tuning]]
- [[Security-Checklist]]

## Specialized Protocols
- [[IMAP-Email]]
- [[CalDAV-CardDAV]]
- [[FTP-Server]]
- [[RSS-Feeds]]
- [[Event-System]]

---

🔧 = Troubleshooting
⭐ = Must Read
```

---

## Documentation Principles Going Forward

### 1. Every Page Must Answer "Why" and "How"
**Bad**: "The signup action has these parameters..."
**Good**: "You need to create user accounts. Here's how..."

### 2. Show Complete Examples, Not Snippets
**Bad**: `curl -X POST /api/entity -d '...'`
**Good**: Full curl with headers, token, response, what to do next

### 3. Lead With Common Use Case
**Bad**: "The permission system uses bit-shifted integers..."
**Good**: "Make your todo list public: `curl -X PATCH ...`"

### 4. Troubleshooting in Every Guide
Each guide should end with:
- Common errors for this feature
- How to verify it's working
- Link to Common-Errors.md for more

### 5. Prerequisites Are Explicit
**Bad**: Assumes you have admin token
**Good**: "Prerequisites: Admin access (see First-Admin-Setup.md), Token in /tmp/daptin-token.txt"

### 6. One Primary Path, Reference Alternatives
**Bad**: "You can do this with API or schema or direct DB insert..."
**Good**: "Use schema files (recommended). Alternative: API method (see reference)"

### 7. Testing-Based Documentation
From walkthrough testing:
- Write the guide
- Test EVERY command
- Document ACTUAL output (not expected)
- Add errors encountered to Common-Errors.md
- Update guide based on what actually worked

---

## Success Metrics

### Before Reorganization
- 60+ pages
- Users must read 4-6 pages to complete one task
- "Unauthorized" error not documented
- API filtering behavior not documented
- Two-level permissions buried

### After Reorganization
- 39 pages
- Users read 1 page to complete one task
- 90% of issues covered in Common-Errors.md
- Every behavior discovered in testing is documented
- Clear path: Getting Started → Essential Guides → Advanced → Reference

---

## Implementation Priority

### Week 1 (Do Now)
1. ✅ Create Common-Errors.md
2. ✅ Create First-Admin-Setup.md
3. ✅ Update Home.md to link to new pages
4. ✅ Update Getting-Started-Guide.md to point to new structure

### Week 2
1. Create Create-Your-First-Table.md
2. Create Understanding-Permissions.md
3. Create Security-Checklist.md
4. Merge 4-5 most-used guide pairs

### Week 3
1. Complete Essential Guides section
2. Consolidate Reference docs
3. Update _Sidebar.md
4. Archive old pages

### Week 4
1. Test all examples end-to-end
2. Fix broken links
3. Add cross-references
4. Final review

---

## Open Questions

1. **File Organization**: Keep flat `wiki/*.md` or create subdirectories `wiki/guides/`, `wiki/reference/`, etc.?
   - **Recommendation**: Keep flat for GitHub wiki compatibility

2. **Page Naming**: Dash-case or Title-Case-With-Dashes?
   - **Current**: Title-Case-With-Dashes.md
   - **Recommendation**: Keep current for consistency

3. **Redirect Strategy**: Leave stub files or rely on wiki search?
   - **Recommendation**: Stub files for 6 months, then remove

4. **Version History**: Keep old audit reports?
   - **Recommendation**: Yes, shows documentation evolution

---

## Next Steps

**Immediate (Today)**:
1. Create Common-Errors.md with all findings from walkthrough testing
2. Create First-Admin-Setup.md extracted from Getting-Started-Guide.md
3. Update Home.md "I want to..." section with links to these

**This Week**:
1. Get user feedback on new Common-Errors.md
2. Start consolidating most-used references
3. Create Create-Your-First-Table.md

**This Month**:
1. Complete Essential Guides section
2. Consolidate references
3. Archive old pages
4. Test end-to-end

---

*Created based on comprehensive walkthrough testing (Steps 0-7) and user question: "is your wiki docs fragmented and unusable"*
