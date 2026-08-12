# Subsite performance baseline

This harness measures the cached subsite response path without requiring a
database, cloud provider, or third-party load-test binary.

## Microbenchmarks

Run the actual Gin subsite router and handler in process:

```bash
go test ./server \
  -run '^$' \
  -bench '^BenchmarkSubsite' \
  -benchmem \
  -benchtime=3s \
  -cpu=1 \
  -count=5
```

Keep `-cpu`, Go version, hardware, power mode, and benchmark duration the
same when comparing changes. Save output to a dated file when needed:

```bash
mkdir -p loadtest/subsite/results
go test ./server -run '^$' -bench '^BenchmarkSubsite' \
  -benchmem -benchtime=3s -cpu=1 -count=5 | \
  tee loadtest/subsite/results/baseline.txt
```

The workloads cover cached index HTML, a 64 KiB JavaScript asset, runtime GZIP,
an ETag `304 Not Modified`, and a 1 KiB byte range.

The initial measured results and environment are recorded in [BASELINE.md](BASELINE.md).
Results after adding reusable compressed representations and single-open serving
are recorded in [COMPARISON.md](COMPARISON.md).

## Loopback load generator

The standard-library load generator can measure an independently running Daptin
subsite router:

```bash
go run ./loadtest/subsite \
  -url http://127.0.0.1:6336/app.js \
  -header 'Host: example.test' \
  -duration 15s \
  -concurrency 100 \
  -output loadtest/subsite/results/current.json
```

It reports completed requests per second, success/error counts, p50/p95/p99/max
latency, response throughput, Go version, architecture, and `GOMAXPROCS`.

Create representative assets and start the subsite router in one terminal:

```bash
mkdir -p /tmp/daptin-loadtest
dd if=/dev/zero of=/tmp/daptin-loadtest/app.js bs=65536 count=1
go run ./loadtest/subsite -mode serve \
  -dir /tmp/daptin-loadtest \
  -listen 127.0.0.1:18080
```

Then target it from another terminal. For production-scale testing, run the load
generator on separate machines so client CPU and loopback networking do not cap
the measured server.
