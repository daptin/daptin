# Go test workflow

The executable and its package tests live in `cmd/daptin`. Tests for a server
package stay beside that package's code. The root directory holds project
configuration and documentation.

## Commands

```bash
make quicktest          # ordinary Go tests; real E2E tests skip
make test-e2e-smoke     # 3-node PostgreSQL and Olric cluster, also run in CI
make test-e2e           # all cmd/daptin tests with E2E in the test name
```

`test-e2e-smoke` starts PostgreSQL in Docker and three Daptin nodes sharing
that database and Olric peer configuration. It checks readiness and signs in
the same administrator on every node, then stops the cluster. For an
interactive local cluster, run `scripts/testing/cluster-test-runner.sh bootstrap`
and later `scripts/testing/cluster-test-runner.sh stop`. The nodes listen on
ports 16336, 16338, and 16340.

`test-e2e` runs isolated Go test cases and can take time. Tests needing an
external PostgreSQL, S3, or provider service check their own environment
variables and skip when it is absent. CI runs the cluster smoke test without those
services. The `live` provider tests and capacity benchmarks have separate
build tags or environment flags.

## Writing an HTTP E2E test

Use the shared setup in `cmd/daptin/e2e_harness_test.go`:

```go
func TestExampleRealE2E(t *testing.T) {
    fixture := startDaptinE2E(t, transportE2EDaptinOptions{schema: exampleSchema})
    token := fixture.SignupAdmin(t)
    response := transportE2EPostJSON(t, fixture.Client,
        fixture.URL+"/api/example", token, payload)
    // Assert the resource response and downstream effects here.
    _ = response
}
```

The fixture chooses distinct HTTP, HTTPS, and Olric ports, creates an isolated
SQLite database and storage directory, waits for readiness, and stops the
process through `t.Cleanup`. It also skips unless `DAPTIN_REAL_E2E=1`. Use
the lower-level `startTransportE2EDaptin` only for restarts, multi-node tests,
or a database shared across processes. Keep setup through Daptin resources and
relationships; do not insert fixture data with SQL.

The older `server_test.go`, WebSocket, and Yjs tests share an in-process server
on port 6337. They remain in the ordinary suite; use the isolated fixture for
new HTTP E2E coverage.

For a focused run while developing:

```bash
DAPTIN_REAL_E2E=1 go test -count=1 -run '^TestExampleRealE2E$' ./cmd/daptin
```

## Document processing acceptance

`TestDocumentActionsRealE2E` runs the published document schema on an empty
database, including private-record conversion, REST/GraphQL, revocation,
scheduled execution and metering ownership. `TestDocumentActionsQualificationRealE2E`
adds the five-format matrix (including expected unsupported pairs), PPTX native
part preservation and loss checks, malformed inputs, wire/decoded/output/archive
limits, independent Poppler PDF reading and the pinned Manifold corpus.
`TestDocumentActionsQuotaRealE2E` proves failed conversions release quota and
exhausted quota denies work before parsing or writes.

```bash
go build -o /tmp/daptin-document-test ./cmd/daptin
DAPTIN_REAL_E2E=1 DAPTIN_E2E_BINARY=/tmp/daptin-document-test \
DAPTIN_DOCUMENT_CORPUS_DIR="$(go list -m -f '{{.Dir}}' github.com/artpar/manifold)/testdata/corpus" \
DAPTIN_E2E_MAIL_POSTGRES_BIN=/path/to/postgresql/bin \
go test ./cmd/daptin -run '^TestDocumentActions' -count=1 -v -timeout=10m
```

The PostgreSQL setting reuses the existing private-cluster fixture; it never
connects to an installed database. `TestDocumentActionsConcurrencyPostgresRealE2E`
uses it to test shared admission, disconnect recovery and the real 30-second
deadline. SQLite serializes database transactions, so renderer saturation is
qualified on PostgreSQL. Corpus and PostgreSQL checks skip when their settings
are absent; install Poppler for the independent PDF reader checks.

## Browser Mail protocol acceptance

`TestMailNativeRoundTripRealE2E` reuses the shared executable harness with a
fresh database (SQLite by default), HTTP-created resources and a restarted loopback-only
IMAP listener. It uses STARTTLS and verifies the hostname against a
fixture-generated certificate in the test client's private trust pool.
It exercises MIME/UID/flags,
native drafts and HTTP adoption, COPY/EXPUNGE, subscriptions, empty bodies,
APPEND bounds and live permission revocation. It also checks that EXAMINE
advertises empty permanent flags, that ordinary BODY FETCH does not set
Seen in the read-only selection, and that CLOSE does not expunge it.
Regular SELECT with read-only account or mailbox grants must also report
read-only. The same connection can still fetch bytes without setting Seen
when a write grant is revoked at the mailbox or message level:

```bash
DAPTIN_REAL_E2E=1 DAPTIN_E2E_BINARY=/path/to/candidate \
  go test ./cmd/daptin -run '^TestMailNativeRoundTripRealE2E$' -count=1
```

Set `DAPTIN_E2E_MAIL_SEARCH_SCALE=1` to append 320 synthetic messages, including
16 with binary MIME attachments, and time broad and scoped native searches.
This verifies every returned UID set and prints five timed samples per search
after one warmup. It does not impose machine-dependent latency thresholds.

On macOS, also set `DAPTIN_E2E_MUTT_BINARY` to the absolute path of a Mutt
build with IMAP and OpenSSL. The optional case drives Mutt through the system
`expect` terminal, checks that it reads the multilingual draft using STARTTLS,
then checks the changes to Seen and Flagged through Daptin HTTP on the same
mailbox UID. Mutt starts with an isolated configuration, local directory and
only the fixture's public certificate. Use a terminfo database containing
`xterm` if the host terminal sets a custom `TERMINFO`.

The same IMAP case can use a disposable Thunderbird profile on macOS. Set
`DAPTIN_E2E_THUNDERBIRD_DRIVER` to the absolute path of Office's
`scripts/check-mail-thunderbird.py` and `MAIL_QUALIFICATION_THUNDERBIRD_APP` to
the Thunderbird executable inside a verified app bundle. The driver requires
NSS `certutil` and imports only the temporary CA into its profile. It requires
STARTTLS, renders the multilingual draft using Thunderbird's message view and
sets Seen and Flagged through Thunderbird. Daptin HTTP must observe both
changes on the same UID. Thunderbird also identifies Draft and Sent by role,
appends a mixed plain-text draft with a binary attachment, and uses its online
search session to find the draft by a custom header. The driver requires the
matching command in Thunderbird's IMAP trace. Daptin HTTP checks the returned
UID and decoded attachment, then adopts the draft without changing the
canonical reference or mailbox. Editing the MIME allocates a replacement UID.
Finally, Thunderbird moves a flagged
multipart fixture from Inbox to Archive through its own move service. The
driver checks that Thunderbird actually sent UID MOVE, removed the Inbox
entry, and rendered the message at its Archive UID. Daptin HTTP compares that
UID and the original canonical reference, flags and exact MIME bytes.
The driver also opens Thunderbird's compose window and saves a new draft with
a local binary file, then removes the original file and reopens the draft in
the compose window. Daptin's authorized unpack action checks its mailbox UID,
multilingual text, To and Bcc, and exact attachment bytes. No SMTP send runs.
This option runs in place of the optional Mutt case.

`TestMailLoopbackSMTPAndCrashRealE2E` binds an ephemeral unprivileged
**127.0.0.1** SMTP port and configures the disposable server through the normal
administrator-only `/_config/backend/outbox.smtp_port` setting. Recipients use
`localhost`, so no custom DNS server, privileged port or container is needed.
The production default remains port 25. Invalid port configuration is rejected
before claiming queued messages. The test captures To/Bcc envelopes and signed
MIME, checks `smtp_accepted`, qualifies the real 30-second final-acknowledgment
deadline/uncertainty/no-redelivery path, then holds another acknowledgment and abruptly
kills the disposable worker and checks that restart preserves `uncertain`
without redelivery:

```bash
DAPTIN_REAL_E2E=1 DAPTIN_E2E_BINARY=/path/to/candidate \
  go test ./cmd/daptin -run '^TestMailLoopbackSMTPAndCrashRealE2E$' -count=1
```

This SMTP test is opt-in and never uses a live installation or external
recipient. Successful compilation or an ordinary skipped run is not SMTP
acceptance. Results are recorded in the canonical Office Mail plan.

`TestMailInboundSMTPRealE2E` exercises the receiving side. It provisions two
private mail accounts through the resource APIs, restarts a disposable
server with an unprivileged loopback SMTP listener, and sends one unsigned MIME
message to two local envelope recipients. The listener creates the Inbox and
message records. The test verifies sparse owner-scoped metadata, UID allocation,
decoded attachment bytes, canonical source after SMTP DATA line-ending
normalization, cross-user denial, and independent unread state.

```bash
DAPTIN_REAL_E2E=1 DAPTIN_E2E_BINARY=/path/to/candidate \
  go test ./cmd/daptin -run '^TestMailInboundSMTPRealE2E$' -count=1
```

For the same three protocol scenarios on PostgreSQL, supply the absolute
path to its local `bin` directory:

```bash
DAPTIN_E2E_MAIL_POSTGRES_BIN=/path/to/postgresql/bin \
DAPTIN_REAL_E2E=1 DAPTIN_E2E_BINARY=/path/to/candidate \
  go test ./cmd/daptin -v -count=1 \
    -run '^(TestMailNativeRoundTripRealE2E|TestMailInboundSMTPRealE2E|TestMailLoopbackSMTPAndCrashRealE2E)$'
```

Each scenario initializes a new UTF-8 cluster and uses its private Unix
socket across Daptin restarts. PostgreSQL has no TCP listener and requires
no installed database or external test DSN. The fixture stops the cluster
and removes its files on cleanup.

## Administrator restart in Docker

Build the candidate Linux binary and package it with the repository's production
`Dockerfile`. Point the opt-in tests at that local image:

```bash
DAPTIN_REAL_E2E=1 DAPTIN_E2E_DOCKER_IMAGE=daptin/admin-restart-test:local \
  go test ./cmd/daptin -run '^TestAdminRestartDocker' -count=1 -v
```

These tests create uniquely named disposable containers, volumes, and networks.
They exercise the non-root production entrypoint as PID 1 with Docker restart
disabled, an init process with a read-only root filesystem, SQLite, PostgreSQL
17, and MariaDB 10.11. They verify readiness transitions, unchanged container
PID/start time, preserved authentication, administrator-owned system tasks,
no duplicate tasks after a persisted restart, an accepted request draining
before replacement, and SIGTERM canceling a pending restart. Existing containers
are not used. The database images must be available to Docker.

## Long integration reservation acceptance

`TestIntegrationReservationDeadlineRealE2E` deliberately waits for a real
330-second provider with a 600-second timeout. It verifies that quota stays
reserved beyond five minutes across both HTTP routes, GraphQL, a switched-user
workflow and a scheduled action. Timeout/disconnect controls must stop provider
work and settle zero successful-work units. The test does not shorten the server
clock or lease. It is skipped unless both opt-ins are present:

```bash
DAPTIN_REAL_E2E=1 DAPTIN_LONG_INTEGRATION_E2E=1 \
DAPTIN_E2E_RUNTIME=release DAPTIN_E2E_BINARY=/path/to/candidate \
DAPTIN_TEST_POSTGRES_DSN='postgres://.../empty_disposable_database?sslmode=disable' \
  go test ./cmd/daptin -run '^TestIntegrationReservationDeadlineRealE2E$' \
    -count=1 -v -timeout=25m
```

Without the PostgreSQL DSN, that subtest skips. Each database takes over five
minutes; run individual `/sqlite` and `/postgres` subtests separately when needed.

For the metering row-lock race contract, use a separate empty PostgreSQL database:

```bash
DAPTIN_TEST_METERING_POSTGRES_DSN='postgres://.../empty_disposable_database?sslmode=disable' \
  go test ./server/resource -run '^TestMeteringDeadlinePostgresLockRaces$' -count=1
```

This test observes actual PostgreSQL lock waits and verifies both orderings of
extension versus recovery, plus completion/cancellation versus extension.

## Durable agent qualification

`TestAgentDurableInterfaceRealE2E` uses a fake model with real resource CRUD,
relationship reads, named actions, REST/GraphQL controls, model permission
revocation, persisted waits, and process-crash recovery. With the existing
private PostgreSQL fixture enabled it also starts three Daptin/Olric nodes and
checks competing wakes, cross-node controls and persisted lease renewal.
Interrupted inference may retry; an effect dispatched before a crash must become
unknown and require reconciliation without redispatch.

```bash
go build -o /tmp/daptin-agent-test ./cmd/daptin
DAPTIN_REAL_E2E=1 DAPTIN_E2E_BINARY=/tmp/daptin-agent-test \
  go test ./cmd/daptin -run '^TestAgentDurableInterfaceRealE2E$' -count=1 -timeout=6m
DAPTIN_REAL_E2E=1 DAPTIN_E2E_BINARY=/tmp/daptin-agent-test \
  DAPTIN_E2E_MAIL_POSTGRES_BIN=/path/to/postgresql/bin \
  go test ./cmd/daptin -run '^TestAgentDurableInterfaceRealE2E$' -count=1 -timeout=6m
```

See [Durable agents](../../wiki/Agents.md) for the public configuration and recovery
contract, including performer cancellation and external-effect boundaries.

### CPU and memory load measurement

`TestPermissionLoadRealE2E` is an opt-in macOS measurement using the same isolated
HTTP/process fixture. Four ordinary users read 50 group-shared records through
single-record and 25-row list requests. It records validated throughput,
latency, process CPU time and RSS sampled every 500 ms. CPU 100% means one core;
three-node figures sum Daptin processes, including embedded Olric, and exclude
PostgreSQL and the load generator. It asserts correctness, not a hardware-specific
performance threshold. The private instance's measured route rate limits are
raised so the test measures resource work rather than rate-limit rejection.

```bash
GOWORK=off go build -o /tmp/daptin-agent-test ./cmd/daptin
DAPTIN_REAL_E2E=1 DAPTIN_PERMISSION_LOAD=1 \
  DAPTIN_E2E_BINARY=/tmp/daptin-agent-test \
  DAPTIN_E2E_MAIL_POSTGRES_BIN=/path/to/postgresql/bin \
  DAPTIN_LOAD_VARIANT=cached DAPTIN_LOAD_SECONDS=10 \
  DAPTIN_LOAD_AGENT=1 DAPTIN_LOAD_RESULTS=/tmp/daptin-load.json \
  GOWORK=off go test ./cmd/daptin -run '^TestPermissionLoadRealE2E$' \
    -count=1 -v -timeout=8m
```

Set `DAPTIN_LOAD_CLUSTER=1` for three nodes sharing private PostgreSQL and Olric.
Set `DAPTIN_LOAD_ONLY_AGENT=1` with `DAPTIN_LOAD_AGENT=1` to measure agents on a
fresh process without preceding read saturation. Each agent uses a local mock
provider, performs two model calls and one real tool read, and persists its
history. Agent timing includes admission, scheduling and status polling. RSS
includes retained heap; a short cooldown does not establish a memory leak.

### Live provider qualification

The `live` build-tag test uses real provider credentials and makes billable model
calls with synthetic records in a disposable Daptin database. The credentials
file has a `providers` map keyed by `google`, `openrouter`, or `lilac`, each with
an `api_key` field. Keys are read in process. The optional report includes
provider request bodies, sanitized rejections, conversations, usage, and assertions,
without credential headers or keys.
Every provider HTTP exchange retains its status, content type, content encoding,
and complete received body as base64 bytes, including non-JSON errors and empty
bodies. Text and JSON views are derived from those bytes; gzip bodies also have
a decoded view. Bodies are not capped or substituted before forwarding. Known
keys are redacted in retained data only. A read failure retains the received
prefix with `complete: false`; a transport failure explicitly records that no
HTTP response was received. Each response stays paired with its request.
When a report path is set, exchanges are also appended synchronously to its
`.http.jsonl` journal before the response is forwarded, so a later test failure
cannot discard already captured bodies. Journal write failures fail qualification.
A local recording HTTP transport forwards requests to each provider's normal
HTTPS endpoint; execution still enters through Daptin's gateway.

```bash
DAPTIN_REAL_E2E=1 DAPTIN_E2E_BINARY=/tmp/daptin-agent-test \
  DAPTIN_AGENT_CREDENTIALS="$HOME/.pragma/credentials.yml" \
  DAPTIN_AGENT_MODELS=google-flash,router-mini,router-haiku \
  DAPTIN_AGENT_REPORT=/tmp/daptin-agent-live.json \
  go test -tags live ./cmd/daptin -run '^TestAgentLiveWorkflows$' \
    -count=1 -v -timeout=20m
```

`DAPTIN_AGENT_SCENARIOS` optionally selects a comma-separated subset of `crud`,
`triage`, `decision`, `wait`, `scope`, and `repair`. These check stored changes, action
invocation, deletion, relationship reads, hostile ticket text, a decision across
process replacement, timer resumption, and row denial with the whole CRUD/action
interface exposed. Denial is expected in `scope`; provider failures and false
claims of task completion fail qualification. Select `google-three` with
`DAPTIN_AGENT_SCENARIOS=crud,decision,repair` to qualify Gemini 3 tool continuation,
persisted signatures across process replacement, and backend-guided repair.
Omit the model selector to probe all configured models, including Lilac.
`repair` changes the backend verifier through the ordinary action resource API,
requires an initial rejection, then checks that the agent updates the draft and
obtains a successful verification result. This application policy stays in the
action definition.

## Agent schema restart acceptance

`TestAgentSchemaRestartRealE2E` runs a deterministic provider against real Daptin
processes. It covers SQLite and a three-node PostgreSQL/Olric cluster, schema
created through `world`, schema uploads larger than 5 KiB installed into canonical records, denied
non-administrator actions, one restart request, saved-run continuation, and reads
of the resulting record on every node without operator restarts.

```bash
GOWORK=off go build -o /tmp/daptin-restart-test ./cmd/daptin
GOWORK=off DAPTIN_REAL_E2E=1 DAPTIN_E2E_BINARY=/tmp/daptin-restart-test \
DAPTIN_E2E_MAIL_POSTGRES_BIN=/path/to/postgresql/bin \
go test ./cmd/daptin -run '^TestAgentSchemaRestartRealE2E$' -count=1 -v -timeout=8m
```

`TestAgentSchemaRestartLive`, with the `live` build tag, repeats the workflow
using OpenRouter `openai/gpt-4.1-mini`. Set `DAPTIN_AGENT_CREDENTIALS` to the provider
credentials YAML path. Each trial prints a separate retained evidence directory
containing sanitized complete provider bodies, run/step/attempt rows, per-node
results and process logs. No authentication headers are recorded.

`TestAgentRestartCancelsInferenceAtDrainDeadlineRealE2E` holds an inference past
the configured drain deadline and verifies that process replacement still
happens and the same run resumes. It uses the normal shutdown timeout and can
take about a minute because an interrupted run may await its existing lease.

## Agent reliability qualification

Use Go 1.26.11 for release qualification, matching the release workflow.

`cmd/daptin/agent_reliability_real_e2e_test.go` uses the shared process harness,
normal resource/action APIs, and deterministic local providers. Set
`DAPTIN_RELIABILITY_RESULTS` to retain provider bodies, node logs, restart and
membership observations, transaction responses, and resource measurements.

```bash
GOTOOLCHAIN=go1.26.11 GOWORK=off DAPTIN_REAL_E2E=1 DAPTIN_E2E_BINARY=/tmp/daptin-candidate \
DAPTIN_E2E_MAIL_POSTGRES_BIN=/path/to/postgresql/bin \
DAPTIN_RELIABILITY_RESULTS=/tmp/daptin-reliability \
go test -vet=off ./cmd/daptin \
  -run '^(TestSchemaUploadTransactionRollbackRealE2E|TestConcurrentSchemaUploadsRealE2E|TestActiveAgentActionBatchDrainRealE2E)$' \
  -count=1 -v -timeout=10m
```

The rollback and import/merge database matrices additionally use MariaDB when
`DAPTIN_RELIABILITY_MYSQL_CONTAINER` identifies a disposable `mariadb:10.11`
container with root password `test-only` and port 3306 published on loopback.
The fixture creates a separate empty database for each case.

`TestAgentClusterReliabilityRealE2E` additionally requires
`DAPTIN_RELIABILITY_SOAK=1`. It targets 50 schema/restart cycles on the same
three-node PostgreSQL/Olric cluster, rotates the initiating node, resumes a
waiting run after each restart, checks the new column on every node, and checks
for duplicate effects. Background reads continue through restart windows;
separate stable traffic windows measure throughput, latency, process CPU and
RSS. Each cycle also requires identical membership/coordinator views, routed
partitions, matching current primary ownership on every member, one shared
atomic counter across all nodes, and PubSub delivery to
every node. SQL pool, OS thread, network connection, and Olric snapshots
accompany each completed cycle. Go runtime snapshots include goroutine counts
and heap statistics; OS thread counts are recorded separately. Saved run,
step, attempt, and result records retain the conversation alongside full
provider request/response bodies. Failed HTTP response bodies and direct cache
metadata are also retained for diagnosis.

Membership failures are recorded as test failures while observation continues.
A process that exits or fails to replace exactly once stops the experiment;
successful HTTP readiness alone does not qualify cluster convergence. Use
`-run '^TestAgentClusterReliabilityRealE2E$' -timeout=30m` for this case.
