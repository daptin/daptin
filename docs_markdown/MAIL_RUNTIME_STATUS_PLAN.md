# Mail Runtime Status

Implementation of [issue 336](https://github.com/daptin/daptin/issues/336).
The supported API and operational contract are documented in
[SMTP Server](../wiki/SMTP-Server.md#mail-service-runtime-status).

## Verified invariant

An authenticated administrator can inspect the current SMTP and IMAP listeners
independently of saved configuration and HTTP readiness. The occupied-port E2E
case proves that IMAP can fail to bind while HTTP `/ready` returns 200; the mail
status action reports that failure. Freeing the port and restarting produces a
new runtime identity, a running listener, and no stale error.

## Ownership and security

The go-guerrilla daemon owns SMTP listener observations, including reload and
shutdown. Daptin owns IMAP observations at bind, serve termination, and quiesce.
`Runtime` owns their lifetime. One local action performer combines copied
observations; it neither probes arbitrary sockets nor infers service state from
saved settings. The existing runtime instance ID also identifies the snapshot.

The wrapper action uses Daptin's administrator-only permission and access group.
The performer separately reloads persisted membership and applies
`PermissionInstance.CanExecute` with the administrator-group authority. This
prevents transient action privileges from exposing diagnostics through a custom
wrapper and follows the active account after `SWITCH_USER`.

Durable settings remain in resources and `ConfigStore`. No new status table,
polling worker, shared resource signature, credential authority, or public
health endpoint is introduced. Fixed diagnostic codes/messages exclude backend
error strings and secrets. SMTP startup errors retain the existing fatal
boundary; optional IMAP startup failure retains HTTP availability.

## Production scope

| File | Purpose |
| --- | --- |
| `server/mail_service_status.go` | Copies owned lifecycle observations and enforces persisted administrator membership at the performer boundary. |
| `server/endpoint_imap.go` | Records the actual TLS load, bind, serving, and failure observations. |
| `server/runtime.go` | Updates status during the existing owned shutdown. |
| `server/server.go` | Creates the runtime status authority and registers the performer in the existing action map. |
| `server/resource/columns.go` | Defines the trusted system action with administrator access-group permissions. |
| `go.mod`, `go.sum` | Pin the upstream listener snapshot API together with the LOGIN credential fix. |

## Validation

Focused unit/race tests cover copied snapshots, concurrent lifecycle reads,
unexpected termination, denied identity context, and quiesce. Executable E2E
coverage uses resource and relationship APIs for administrator grant/revocation,
wrapper and switched-account denial, disabled services, environment suppression,
occupied IMAP startup, missing certificate keys, restart recovery, verified TLS handshakes, and an SMTP
reload where one listener fails while another remains running. The separate
SMTP LOGIN E2E suite continues to cover both credential exchange forms.

The upstream root suite retains pre-existing constructor-call compilation
failures. Its focused production-file protocol and listener lifecycle tests
provide the runnable regression coverage for these dependency changes.
