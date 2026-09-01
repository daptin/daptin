# Daptin LLM Gateway: Corrected Implementation Plan

Status: implementation complete; external capacity and account-funded media verification explicitly skipped
Prepared: 2026-08-31
Daptin baseline: `4dec8199`
Standalone module release: `github.com/daptin/llmgateway@v0.1.0-dev.31` (`81f292d`)

### Implementation status as of 2026-09-02

| Area | Evidence-backed status |
|---|---|
| Reusable module boundary | Complete. `github.com/daptin/llmgateway` owns protocol, routing, retries, streaming, cache, guardrails, health, and provider-neutral usage facts. Its architecture test prohibits Daptin, Gin, api2go, SQL/sqlx, Goqu, and Olric imports and module-owned metering policy. |
| Daptin composition | Complete. One gateway is constructed in `server/server.go`, shared by HTTP and both LLM actions, and drained by `Runtime`. Catalog reads use `DbResource`, relation expansion, existing credential APIs, and `DaptinReferenceId`. File assets use the existing `document` resource and asset-column lifecycle; file, batch, usage, and quota mutations use only canonical resource create/update/delete methods. Architecture tests scan production and test files for direct SQL execution and alternate transaction write paths. |
| Generic metering | Complete for deterministic qualification. The existing `server/resource` owner has one transactional admit/complete/cancel/expiry state machine used by CRUD, actions, and LLM facts. The existing Daptin scheduler runs bounded reservation recovery every ten seconds, including during idle request periods, and quiesces through `Runtime`. SQLite, PostgreSQL 17, and MySQL 8.4 tests cover concurrent hard admission and idempotent terminalization. |
| One-way cutover | Complete. GoAI execution, duplicate OpenAI wire types, comma-separated model resolution, `llm_usage`, and the lossy chat-converted `/v1/completions` path are removed; native text completions now use the shared canonical engine with no compatibility branch. |
| Deterministic verification | Passing: both repositories' full normal, vet, and race suites; focused Daptin race suites for `server`, `server/llm`, and `server/resource`; standalone `GOWORK=off` normal/race suites; module dependency-boundary checks; protocol/adapter fixtures; bounded fuzzing; routing/fallback; cache isolation/coalescing; cancellation; lifecycle; architecture scans; catalog fan-out; readiness; provider health; action E2E; and the Daptin database matrix. The previously flaky root `TestServerApis` also passes both the current complete normal/race runs and its isolated race rerun without a detector report. |
| Live certification | Complete for the required release scope; the manifest remains `kind: target` and makes no claim for an unverified cell. The 2026-09-02 real-provider run passed every configured Google cell: chat, streaming, tools, parallel tools, reasoning, structured output, vision, embeddings, image generation, and credential rejection. OpenRouter passed chat, streaming, tools, structured output, vision, all embedding forms, Responses, native text completions in stream and non-stream modes, rerank, speech generation, and credential rejection. Rerank preserves OpenRouter's top-level search-unit usage through the same canonical response used for Cohere metadata. Image generation returned the safely normalized `insufficient_quota` error; transcription was stopped by OpenRouter's provider-side minimum-balance gate before request validation. These two account-funded OpenRouter checks are explicitly skipped, remain uncertified, and are not release blockers. Lilac passed four chat models, streaming, tools, structured output, vision, Responses, and credential rejection. Its current Responses shape exposed optional reasoning-item status plus typed reasoning-content part events; the shared decoder was corrected against the pinned OpenAI reasoning-item schema and a focused live rerun passed every Lilac cell. No credential or provider payload is stored in either repository. |
| Operational qualification | Complete for the required release scope. Two independent Daptin processes sharing PostgreSQL and clustered Olric both served the same model, then the second process observed a credential rotation made through the first within the five-second gate. Pub/sub remains a latency hint; a three-second content-fingerprint poll is the bounded recovery path because Olric pub/sub clients subscribe to one member. Daptin schedules the module's interval-aware health probes with at most one probe pass in flight; shutdown cancellation cannot poison shared provider circuit state. A reproducible gateway-core serial/parallel allocation benchmark is published in the module operating profile. The opt-in full-path benchmark passes through authenticated HTTP, Daptin-managed PostgreSQL configuration and durable metering, and a deterministic upstream: on an Apple M1 Max it measured 16.8–17.4 ms/request serial and 5.14–5.24 ms/request at the Go benchmark's 10-way parallel execution across three five-second samples, with no request failures. A separate sustained parallel sample completed 27,333 requests without failure at 5.44 ms/request aggregated benchmark time (212 seconds wall-clock including startup and calibration). Its allocation figures describe the benchmark client because Daptin runs as a subprocess. The repository retains an opt-in production-topology gate which rejects non-Linux or undersized hosts and inadequate file-descriptor limits, fixes the Daptin database pool ceiling at 20, paces at least 250 requests/second for at least ten minutes, checks latency and throughput, and validates 1,000 concurrent SSE streams against a 96-KiB-per-stream ceiling. Execution on the documented 8-core/16-GiB PostgreSQL topology is explicitly skipped and is not a release blocker; no production capacity claim is made from the local results. |
| Release reproducibility | Module commit `81f292d` is published as `v0.1.0-dev.31`, and both hosted workflows for the exact branch/tag SHA pass ([branch run](https://github.com/daptin/llmgateway/actions/runs/33546623747), [tag run](https://github.com/daptin/llmgateway/actions/runs/33546621601)). Daptin pins that tag and checksum directly. With workspace resolution disabled, `go mod verify`, the complete Daptin normal/vet suites, and focused `server`, `server/llm`, and `server/resource` race suites pass. The adjacent module checkout is no longer required to build or test Daptin. |

Current parity increment: native completions, moderation, rerank, speech,
transcription/translation, image edits and variations, Anthropic Messages, stateless Responses
fidelity (including inline files), search, and OCR now use the same canonical
engine. Responses streaming uses typed, explicitly supported lifecycle, output,
content, refusal, function-call, and reasoning events; it preserves required
indices, annotations, log probabilities, cached/reasoning-token usage, and
strictly increasing sequence numbers while rejecting unknown event types. SSE
keepalives cover both stream setup and idle intervals. Stateless Responses
compaction now uses the same operation pipeline, preserves opaque compaction
items and detailed usage, and explicitly rejects stored `previous_response_id`
state. OCR accepts bounded
multipart uploads or safe URL/data-URI documents and
rejects filesystem/provider-owned identifiers. Usage and provider pricing are
now named-measure based (`measure -> micros per million units`), so token, page,
search-unit, byte, credit, and future non-LLM measures share one calculation and
the existing generic Daptin metering owner. No token-specific pricing path
remains. The standalone normal/race suites and the complete Daptin normal/race
suites pass after this increment. The compatibility manifest records every
in-scope route found in LiteLLM `v1.98.0` commit
`d8f71d7bdbd7c9873d98293f83d64c6db72847e6` as implemented or explicitly
unsupported with its missing invariant. The implemented surface and required
release gates are complete; explicitly skipped checks remain visibly uncertified.

## 1. Purpose

This document replaces the rejected implementation ending at Daptin commit
`1e787fda` and standalone-module commits `ed70e49` and `c7f60f2`. Those commits
remain the audit record, but none of their code is the implementation baseline.

The goal is to make Daptin a reliable, predictable, horizontally scalable
OpenAI-compatible LLM gateway with the useful gateway capabilities of LiteLLM,
while preserving Daptin's architecture:

- Daptin resources remain the source of configuration, relationships,
  permissions, credentials, usage, and quota state.
- `github.com/daptin/llmgateway` is an independently reusable Go module, in the
  same architectural sense that Daptin consumes api2go and rclone.
- LLM execution and metering are independent concepts. The gateway emits a
  generic usage/admission contract; Daptin's existing metering capability
  decides policy and records usage for LLM, CRUD, actions, and future workloads.
- Existing Daptin functions and lifecycle hooks are extended in their current
  owners. No parallel resource access, schema, ID, JSON, transaction,
  credential, configuration, or metering framework is introduced.
- The cutover has one production path. Old execution code is removed in the
  same change that activates its replacement.

This is gateway parity, not a claim to reproduce every LiteLLM endpoint or its
Python SDK. LiteLLM currently advertises a unified OpenAI-format interface for
100+ models/providers, consistent responses, retries/fallbacks, a self-hosted
proxy, virtual keys, cost tracking, guardrails, caching, and observability. Its
router also covers multi-deployment load balancing, cooldowns, timeouts,
retries, fallbacks, and several routing strategies. Daptin must make only
capabilities backed by conformance tests and certified provider tests.

The compatibility manifest is compared with the stable LiteLLM `1.98.0`. Hardware and
live-provider qualification are release evidence, not blockers for implementing
the LLM API contract. Agent/MCP gateway and administrative UI parity are outside
the LLM API goal, but every LLM-facing endpoint advertised by LiteLLM must be
classified as supported, intentionally host-composed, or unsupported with a
specific missing invariant. An arbitrary release freeze must not hide API gaps.

Official comparison sources:

- [LiteLLM overview](https://docs.litellm.ai/docs/)
- [Routing and load balancing](https://docs.litellm.ai/docs/routing)
- [Virtual keys](https://docs.litellm.ai/docs/proxy/virtual_keys)
- [Spend tracking](https://docs.litellm.ai/docs/proxy/cost_tracking)
- [Caching](https://docs.litellm.ai/docs/proxy/caching)
- [Reliability and fallbacks](https://docs.litellm.ai/docs/proxy/reliability)

## 2. What went wrong in the rejected implementation

The rejected Daptin diff changed 99 files with 9,013 insertions and 2,762
deletions. The problem was not merely excessive code. It created competing
owners for mechanisms Daptin already has.

| Rejected addition | Existing Daptin owner | Correct treatment |
|---|---|---|
| `server/schemamigration`, `server/resource/migration`, gateway and metering migration files | `server/resource/columns.go`, `StandardTables`, `StandardRelations`, and existing startup resource reconciliation | Delete the parallel migration lifecycle. Declare resource shape only through the canonical catalog and use the existing reconciliation flow. |
| SQL `CREATE TABLE` fixtures for LLM/metering resources | Daptin table declarations and existing database test setup | Tests create Daptin resources through the normal schema/configuration path. They do not restate physical schemas. |
| `server/resource/resource_select.go` and `ResourceCatalog` | `DbResource.GetRowsByWhereClauseWithTransaction`, `GetObjectByWhereClauseWithTransaction`, relation expansion, and normal CRUD functions | Use existing resource APIs. Add a narrowly scoped method to the current owner only when no existing operation can express the requirement. |
| `server/resource/standard_config.go` | existing `CmsConfig` construction and startup configuration | Use the normal config assembled by Daptin. Do not clone a second standard config graph. |
| `loadProviders` | `DbResource.GetLLMProviderByNameWithTransaction` and `GetActiveLLMProviders` | Extend the existing provider representation and loader in place. Do not issue a second provider query in an adapter. |
| `loadDeployments`, `loadModels`, `loadGuardrails` implemented as hand-built SQL row scanners | normal Daptin resources, relations, and `GetRowsByWhereClauseWithTransaction` | Load relation-aware resource maps through the established API and translate once at the host/module boundary. |
| `loadReferenceIndex` | Daptin relation expansion and reference-ID helpers in `dbmethods.go` | Do not build shadow ID indexes. Request included relations or use the existing ID/reference lookup only where a relation is not already present. |
| `daptinReference`, `gatewayID`, `stableID`, `optionalID`, `referenceBytes`, `referenceString` | `server/id.DaptinReferenceId`, its `Scan`/`String`/JSON methods, and `InterfaceToDIR` | Use `DaptinReferenceId` directly. At the module boundary use its `String()` once because module IDs are deliberately opaque strings. |
| `resolveResourceReference` and `resolveOptionalResourceReference` | `GetReferenceIdToIdWithTransaction` and Daptin relation handling | Call the existing function directly only if an internal numeric ID is truly required. Prefer relation-expanded rows so no lookup is needed. |
| `server/jsonx` and repeated canonical/strict JSON helpers | package-level JSON conventions already used by Daptin; standard decoder is used locally when strict parsing is required | Keep wire decoding with the wire protocol owner and resource JSON with the existing resource package convention. Do not create a general JSON package for one feature. |
| new `server/metering` policy/repository/type hierarchy | `server/resource/metering.go`, `metering_expr.go`, and `metering_middleware.go` | Evolve `MeteringService` in place and keep it domain-neutral. Do not retain two evaluators or ledgers. |
| `server/llmgateway` SQL/sqlx repositories and Daptin-specific engine service | standalone `github.com/daptin/llmgateway` plus existing Daptin composition points | The reusable module owns engine behavior and ports. Daptin's existing `server/llm` package is reduced to the host adapter. |
| catalog revision resources and schema-version state | normal resource updates/events, startup state, and in-process immutable snapshot | Reload from the canonical resources and atomically replace a compiled snapshot. Use existing update notifications as hints and a content fingerprint/poll as recovery; do not create a second schema/revision authority. |
| local counter fallback for distributed hard limits | authoritative metering decision | Never widen a hard limit on coordination failure. A local counter is acceptable only as a test implementation or explicitly best-effort deployment protection, never customer quota/spend enforcement. |
| `llm_usage` async writes plus `api_usage` | existing generic metering ledger | Remove asynchronous LLM ledger writes. One metering terminalization records the request; LLM-specific diagnostic facts are emitted as bounded metadata/telemetry unless a proven query requirement justifies a resource. |

Moving these abstractions to a differently named package would not fix them.
Their duplication is the defect.

## 3. The way of Daptin: implementation rules

These rules are review gates, not preferences.

1. Search before adding. Every proposed type or helper must cite the existing
   functions it evaluated and why they cannot perform the operation.
2. Resource definitions belong in `server/resource/columns.go` and relations in
   `StandardRelations`. There is no LLM migration framework.
3. Resource reads and writes use `DbResource`, Daptin relation expansion, and
   existing CRUD/action lifecycle. No feature-local generic repository.
4. Public references use `daptinid.DaptinReferenceId`. No UUID/reference
   wrappers or alternate scanners/codecs.
5. Credentials use `GetCredentialByName`, `GetCredentialByReferenceId`, or the
   permission-aware integration variant as appropriate. No direct credential
   table reader and no secret in a catalog snapshot.
6. Daptin JSON follows the owning package's current convention. Wire-protocol
   strictness stays in the standalone protocol package. No generic JSON facade.
7. Daptin's existing `database.DatabaseConnection`, `DbResource` connections,
   and caller-owned action transactions remain the host lifecycle. The reusable
   module never imports `database/sql`, sqlx, a database driver, Daptin, Gin,
   api2go, Olric, or a Daptin statement builder.
8. `MeteringService` remains generic. It receives workload identity and named
   measures; it does not import or understand models, providers, deployments,
   prompts, or OpenAI protocol types.
9. No database transaction remains open during DNS, provider connection,
   inference, retry delay, response streaming, cache I/O, or guardrail network
   calls.
10. No compatibility flag, v1/v2 service, fallback to the old resolver, second
    ledger, or old action execution path survives cutover.
11. Unsupported protocol fields fail explicitly. They are never silently
    ignored and are never accepted merely because a provider might understand
    them.
12. A feature is supported only when the compatibility manifest and its
    conformance/live tests agree.

### Mandatory architecture tests

Tests must fail if:

- the standalone module imports Daptin, Gin, api2go, SQL/sqlx, a driver, Goqu,
  Olric, or another host implementation;
- the standalone module declares metering policy concepts such as plans,
  limits, quota windows, quota buckets, or SQL reservations;
- Daptin adds another migration package, resource repository abstraction,
  reference codec, or JSON facade for this feature;
- production code contains both `GoAIProvider` and the new engine path;
- HTTP handlers and actions use different routing/provider/accounting code;
- a hard metering path changes to allow on persistence/counter failure.

## 4. Existing Daptin capability inventory

Implementation begins from this inventory; it is not greenfield.

### Resources, relationships, and identifiers

- `server/resource/columns.go` owns `StandardTables` and `StandardRelations`.
- `server/resource/dbmethods.go` already provides
  `GetRowsByWhereClauseWithTransaction`,
  `GetReferenceIdByWhereClauseWithTransaction`,
  `GetIdToReferenceIdWithTransaction`, and
  `GetReferenceIdToIdWithTransaction`.
- `server/resource/dbfunctions_get.go` already provides
  `GetObjectByWhereClauseWithTransaction`,
  `GetLLMProviderByNameWithTransaction`, `GetActiveLLMProviders`, and
  `ResolveLLMProviderByModel`.
- `RowsToMap` already normalizes reference columns into
  `DaptinReferenceId`; `server/id/id.go` owns every other required conversion.

### Credentials and configuration

- `server/resource/credentials.go` owns encrypted credential lookup and
  decryption.
- `server/resource/cms_config.go` owns config reads/writes.
- Provider secrets must be resolved only while building an adapter instance;
  catalog documents carry a reference, never secret bytes.

### Metering

- `server/resource/metering.go` already owns `MeteringService.Preflight` and
  `Record`, plan/member/quota lookup, cost-expression evaluation, and usage
  creation.
- `server/resource/metering_middleware.go` already composes metering with CRUD.
- the action path already composes metering around actions.
- `server/resource/metering_expr.go` already owns expression evaluation.

The implementation must strengthen these components for atomic admission and
terminalization. It must not replace them with `server/metering`.

### Existing LLM entry points

- `server/endpoint_llm.go` owns `/v1` HTTP registration today.
- `server/actions/action_llm_chat.go` and
  `server/actions/action_llm_embedding.go` own Daptin action translation.
- `server/llm/goai_provider.go` currently owns provider selection/invocation
  and performs an asynchronous `llm_usage` write.
- `server/server.go` and `server/action_provider/action_provider.go` currently
  construct separate `GoAIProvider` values.

These are cutover points, not invitations to add another set beside them.

## 5. Target ownership and dependency direction

```text
Gin / api2go / Daptin actions
             |
             v
existing Daptin composition and server/llm host adapter
  |          |             |              |
  |          |             |              +-- existing telemetry/cache services
  |          |             +-- existing MeteringService
  |          +-- existing credentials/resources/permissions
  v
github.com/daptin/llmgateway
  protocol -> canonical contracts -> engine -> router -> provider adapters
```

Dependencies point inward to the reusable module. The module exposes typed
contracts and narrow host ports. It does not know which host implements them.

### Standalone module owns

- strict OpenAI wire request/response/error/SSE behavior;
- canonical chat, Responses, embedding, and image contracts;
- model/deployment catalog validation and immutable compiled snapshots;
- adapter registry and provider translation;
- routing, weighted selection, bounded retry/fallback, cooldown, circuit
  state, and stream commit rules;
- provider-neutral usage normalization and fixed-point cost calculation;
- cache key/eligibility semantics;
- guardrail sequencing;
- lifecycle events and host-facing telemetry records;
- conformance manifest and reusable testkit.

### Daptin owns

- resource schemas, records, relations, permissions, and configuration;
- encrypted credentials and key lifecycle;
- principal construction from the existing authentication system;
- generic policy evaluation, quotas, reservations, and usage records;
- SQL transaction ownership and Olric integration;
- Gin endpoint registration and api2go action responders;
- startup composition, reload triggers, health exposure, and shutdown.

### Correct metering port

The module may request generic admission and terminalization, but it must not
evaluate policy. Replace module-owned `accounting.Reservations`, policy limit
types, window parsing, and quota calculations with a host port shaped around
facts:

```go
type UsageSink interface {
    Admit(context.Context, Admission) (ReservationToken, error)
    Complete(context.Context, Completion) error
    Cancel(context.Context, Cancellation) error
}
```

`Admission` contains request ID, opaque principal/scope bindings supplied by the
host, operation/workload identity, start time, and estimated named measures.
It does not contain a compiled quota plan or SQL reservation rows. Daptin's
adapter passes the facts to the existing `MeteringService`, which alone loads
plans, evaluates expressions/limits, reserves authoritative capacity, and
persists terminal usage.

The name should reflect the generic contract (`UsageSink` or `MeteringPort`),
not imply that the LLM module owns accounting policy. The final name is chosen
once; no alias of the old interface remains.

## 6. Resource model: minimal canonical changes

The baseline `llm_provider.models` comma-separated field cannot represent
several deployments, weights, priority, health, operation capability, or an
upstream model alias reliably. Two new resources are justified. They are added
to `StandardTables` and `StandardRelations`, not created through migrations.

### `llm_provider`

Keep and evolve the existing resource as the provider-account record:

- keep `name`, `provider_type`, `base_url`, `provider_parameters`, `enable`, and
  the existing credential relation;
- remove runtime use of `models`, `credential_name`, and `model_pricing`;
- after the one-way cutover, remove those fields from the canonical declaration
  and all code. There is no fallback parser for comma-separated models;
- validate base URL and provider parameters through the selected module adapter;
- require HTTPS by default, with explicit configuration for a local/private
  endpoint. Never infer network permission from the URL.

### `llm_model`

One record defines a public gateway model:

- unique `name`;
- supported `operations`;
- explicit capabilities;
- default parameters partitioned by operation;
- routing strategy (initially one supported strategy, not a decorative enum);
- ordered fallback public models;
- unsupported-parameter policy;
- `enable`.

Daptin row permissions remain model visibility/access control. Do not add a
parallel model ACL table.

### `llm_deployment`

One record maps a public model to one upstream target:

- relation to `llm_model`;
- relation to `llm_provider`;
- unique operational `name` and `upstream_model`;
- operations supported by that deployment;
- priority and positive weight;
- request/connect timeout;
- optional max concurrency, RPM, and TPM protection;
- fixed-point per-million token pricing;
- adapter parameters and health-check configuration;
- `enable`.

The resource relation rows produced by Daptin are the linkage. Do not load all
reference IDs into hand-written maps.

### Existing generic metering resources

Keep `api_plan`, `api_member`, `api_usage`, and `api_quota` as generic resources.
Do not add model/provider/deployment relations or LLM-only columns to them.

Evolve only what atomic, workload-neutral reservation requires:

- an idempotency/request identifier for one logical usage;
- generic status/lifecycle state;
- generic named measures or metadata sufficient for tokens, bytes, cost, and
  compute units without teaching metering about LLMs;
- held/finalized/released reservation state and expiry where required for
  crash recovery;
- atomic quota mutation rather than read-modify-write.

Before adding a new reservation resource, prove with transaction and crash
recovery tests that the state cannot be represented safely by the current
`api_usage` plus `api_quota`. Prefer extending those two resources. A new
resource is permitted only by an architecture decision record demonstrating a
correctness requirement, not for code organization.

### `llm_usage`

Remove `logUsageAsync` and all writes to `llm_usage` at cutover. Prefer bounded
LLM dimensions in generic usage metadata and attempt telemetry. If operators
demonstrate a concrete relational query/retention requirement that metadata
cannot serve, add one LLM-owned fact resource later; it must compose with
`api_usage`, not duplicate its tokens/cost/status ledger.

## 7. Catalog loading without a second data layer

There will be one Daptin catalog adapter in the existing `server/llm` package.
It implements the standalone module's `CatalogSource`, but it is not a generic
repository.

Its load sequence is:

1. begin a short transaction using the existing `DbResource` connection;
2. call the existing `GetActiveLLMProviders` after extending that method in
   place for the final provider shape;
3. obtain enabled `llm_model` and `llm_deployment` rows using
   `GetRowsByWhereClauseWithTransaction` with required relations included;
4. translate Daptin maps and `DaptinReferenceId` values directly into module
   catalog DTOs;
5. resolve credentials with existing credential methods only while adapter
   instances are constructed;
6. commit/close the transaction before any provider network work;
7. let the module validate/compile the complete candidate and atomically swap
   it only on success.

There is no `providerRow`, `deploymentRow`, `loadReferenceIndex`, stable-ID
codec, canonical-JSON package, or generic `ResourceCatalog`.

### Reload and consistency

- startup blocks readiness until one valid snapshot is compiled;
- Daptin's existing create/update/delete event path triggers a debounced reload
  for relevant resources;
- a low-frequency poll/content fingerprint is recovery for missed events;
- event payloads are invalidation hints, not authoritative configuration;
- concurrent requests retain their immutable snapshot until completion;
- an invalid candidate is rejected and the last valid snapshot stays active;
- readiness reports the rejected reload and age of the active snapshot;
- no catalog revision table, schema version resource, or second configuration
  authority is added.

## 8. Request lifecycle and transaction boundaries

Every HTTP endpoint and Daptin action uses the same engine instance and request
lifecycle.

### Phase A: normalize and admit

1. HTTP protocol handler or action adapter builds the same canonical module
   request.
2. Daptin authenticates and constructs the principal using existing permission
   and membership data.
3. Module validates protocol/capabilities and builds the bounded route plan.
4. Daptin opens a short transaction for generic metering admission.
5. Existing `MeteringService` evaluates applicable generic policy and performs
   an atomic reservation/idempotent usage start.
6. Commit before leaving Daptin state.

### Phase B: execute

The module performs guardrails, cache lookup, routing, provider calls,
retries/fallbacks, response normalization, and streaming. No SQL transaction is
held. A stream may change deployment only before the first client-visible
event. Once committed, an error is terminal for that stream.

### Phase C: terminalize

1. Module produces generic final usage, bounded attempt telemetry, cache state,
   status, and normalized error class.
2. Daptin creates a fresh context with a bounded timeout even if the client
   disconnected.
3. Daptin opens a new short transaction.
4. Existing `MeteringService` finalizes or releases the reservation
   idempotently and records one `api_usage` result.
5. Commit before sending a non-streamed response; for a stream, finalization is
   part of its terminal handling and failures are surfaced operationally.

No unbounded goroutine performs billable writes. Repeated terminalization with
the same request ID/token must produce the same state without double charge.

## 9. Reliability and scalability contract

### Routing

- deterministic priority tiers;
- weighted choice within the first eligible tier;
- filter disabled, unhealthy, capability-incompatible, saturated, or
  rate-protected deployments before selection;
- bounded attempts and bounded total request time;
- retry only normalized transient failures;
- ordered public-model fallback with cycle validation;
- no retry after stream commitment;
- deterministic selector injection in tests.

Do not implement multiple routing algorithms until the single strategy is
correct under concurrency and failure. Additional strategies are separate,
manifested capabilities.

### Timeouts and cancellation

- separate connect/header, first-event, stream-idle, per-attempt, and total
  request timeouts;
- cancellation closes provider bodies and stops retry timers;
- finalization uses a fresh bounded context;
- all bodies and streams are closed exactly once.

### Limits

- durable customer quotas, spend, and billable usage use authoritative Daptin
  persistence and fail closed when a hard decision cannot be made;
- Olric may coordinate disposable deployment protection and cache state;
- loss of Olric must never grant customer budget;
- no hidden local fallback in production hard-limit wiring;
- reservation estimates include maximum retry exposure, then finalization
  releases unused exposure;
- quota update and reservation lifecycle are atomic and idempotent across nodes.

### Cache

- cache only explicitly eligible deterministic requests;
- key includes canonical request, public model/config fingerprint, tenant/key
  isolation where authorization differs, guardrail revision, and protocol
  operation;
- secrets and raw authorization headers never enter keys or values;
- cache hits still pass authorization and generic metering;
- stampede protection is bounded and failure falls back to normal execution,
  not a stale or cross-tenant response.

### Health and lifecycle

- readiness requires a valid catalog and required host dependencies;
- liveness must not depend on a provider being healthy;
- provider health changes routing eligibility without mutating the catalog;
- startup construction occurs once in `server/server.go`; the same engine is
  injected into endpoint and action registration;
- shutdown stops admission, waits for bounded in-flight completion, closes
  adapters/transports, and performs no indefinite wait.

## 10. OpenAI compatibility scope

### Implemented foundation

| Endpoint | Required behavior |
|---|---|
| `POST /v1/chat/completions` | text/multimodal input, tools/tool choice, structured output, sampling fields, `n`, streaming chunks, tool deltas, optional stream usage, normalized errors |
| `POST /v1/completions` | native prompt text/token inputs, sampling/logprob controls, `n`/`best_of`, streaming chunks, optional stream usage, normalized errors; never converted through chat |
| `POST /v1/responses` | stateless text/multimodal/tool input; typed named SSE lifecycle/output/content/refusal/function/reasoning events with ordered sequence numbers and detailed usage; reject unsupported stored-response/state fields and unknown upstream event types |
| `POST /v1/responses/compact` | stateless explicit input, opaque compaction-item preservation, prompt-cache controls, detailed usage; reject `previous_response_id` rather than introducing gateway-owned response state |
| `POST /v1/embeddings` | scalar/list text and supported token inputs, dimensions and encoding policy |
| `POST /v1/images/generations` | generation for capable deployments and normalized URL/base64 response |
| `POST /v1/moderations` | text/list and multimodal moderation input with normalized category and score maps |
| `POST /rerank`, `/v1/rerank`, and `/v2/rerank` | Cohere-compatible query/document ranking, document objects, provider controls, billed-unit/token metadata |
| `POST /v1/audio/speech` | bounded speech request, binary media response, format/content-type validation |
| `POST /v1/audio/transcriptions` and `/v1/audio/translations` | bounded in-memory multipart input, JSON/text subtitle modes, usage normalization |
| `POST /v1/images/edits` | single/multiple images, optional mask, current edit controls, normalized URL/base64 response |
| `POST /v1/images/variations` | bounded multipart source image, variation count/size/format controls, normalized URL/base64 response |
| `POST /v1/messages` | Anthropic request/response/error/SSE compatibility translated into the canonical chat operation, including tools, tool results, images, and `x-api-key` authentication |
| `POST /v1/search` and `/v1/search/:tool` | Perplexity-compatible scalar/list queries, bounds/domain controls, normalized results, and named search measures |
| `POST /v1/ocr` and `/ocr` | normalized OCR for URL/data-URI or bounded multipart documents, typed extraction controls/results, safe file semantics, and page/document/credit measures |
| `GET /v1/models` | only enabled models visible to the principal |
| `GET /v1/models/:id` | OpenAI-compatible model shape plus non-breaking `llmgateway` metadata |

`POST /v1/completions` is a native `text_completion` operation with its own
prompt, choice, logprob, usage, and stream contracts. It is never translated
through chat. There is no fake compatibility endpoint.

### Active parity work

The remaining surface is split by lifecycle and ownership, not by a convenient
"later" label:

1. Complete current operations first: chat parameter forwarding and streaming
   keepalives; Responses field/event fidelity; provider-reported streaming usage
   and cost preservation. These modify the existing protocol, canonical
   contract, adapter, and stream owners only.
2. The stateless inference operations now share the same canonical pipeline:
   actual text completions, moderation, rerank, audio speech, audio
   transcription/translation, and image edits. Each operation has one contract,
   one strict protocol decoder/encoder, one adapter translation, capability
   validation, routing, generic metering facts, and manifest conformance. There
   is no chat-conversion shim and no generic pass-through escape hatch.
3. Provider-native compatibility surfaces such as `/v1/messages` are added only when
   their wire contract can translate losslessly into an existing canonical
   operation. The wire adapter remains in `protocol`; provider routing remains
   unchanged.
4. Implement files and batches as Daptin-composed durable resources and actions.
   The standalone module owns provider-neutral batch/file contracts and provider
   adapters; Daptin owns persistence, permissions, relationships, encrypted
   credentials, recovery, and scheduling through existing resource/action
   mechanisms. No module database dependency and no shallow upstream-only proxy.
5. Classify video, vector-store inference, and
   other newly advertised LLM surfaces individually after the preceding common
   contracts exist. Assistants, fine-tuning, Agent/A2A, MCP, containers, skills,
   and administrative UI are separate product domains and are not silently
   counted as LLM API parity.

Every operation is enabled only by `llm_model.operations`, deployment
operations, adapter capabilities, and the verified manifest. Adding an endpoint
does not create a second router, provider client, catalog loader, usage ledger,
or Daptin action execution path.

### Pinned LiteLLM route reconciliation

The canonical route inventory was checked against LiteLLM `v1.98.0` commit
`d8f71d7bdbd7c9873d98293f83d64c6db72847e6`, including its proxy endpoint
modules and `ROUTE_ENDPOINT_MAPPING`. The compatibility manifest records each
canonical in-scope route as implemented or unsupported; LiteLLM's unversioned,
`/openai/v1`, provider-prefixed, and Azure deployment aliases are not claimed
as distinct protocol implementations.

| Route family | Classification |
|---|---|
| Stateless chat, completions, Responses, compaction, embeddings, images, moderation, rerank, audio, search, OCR | Implemented through the one canonical engine; `/v2/rerank` uses the same rerank handler and operation |
| Token counting | Unsupported until model-revision-correct tokenizer/provider contracts exist; byte and character estimates are not accepted as token counts |
| Stored/background Responses | Unsupported because retrieval, input-item listing, cancellation, and deletion require durable state and ownership semantics |
| Realtime sessions and calls | Unsupported until bidirectional lifecycle, ephemeral-secret, cancellation, and backpressure contracts exist |
| Video jobs and characters | Unsupported until canonical asynchronous jobs, durable media, authorization, and provider certification exist |
| Gemini, Bedrock, and provider-native wire routes | Unsupported until lossless typed protocol adapters and streaming fixtures exist; generic pass-through remains prohibited |
| Google Interactions and vector-store search | Unsupported until their state/ownership contracts have a canonical host owner |
| Assistants, fine-tuning, Agent/A2A, MCP, containers, skills, RAG ingestion, evals, and administrative endpoints | Separate product domains, excluded from the LLM inference parity claim rather than silently counted |

### Error and streaming consistency

- one OpenAI-compatible error envelope and normalized error taxonomy;
- adapter errors preserve safe status/retry metadata without leaking provider
  secrets or raw bodies;
- strict body size and JSON document limits;
- exactly one JSON document for non-streamed calls;
- valid SSE framing, `[DONE]` behavior where required, usage semantics, client
  disconnect behavior, and no string interpolation into JSON errors.

## 11. Provider strategy

The module baseline currently has an OpenAI-compatible adapter. This is useful
but not equivalent to certified provider parity.

### Adapter requirements

Each adapter declares:

- operations and request features it supports;
- provider-specific authentication/header construction;
- request and response translation;
- streaming event translation;
- usage extraction, including cached/reasoning tokens where provided;
- normalized errors and retryability;
- model/deployment validation;
- optional health probing.

Provider-name switches outside the adapter registry are prohibited.

### Certification tiers

1. Protocol conformance: deterministic fixtures and mock upstream behavior.
2. Provider sandbox/live smoke: one minimal call per supported operation.
3. Feature certification: tools, structured output, vision/media, streaming,
   usage, errors, and cancellation for relevant models.
4. Reliability certification: retry/fallback and timeout behavior under injected
   faults.

Google, OpenRouter, and Lilac credentials supplied for verification must be
stored only in ignored environment/secret configuration or Daptin credentials.
They must never appear in source, fixtures, command output, logs, snapshots, or
the compatibility manifest. Credentials exposed through a conversation must be
rotated after the authorized verification run.

## 12. File-level implementation plan

### Standalone `github.com/daptin/llmgateway`

Modify existing owners only:

- `host.go`: replace policy-owning accounting semantics with the generic host
  metering/usage port; keep host-neutral interfaces.
- `contract/contract.go`: remove quota windows/reservation structures and pass
  only workload facts plus opaque host policy bindings.
- `engine.go`: stop compiling/evaluating plans; call the host port at admit and
  terminal states.
- `accounting/policy.go`: delete module-owned policy evaluation. Keep only
  provider-neutral fixed-point usage cost code if it has no quota semantics.
- `catalog/document.go` and `catalog/compiler.go`: remove metering plan contents
  from the catalog. At most keep opaque policy IDs needed to hand bindings back
  to the host.
- `counters.go`: remove production fail-open fallback semantics. Keep local
  stores in `testkit` unless an explicitly best-effort use is named at wiring.
- `protocol/openai/*`, `adapter/*`, routing, cache, guardrails, health, and
  stream files: complete the manifested contract without host dependencies.
- `architecture_test.go`: enforce forbidden imports and metering-policy
  ownership.
- `compatibility/manifest.json`: change from aspirational `kind: target` to a
  generated/verified support declaration; unverified entries cannot be marked
  supported.

The module stays a normal tagged Go module. Daptin consumes a pinned version;
local development may use a temporary workspace/replace outside committed
release configuration, not a permanent filesystem coupling.

### Daptin

- `go.mod`/`go.sum`: add only the tagged module dependency and remove GoAI when
  no remaining code imports it.
- `server/resource/columns.go`: evolve `llm_provider`; add `llm_model` and
  `llm_deployment`; add only proven generic metering fields; update
  `StandardRelations`.
- `server/rootpojo/llm_provider.go`: evolve the existing provider DTO rather
  than create a second provider row type. Add narrowly owned model/deployment
  DTOs only if maps at the boundary become less safe than existing conventions.
- `server/resource/dbfunctions_get.go`: modify existing provider functions in
  place. Do not add parallel provider loaders. Use existing generic resource
  reads for model/deployment data.
- `server/resource/metering.go`: evolve `MeteringService` to idempotent
  admit/finalize/cancel and atomic, generic quota enforcement. Preserve its
  expression and post-action integration.
- `server/resource/metering_middleware.go` and action metering integration:
  consume the same enhanced service contract so CRUD/actions gain the same
  reliability rather than becoming a legacy path.
- `server/llm`: replace `GoAIProvider` with the smallest Daptin host adapter for
  catalog, secrets, principal/authorization, generic metering, counters/cache,
  and telemetry. This package may know Daptin; the imported module may not.
- `server/endpoint_llm.go`: retain endpoint registration but delegate strict
  wire handling to the module's `net/http` handler or explicit protocol API.
  It must not reimplement request validation/error/SSE/routing.
- `server/actions/action_llm_chat.go` and
  `action_llm_embedding.go`: preserve action names and api2go responder shape,
  but translate to the same canonical engine calls. Remove duplicated option
  parsing when the module contract already represents it.
- `server/action_provider/action_provider.go` and `server/server.go`: construct
  one engine/composition root and inject it. Do not construct endpoint and
  action engines separately.
- delete `server/llm/goai_provider.go`, its OpenAI types, and tests after the new
  shared path passes all gates; delete `llm_usage` runtime code concurrently.

No `server/llmgateway`, `server/metering`, `server/schemamigration`,
`server/resource/migration`, `ResourceCatalog`, `standard_config`, or `jsonx`
package is part of this plan.

## 13. Implementation phases and hard gates

### Phase 0: freeze and characterization

- keep Daptin at `4dec8199` and module at `d88d274` until tests describe current
  behavior and desired cutover;
- add baseline tests for existing provider credential resolution, actions,
  endpoints, metering middleware, and transaction closure;
- turn every compatibility claim into a manifest test;
- inventory duplicate/helper proposals in review before implementation.

Gate: no application code change; both repositories pass unit and race tests.

### Phase 1: correct module boundary

- remove policy evaluation and quota data from the module;
- establish the generic host metering port;
- prohibit host imports and fail-open hard limits through architecture tests;
- keep/test reusable in-memory fakes under `testkit` only.

Gate: `go test ./...`, `go test -race ./...`, example builds without Daptin,
and public API documentation matches the dependency boundary.

### Phase 2: protocol and adapter conformance

- finish strict endpoint contracts, errors, streaming, usage normalization,
  retry/fallback semantics, cache rules, and provider capability declarations;
- certify OpenAI-compatible behavior first, then provider-specific adapters only
  where compatibility translation is insufficient;
- generate support documentation from the tested manifest.

Gate: wire golden tests, malformed-input/fuzz tests, stream disconnect tests,
fault-injection tests, and no unsupported field silently accepted.

### Phase 3: canonical Daptin resources

- update `StandardTables`/`StandardRelations` only;
- modify existing provider DTO/loader in place;
- add model/deployment resources and validate through normal Daptin CRUD;
- test PostgreSQL, MySQL, and SQLite through Daptin's existing schema test
  harness—never copied `CREATE TABLE` statements.

Gate: fresh and existing Daptin databases reconcile through the established
startup path; normal CRUD and relations expose the resources; there is no new
schema framework or physical schema duplicated in tests.

### Phase 4: generic metering reliability

- extend existing `MeteringService` with one idempotent state machine;
- make reservation/finalization atomic under concurrent nodes;
- use it from current CRUD and action interception before connecting LLM;
- test crashes, duplicate finalize/cancel, expiry recovery, boundary windows,
  database errors, and hard-limit fail-closed behavior.

Gate: one evaluator, one ledger, one state machine, race/DB matrix green, and
no read-modify-write oversubscription.

### Phase 5: Daptin adapter and composition

- implement the thin adapter in existing `server/llm` using the inventory in
  this document;
- build and atomically reload module snapshots from Daptin resources;
- wire existing credentials, permissions, metering, Olric/cache, and telemetry;
- construct one engine for HTTP and actions;
- verify no provider call holds a transaction.

Gate: architecture tests, transaction leak tests, invalid reload preservation,
multi-node invalidation recovery, readiness, and shutdown tests pass.

### Phase 6: one-way cutover

- switch `/v1` registration and both LLM actions to the shared engine in one
  change;
- remove `GoAIProvider`, duplicate OpenAI types/translation, async
  `llm_usage`, comma-separated model resolution, and lossy completions behavior;
- remove GoAI dependency if unused;
- remove all old tests that assert deleted behavior and replace them with
  conformance tests—not compatibility shims.

Gate: searches prove no old symbols/path/config flags remain. A clean build has
one constructor, one route resolver, one protocol implementation, and one
metering terminalization path.

### Phase 7: live matrix and operations

- run provider/model/mode/media matrix using rotated external secrets;
- run concurrency, soak, cancellation, failover, cache isolation, and graceful
  shutdown tests;
- publish the verified manifest and operational limits;
- tag the standalone module and pin Daptin to that tag.

The capacity/soak gate uses the existing Daptin subprocess, canonical catalog
creation, authenticated `/v1` request path, deterministic upstream, PostgreSQL
17 connection, and durable generic metering path:

```sh
DAPTIN_LLM_CAPACITY=1 \
DAPTIN_TEST_POSTGRES_DSN='postgres://user:password@host/database?sslmode=require' \
go test . -run '^TestLLMGatewayCapacityPostgres$' -count=1 -v -timeout 20m
```

`DAPTIN_LLM_CAPACITY_DURATION` and `DAPTIN_LLM_CAPACITY_RPS` may raise the
ten-minute and 250-RPS floors for longer soak and saturation runs; they cannot
lower the release gate.

The production-topology execution of this opt-in gate is explicitly skipped for
this implementation completion. The test remains available for later capacity
qualification, and no production-capacity claim is made without running it.

Gate: only tested cells are marked certified; secrets and payloads are absent
from artifacts and logs.

## 14. Verification matrix

### Required dimensions

| Dimension | Values |
|---|---|
| Entry point | HTTP endpoint, `$llm.chat`, `$llm.embedding`, later actions for newly supported operations |
| Provider | Google, OpenRouter, Lilac, local deterministic OpenAI-compatible fixture, each added native adapter |
| Operation | chat, native text completions, Responses, embeddings, image generation; then audio/moderation/rerank |
| Mode | non-stream, stream, tools, parallel tools where supported, structured output, reasoning where supported |
| Media | text, image input, image output; audio only after endpoint support exists |
| Outcome | success, provider 4xx, auth failure, rate limit, timeout, malformed response, disconnect, retryable 5xx |
| Routing | weighted same-tier, priority failover, public-model fallback, unhealthy deployment, saturation |
| Metering | admitted, denied, estimate greater/less than actual, cache hit, retry exposure, cancel, duplicate terminalization |
| Topology | single process, two Daptin nodes, coordination unavailable, database unavailable/recovered |
| Database | SQLite, PostgreSQL, MySQL using existing Daptin test infrastructure |

Every live cell records provider/model/version, requested feature, normalized
result, usage availability, and skip reason. A skip is not a pass. Tests use
small token/media limits and explicit spend caps. OpenRouter image generation
and transcription are explicitly excluded from the required release matrix;
they remain recorded as uncertified rather than being reported as passes.

### Required non-live tests

- protocol golden requests/responses and SSE event streams;
- adapter fixture tests with redacted upstream bodies;
- fuzzing for JSON, SSE, tool arguments, and malformed provider responses;
- deterministic router distribution and failover tests;
- race tests for reload, routing state, cache, and terminalization;
- property tests for fixed-point cost and usage aggregation overflow;
- transaction/pool leak tests under streaming and cancellation;
- cross-tenant cache and authorization tests;
- hard-limit oversubscription test with concurrent nodes;
- architecture/import and duplicate-symbol scans.

## 15. No-duplication review checklist

Every implementation PR answers these questions with file/function links:

1. Which existing Daptin owner is being changed?
2. Which existing functions were reused?
3. Why is each new exported symbol necessary?
4. Does a similarly named converter/loader/JSON helper/config constructor or
   repository already exist?
5. Is any physical schema written outside `StandardTables`/`StandardRelations`?
6. Does the standalone module contain a host concern or metering policy?
7. Do HTTP and actions invoke the exact same engine path?
8. Is there any fallback to old provider/model resolution or usage recording?
9. Can a hard quota allow traffic when its authority is unavailable?
10. Is a transaction open during external I/O?
11. Does each compatibility claim have a conformance test and manifest entry?
12. Can the same request be terminalized twice without double charge?

Any unexplained duplicate blocks merge.

## 16. Definition of done

The implementation is complete only when:

- `github.com/daptin/llmgateway` builds/tests independently and contains no
  Daptin, SQL, Gin, api2go, Olric, or metering-policy implementation;
- Daptin uses its canonical resources, relations, IDs, credentials, config,
  action lifecycle, and existing metering owner;
- no new migration/resource repository/reference/JSON/config abstraction was
  introduced;
- one engine instance serves HTTP and actions;
- no transaction spans external work;
- usage and quota transitions are atomic, idempotent, generic, and multi-node
  correct;
- hard limits fail closed and have no process-local enforcement fallback;
- old GoAI execution, comma-separated routing, async `llm_usage`, and duplicate
  wire types are gone;
- the verified compatibility manifest exactly matches conformance and live
  certification results;
- Google, OpenRouter, Lilac, and deterministic fixture tests cover every
  required operation/mode/media cell without leaking credentials; the two
  explicitly skipped OpenRouter media checks remain uncertified;
- unit, integration, race, fuzz smoke, database matrix, multi-node, and
  graceful-shutdown gates pass; the implemented production-topology soak gate
  is retained but its execution is explicitly skipped;
- searches show no legacy path, feature flag, compatibility shim, duplicated
  helper, or rejected architecture package remains.

## 17. Historical first implementation slice

The first code change after approval should be deliberately small:

1. add the standalone module architecture test;
2. change its admission contract so policy evaluation belongs to the host;
3. delete module-owned quota policy evaluation and fail-open production counter
   fallback;
4. update its testkit and run unit/race tests;
5. make no Daptin application change in that PR.

The second change updates only canonical Daptin resource declarations and
existing provider loading tests. The metering reliability change follows in its
existing owner. Integration happens only after both sides have stable, tested
contracts. This sequencing prevents another large greenfield rewrite and keeps
every architectural decision reviewable.
