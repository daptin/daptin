# Current subsite-serving baseline

Measured on 2026-08-12 before the proposed static-serving optimizations.

## Environment

- Commit under test: `f69d6f35` plus the load-test harness
- Hardware: Apple M1 Max, 10 logical CPUs, 32 GiB RAM
- OS: macOS 15.4, arm64
- Go: 1.25.0
- Microbenchmark setting: `-cpu=1`, five samples, two seconds per sample
- Loopback setting: server and load generator on the same machine

These results measure the subsite engine and cached local-file handler. The
standalone server intentionally omits database initialization, cloud misses,
authentication, distributed rate limiting, and TLS. It is suitable for
before/after comparisons of the serving path, not as a production capacity claim.

## In-process microbenchmark

Values are arithmetic means of the five samples in
`results/baseline-v0.12.35-pre-cpu1.txt`.

| Workload | Approx. ops/s | ns/op | B/op | allocs/op |
|---|---:|---:|---:|---:|
| Cached `index.html` | 145,760 | 6,861 | 10,535 | 42 |
| Cached 64 KiB JavaScript | 8,825 | 113,310 | 266,075 | 66 |
| Cached 64 KiB JavaScript + runtime GZIP | 2,621 | 381,530 | 851,971 | 86 |
| ETag `304 Not Modified` | 58,346 | 17,139 | 2,671 | 43 |
| 1 KiB byte range | 28,720 | 34,819 | 6,567 | 71 |

The benchmark uses `httptest.ResponseRecorder`, so its response buffering is
included in allocations. Absolute allocation totals are therefore higher than a
real socket response, but remain useful when comparing the same benchmark before
and after a change.

## Loopback HTTP baseline

The fixture was the existing 46,946-byte minified documentation JavaScript file.
Each case ran for five seconds. Full machine-readable results are under `results/`.

### Uncompressed

| Concurrency | Requests/s | p50 | p95 | p99 | Errors |
|---:|---:|---:|---:|---:|---:|
| 1 | 8,485 | 0.095 ms | 0.138 ms | 0.228 ms | 0 |
| 10 | 32,402 | 0.220 ms | 0.568 ms | 1.287 ms | 0 |
| 100 | 37,188 | 1.812 ms | 8.007 ms | 13.830 ms | 0 |

### Runtime GZIP

| Concurrency | Requests/s | p50 | p95 | p99 | Errors |
|---:|---:|---:|---:|---:|---:|
| 1 | 775 | 1.143 ms | 2.010 ms | 2.456 ms | 0 |
| 10 | 4,199 | 1.950 ms | 4.347 ms | 6.594 ms | 0 |
| 100 | 4,384 | 9.266 ms | 85.502 ms | 131.520 ms | 0 |

In a separate 10-second runtime-GZIP run at concurrency 100, the server produced
4,583 requests/s. Its resident memory rose from 65,568 KiB idle to approximately
447,120 KiB after the run, while `ps` reported roughly 550–670% CPU during/at the
end of the run. This is a coarse process-level observation, but it confirms that
runtime compression is the first optimization target.

## Comparison rules

Use the same fixture, commands, `-cpu` value, machine, and power conditions. Run
at least five samples and compare distributions rather than a single best result.
For network capacity tests, move the generator to separate hosts and record server
CPU, RSS, network throughput, open file descriptors, errors, and p99 latency.
