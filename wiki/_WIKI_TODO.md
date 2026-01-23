# Daptin Wiki TODO Tracker

## Progress: 45/45 pages created

---

## Getting Started
- [x] Home.md - Main landing page
- [x] Installation.md - Binary, Docker, Kubernetes
- [x] Configuration.md - Env vars, flags, runtime config
- [x] Database-Setup.md - SQLite, MySQL, PostgreSQL

## Core Concepts
- [x] Schema-Definition.md - Tables, columns, relationships
- [x] Column-Types.md - Supported types
- [x] Permissions.md - Linux FS-like model
- [x] Users-and-Groups.md - Auth and authz

## REST API
- [x] API-Overview.md - JSON:API compliance
- [x] CRUD-Operations.md - Create, Read, Update, Delete
- [x] Filtering-and-Pagination.md - Query parameters (FIXED: uses JSON query syntax)
- [x] Relationships.md - Include, loading
- [x] Aggregation-API.md - SQL-like via REST

## Actions (Business Logic)
- [x] Actions-Overview.md - Action system (FIXED: query syntax)
- [x] User-Actions.md - signup, signin, password reset, OTP
- [x] Admin-Actions.md - become_admin, restart, enable_graphql (FIXED: query syntax)
- [x] Data-Actions.md - import, export, csv_to_entity
- [x] Cloud-Actions.md - upload, delete, move, sync
- [x] Email-Actions.md - mail.send, aws.mail.send (FIXED: query syntax)
- [x] Certificate-Actions.md - TLS, ACME
- [x] Custom-Actions.md - Creating actions (FIXED: query syntax)

## Real-time Features
- [x] WebSocket-API.md - Pub/sub, topics
- [x] YJS-Collaboration.md - Document editing
- [x] Event-System.md - Database events

## Communication Protocols
- [x] SMTP-Server.md - Email infrastructure (FIXED: query syntax)
- [x] IMAP-Support.md - Email retrieval
- [x] CalDAV-CardDAV.md - Calendar/contacts
- [x] FTP-Server.md - File transfer
- [x] RSS-Atom-Feeds.md - Feed generation (FIXED: removed bad filter examples)

## Storage
- [x] Cloud-Storage.md - S3, GCS, Dropbox, etc.
- [x] Asset-Columns.md - File columns
- [x] Subsites.md - Multi-site hosting

## Advanced Features
- [x] GraphQL-API.md - Auto-generated schema
- [x] State-Machines.md - FSM workflows (FIXED: query syntax)
- [x] Task-Scheduling.md - Cron jobs (FIXED: query syntax)
- [x] Data-Exchange.md - External APIs
- [x] Integrations.md - Third-party services

## Security
- [x] Authentication.md - JWT, OAuth
- [x] TLS-Certificates.md - HTTPS, Let's Encrypt (FIXED: query syntax)
- [x] Two-Factor-Auth.md - TOTP/OTP
- [x] Encryption.md - Data at rest

## Operations
- [x] Monitoring.md - Stats, health
- [x] Caching.md - Olric
- [x] Rate-Limiting.md - API throttling
- [x] Clustering.md - Multi-node

## Reference
- [x] API-Reference.md - All endpoints
- [x] Action-Reference.md - All actions
- [x] Column-Type-Reference.md - All types detailed

---

## Key Fix Applied:
Changed incorrect `filter=field:value` syntax to correct JSON query syntax:
`query=[{"column":"field","operator":"is","value":"value"}]`

## All Pages Complete
All 45 wiki pages have been created and verified against source code.
