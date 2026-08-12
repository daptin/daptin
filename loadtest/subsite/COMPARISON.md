# Precompressed static-serving comparison

Measured on the same Apple M1 Max system and with the same fixtures described in
[BASELINE.md](BASELINE.md).

## Single-core microbenchmarks

| Workload | Before ops/s | After ops/s | Change | Before B/op | After B/op |
|---|---:|---:|---:|---:|---:|
| Cached `index.html` | 145,760 | 161,441 | +10.8% | 10,535 | 10,535 |
| Cached 64 KiB JavaScript | 8,825 | 15,060 | +70.7% | 266,075 | 265,079 |
| Cached 64 KiB JavaScript + GZIP | 2,621 | 31,153 | **11.9x** | 851,971 | **4,304** |
| ETag `304 Not Modified` | 58,346 | 60,179 | +3.1% | 2,671 | 3,082 |
| 1 KiB byte range | 28,720 | 46,295 | +61.2% | 6,567 | 5,527 |

The compressed workload now reuses a sidecar representation rather than creating
a gzip stream on every request. Its measured allocation volume fell by 99.5%.

## Loopback HTTP at concurrency 100

The response fixture was the same 46,946-byte minified JavaScript asset.

| Workload | Before RPS | After RPS | Before p99 | After p99 |
|---|---:|---:|---:|---:|
| Uncompressed | 37,188 | 51,764 | 13.83 ms | 10.22 ms |
| GZIP | 4,384 | **46,044** | 131.52 ms | **5.62 ms** |

In the repeated 10-second GZIP run, throughput increased from 4,583 to 45,898
requests/s. Observed server RSS after load fell from approximately 447 MiB to
119 MiB. CPU utilization remained around five to seven cores, but it delivered
roughly ten times as many responses.

These are local comparison results, not production capacity guarantees. The
server and generator shared one machine and the standalone server excludes TLS,
authentication, distributed rate limiting, and cloud-cache misses.
