# Daptin v0.13.14 Feature Status

This page is the release-specific authority for what was exercised, what is
experimental, and what must not be presented as production-ready. It records a
hands-on audit completed on 2026-09-16 against the released v0.13.14 binary,
container, source, and published commands.

Status means:

- **Verified**: the stated flow completed in the audit environment.
- **Experimental**: a useful subset worked, but durability, compatibility, or
  failure behavior is incomplete.
- **Known broken**: a reproduced defect makes the documented production claim
  unsafe.
- **Not exercised**: code or documentation exists, but this audit did not prove
  the behavior.

Code presence is not verification. An HTTP 200 is not verification when work
continues asynchronously; the audit checked the durable/provider result where
possible.

## Status matrix

| Capability | Status | v0.13.14 evidence and boundary |
|---|---|---|
| SQLite CRUD, filtering, pagination, aggregation | Verified | Development and single-node flows exercised |
| PostgreSQL 15 | Verified | Restart persistence, outage/recovery, two shared-DB nodes, and 50 concurrent creates |
| MySQL/MariaDB initialization | **Known broken** | MariaDB 10.11 omitted required tables after column-size and identifier-length errors while `/ready` stayed 200 |
| Relationships | Verified with corrected syntax | For `belongs_to`, set the FK-side `ObjectName`; do not define both naming sides |
| Table and row permissions | Verified | Both gates are required; action execute permission does not bypass resource/credential permissions |
| Public signup shutdown | **Known unsafe unless explicitly tested** | A second unauthenticated signup succeeded after bootstrap in the audit; explicitly lock the action and assert 403 |
| Custom actions and state transitions | Verified | Transport HTTP status can differ from the embedded action/state outcome |
| Scheduled tasks | Verified with execution user | `as_user_id` is mandatory; general cluster-wide singleton ownership is unverified |
| Data import action | **Known broken** | Instance/world route mismatch and contradictory successful/failed reporting with zero imported rows |
| Data export | Verified | JSON/CSV/basic export exercised |
| Audit writes | Experimental | Rows were written; generated audit-resource reads failed when a join table was absent |
| Local cloud store | Verified | Basic local operations exercised |
| MinIO/S3-compatible store | Experimental | Upload/delete worked only with relationship plus `credential_name`; provider failures may still return HTTP 200 |
| FTP/explicit FTPS | **Experimental, non-durable** | Protocol operations worked only against the temporary site directory; writes are not synced back |
| Static subsites/templates | Verified locally | Host routing worked; cache invalidation and multi-node semantics require deployment testing |
| CalDAV/CardDAV | **Known broken for writes** | Authentication/PROPFIND worked; documented MKCOL/PUT flow returned 500/404 |
| SMTP ingestion and IMAP STARTTLS | Verified | Message persisted and was fetched from standard mailboxes in the audit setup |
| Direct-MX delivery | Verified with DNS caveat | Valid MX delivered with DKIM; a no-MX lookup error did not fall back to an A record |
| TLS for matching SNI hostname | Verified | Client must request a hostname covered by a stored site certificate |
| ACME | Experimental/production-only | Let's Encrypt production directory is hardcoded; no staging or custom CA |
| REST numeric fields | Verified | Fractional resource values work through the resource API where the SQL type supports them |
| GraphQL basic/nested reads | Verified | A Daptin `float` was observed as GraphQL `Int`; decimals were rejected |
| OAuth/OIDC provider flow | Verified | Discovery, JWKS, dynamic registration, code+PKCE, token, userinfo, introspection, revocation |
| OpenAPI integration install/call | Verified | Installation, operation discovery, and invocation exercised |
| WebSockets and cross-node PubSub | Verified | Event published through one node reached a client on another |
| Global route rate limiting | Verified | Versioned JSON, one-second window, Olric counters/fallback, headers and JSON 429 |
| API metering hard limit | Verified | Two successful requests then 402; quota `reserved=0`, `consumed=2`, usage `state=completed` |
| Ollama via OpenAI-compatible LLM gateway | Experimental | Non-streaming and SSE worked; `max_tokens` was not enforced as expected |
| `/ready` dependency recovery | Experimental | Correctly followed PostgreSQL availability, but does not prove required-schema initialization |
| `/statistics` | Functional but sensitive | Unauthenticated in the tested default and exposes host/runtime/database details |
| Backup/restore, upgrade/rollback, failover, soak, secret rotation | Not exercised | Required before an enterprise production claim |

## Production blockers for this release

Do not claim enterprise production readiness for v0.13.14 without resolving or
explicitly fencing these boundaries:

1. Use PostgreSQL 15; do not use the known-broken MySQL/MariaDB initialization
   path.
2. Keep first bootstrap private, explicitly remove `GuestExecute` from signup,
   and prove an unauthenticated signup is rejected.
3. Run a resource-level schema preflight after startup. `/ready` checks the
   runtime gate and database connectivity, not completeness of required tables.
4. Do not publish through FTP; write the durable backing store and then sync
   into the site's temporary directory.
5. Treat asynchronous cloud action responses as acknowledgements and verify
   provider objects.
6. Restrict `/statistics` at the ingress/private network.
7. Fence general scheduled/background jobs to one node unless their durable
   idempotency or lease behavior has been demonstrated. Outbox has a verified
   per-item Olric NX claim; that does not establish ownership for other jobs.
8. Do not use CalDAV writes, `import_data`, audit reads, or GraphQL fractional
   writes as critical production paths in this release.

## Production preflight

Before routing traffic, automate all of the following and fail deployment on
any mismatch:

- Confirm the pinned Daptin and PostgreSQL versions and the intended database
  DSN/SSL mode.
- Require clean startup logs and authenticated reads of required built-in and
  application resources, including their relationship endpoints.
- Create/read/update/delete a permissioned canary through JSON:API; do not use
  direct SQL as an application workflow.
- Assert public signup rejection and table-plus-row permission behavior using a
  guest, regular user, and administrator.
- Confirm stable JWT/encryption secrets are supplied from the secret manager;
  verify encrypted credential content is not plaintext without printing it.
- Verify certificate association and SNI with the public hostname.
- Verify every cloud credential relationship and `credential_name`, then
  create/list/download/delete a canary object.
- Verify mail MX/DKIM, relay policy, outbox terminal state, and retry alerts if
  mail is enabled.
- Verify Olric data and membership reachability, routable advertised addresses,
  cluster membership, and a cross-node PubSub event.
- Verify singleton fencing for every enabled task/background job.
- Confirm only accepted `schema_*.(json|yaml|yml)` files were loaded from the
  intended working/schema directories.
- Check `/ping`, `/ready`, the LLM-specific readiness endpoint when enabled,
  and a schema preflight independently.

## Operations contract still required

The following are release qualification requirements, not implied features:

- **Backup and disaster recovery:** back up SQL, the encryption secret,
  external object/site content, certificates/private keys, and schema files as
  one recovery set. Restore into an isolated environment in that order and
  verify decrypted credentials, SNI, resource permissions, and canary CRUD.
- **Upgrade and rollback:** pin versions, document supported hops, prohibit
  untested mixed-version clusters, take a verified database/secret backup
  before migration, and state when rollback is impossible after schema change.
- **High availability:** use a reverse proxy with `/ready`, two or more Daptin
  nodes, routable Olric data/membership ports, PostgreSQL HA, shared external
  object storage, explicit job fencing, and graceful SIGTERM drain tests.
- **Capacity and limits:** measure database pool limits, request/upload sizes,
  WebSocket queues, subsite cache entries, mail worker/retry behavior, and
  database identifier/column limits for the chosen release and dependencies.
- **Observability:** centralize structured logs and request identifiers, alert
  on readiness, schema preflight, task/action terminal failures, outbox retries,
  quota exhaustion, provider failures, and Olric membership changes. Daptin
  v0.13.14 has no native Prometheus/OpenTelemetry endpoint.
- **Retention:** define and test cleanup for `api_usage`, `api_quota`, audit and
  timeline rows, OAuth codes/tokens, mail/outbox, data-exchange executions,
  temporary site directories, caches, and expired OTP state.
- **Security hardening:** close signup, replace permissive bootstrap/default
  permissions, protect statistics and Olric, use secret injection, enforce
  matching TLS/SNI, constrain SMTP/FTP/CORS, add ingress brute-force controls,
  and test log redaction.

## Conformance suite backlog

A reproducible documentation suite should provision PostgreSQL 15, MariaDB
10.11 (as a negative regression target), MinIO, Mailpit plus controlled DNS/MX,
Ollama, and two Daptin nodes. It should execute the literal wiki commands with
real protocol clients, save HTTP/protocol statuses and redacted logs, assert
resource/provider state, and fail CI when commands or expected outcomes drift.
Backup/restore, version upgrade/rollback, network partition, PostgreSQL
failover, object-store outage/retry, mail retry/bounce, YJS convergence,
payment-webhook replay, large-asset/range behavior, secret rotation, and public
endpoint security remain explicit unverified cases.
