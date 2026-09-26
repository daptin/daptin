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
