# Benchmark results

`go run ./report -comparison` generates this file. See the README for the method and the differences between clients.

Instances: [Example CPU @ 3.00GHz](#example-cpu--300ghz)

- Each cell shows the median.
- A dash marks a client with no run.
- A latency cell shows the percentile of the merged histogram of all runs.
- `!` marks a fixed-rate cell where a run missed its send rate or lost replies.

## Example CPU @ 3.00GHz

- Run: `20261018T120000Z` at commit `4f2a9c1e8b7d6053a1e2f3c4d5b6a7980e1f2d3c`. Runs per cell: 3, interleaved.
- Kernel: 7.2.9-example
- CPU: Example CPU @ 3.00GHz
- Governor: performance
- SMT: off
- Isolated CPUs: 2-7
- Driver: aeronmd 1.53.3, commit 377943bcb8
- Luas: v0.0.0-20261018064756-a26d23547a96, commit a26d23547a96
- Aeron (Java): 1.53.3, commit 377943bcb8
- aeron-go: v0.0.0-20240606170339-8b05ad14e456, commit 8b05ad14e456
- Go: go1.26.4, GOMAXPROCS 4, GOGC unset, GOMEMLIMIT unset, GODEBUG unset
- JDK: 26.0.1+9 OpenJDK 64-Bit Server VM (Eclipse Adoptium), flags: --add-opens java.base/jdk.internal.misc=ALL-UNNAMED

Scenarios: [ipc-throughput](#ipc-throughput-mmsgs) | [ipc-ping-pong](#ipc-ping-pong-us) | [udp-ping-pong](#udp-ping-pong-us) | [ipc-fixed-rate](#ipc-fixed-rate-us) | [udp-fixed-rate](#udp-fixed-rate-us) | [archive-record](#archive-record-mmsgs) | [archive-replay](#archive-replay-mmsgs) | [cluster-ping-pong, 1 member](#cluster-ping-pong-1-member-us) | [cluster-ping-pong, 3 members](#cluster-ping-pong-3-members-us)

### ipc-throughput, Mmsg/s

| Publication | Send | Bytes | Luas | Aeron (Java) | aeron-go |
|---|---|---:|---:|---:|---:|
| concurrent | offer | 32 | 21.5 | 30.1 | 9.92 |
| concurrent | offer | 16384 | 0.305 | 0.420 | 0.150 |
| concurrent | offer | 65536 | 0.0523 | 0.0742 | 0.0365 |
| exclusive | try-claim | 1024 | 4.10 | 5.60 | - |

### ipc-ping-pong, us

| Publication | Send | Bytes | Percentile | Luas | Aeron (Java) | aeron-go |
|---|---|---:|---|---:|---:|---:|
| concurrent | offer | 256 | p50 | 0.312 | 0.285 | 0.455 |
| concurrent | offer | 256 | p90 | 0.345 | 0.305 | 0.495 |
| concurrent | offer | 256 | p99 | 0.430 | 0.395 | 0.620 |
| concurrent | offer | 256 | p99.9 | 1.20 | 0.950 | 1.85 |
| concurrent | offer | 256 | p99.99 | 1.20 | 0.950 | 1.85 |
| concurrent | offer | 256 | p99.999 | 1.20 | 0.950 | 1.85 |

### udp-ping-pong, us

| Publication | Send | Bytes | Percentile | Luas | Aeron (Java) | aeron-go |
|---|---|---:|---|---:|---:|---:|
| concurrent | offer | 32 | p50 | 9.21 | 8.95 | 9.65 |
| concurrent | offer | 32 | p90 | 9.90 | 9.65 | 10.4 |
| concurrent | offer | 32 | p99 | 12.5 | 11.9 | 13.7 |
| concurrent | offer | 32 | p99.9 | 26.1 | 23.1 | 31.0 |
| concurrent | offer | 32 | p99.99 | 26.1 | 23.1 | 31.0 |
| concurrent | offer | 32 | p99.999 | 26.1 | 23.1 | 31.0 |

### ipc-fixed-rate, us

| Publication | Send | Bytes | Rate | Percentile | Luas | Aeron (Java) | aeron-go |
|---|---|---:|---:|---|---:|---:|---:|
| exclusive | try-claim | 32 | 501000 | p50 | 0.523 | 0.466 | 0.920 ! |
| exclusive | try-claim | 32 | 501000 | p90 | 0.605 | 0.546 | 1.07 ! |
| exclusive | try-claim | 32 | 501000 | p99 | 8.89 | 8.45 | 16.7 ! |
| exclusive | try-claim | 32 | 501000 | p99.9 | 11.6 | 10.5 | 20.7 ! |
| exclusive | try-claim | 32 | 501000 | p99.99 | 12.3 | 10.9 | 21.1 ! |
| exclusive | try-claim | 32 | 501000 | p99.999 | 12.3 | 10.9 | 21.1 ! |

### udp-fixed-rate, us

| Publication | Send | Bytes | Rate | Percentile | Luas | Aeron (Java) | aeron-go |
|---|---|---:|---:|---|---:|---:|---:|
| exclusive | try-claim | 1344 | 101000 | p50 | 11.8 | 11.4 | - |
| exclusive | try-claim | 1344 | 101000 | p90 | 13.6 | 13.2 | - |
| exclusive | try-claim | 1344 | 101000 | p99 | 190 | 177 | - |
| exclusive | try-claim | 1344 | 101000 | p99.9 | 227 | 250 | - |
| exclusive | try-claim | 1344 | 101000 | p99.99 | 227 | 269 | - |
| exclusive | try-claim | 1344 | 101000 | p99.999 | 227 | 269 | - |

### archive-record, Mmsg/s

| Publication | Send | Bytes | Luas | Aeron (Java) | aeron-go |
|---|---|---:|---:|---:|---:|
| concurrent | offer | 256 | 6.00 | 7.00 | 4.50 |

### archive-replay, Mmsg/s

| Publication | Send | Bytes | Luas | Aeron (Java) | aeron-go |
|---|---|---:|---:|---:|---:|
| concurrent | offer | 4096 | 1.10 | 1.30 | 0.900 |

### cluster-ping-pong, 1 member, us

| Publication | Send | Bytes | Percentile | Luas | Aeron (Java) | aeron-go |
|---|---|---:|---|---:|---:|---:|
| exclusive | offer | 256 | p50 | 18.5 | - | - |
| exclusive | offer | 256 | p90 | 20.5 | - | - |
| exclusive | offer | 256 | p99 | 24.5 | - | - |
| exclusive | offer | 256 | p99.9 | 38.5 | - | - |
| exclusive | offer | 256 | p99.99 | 38.5 | - | - |
| exclusive | offer | 256 | p99.999 | 419 | - | - |

### cluster-ping-pong, 3 members, us

| Publication | Send | Bytes | Percentile | Luas | Aeron (Java) | aeron-go |
|---|---|---:|---|---:|---:|---:|
| exclusive | offer | 32 | p50 | 42.3 | 40.1 | 46.1 |
| exclusive | offer | 32 | p90 | 44.3 | 42.1 | 48.1 |
| exclusive | offer | 32 | p99 | 48.3 | 46.1 | 52.1 |
| exclusive | offer | 32 | p99.9 | 62.3 | 60.1 | 66.1 |
| exclusive | offer | 32 | p99.99 | 62.3 | 60.1 | 66.1 |
| exclusive | offer | 32 | p99.999 | 442 | 440 | 446 |
