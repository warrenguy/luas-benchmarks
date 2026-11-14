# Luas, Java and aeron-go comparison

Run `20261018T120000Z` at commit `4f2a9c1e8b7d6053a1e2f3c4d5b6a7980e1f2d3c`. See the README for the method and the differences between clients.

## Host

- Kernel: 7.2.9-example
- CPU: Example CPU @ 3.00GHz
- Governor: performance
- SMT: off
- Isolated CPUs: 2-7
- Driver: aeronmd 1.53.3, commit 377943bcb8
- Luas: v0.0.0-20261018064756-a26d23547a96, commit a26d23547a96
- Java: 1.53.3, commit 377943bcb8
- aeron-go: v0.0.0-20240606170339-8b05ad14e456, commit 8b05ad14e456
- Go: go1.26.4, GOMAXPROCS 4, GOGC unset, GOMEMLIMIT unset, GODEBUG unset
- JDK: 26.0.1+9 OpenJDK 64-Bit Server VM (Eclipse Adoptium), flags: --add-opens java.base/jdk.internal.misc=ALL-UNNAMED

## Run

- Runs per cell: 3, interleaved.
- Messages per run: 600, 20000, 262144, 1000000, 4194304. Warmup messages per run: 100, 2000, 26214, 100000, 419430.
- Cluster messages per run: 20000. Cluster warmup messages per run: 2000.
- Fixed-rate messages per run: 600. Fixed-rate warmup messages per run: 100.
- Fixed rates: 101000, 501000 msg/s. Fixed-rate warmup rates: 25000 msg/s.
- Housekeeping CPUs: 0-1.
- Server CPUs: 8-11.
- Client hot thread CPUs: 6-7.

## Comparison

- Each cell shows the median.
- A dash marks a client with no run.
- A latency cell shows the percentile of the merged histogram of all runs.
- `!` marks a fixed-rate cell where a run missed its send rate or lost replies.

Scenarios: [ipc-throughput](#ipc-throughput-mmsgs) | [ipc-ping-pong](#ipc-ping-pong-us) | [udp-ping-pong](#udp-ping-pong-us) | [ipc-fixed-rate](#ipc-fixed-rate-us) | [udp-fixed-rate](#udp-fixed-rate-us) | [archive-record](#archive-record-mmsgs) | [archive-replay](#archive-replay-mmsgs) | [cluster-ping-pong, 1 member](#cluster-ping-pong-1-member-us) | [cluster-ping-pong, 3 members](#cluster-ping-pong-3-members-us)

### ipc-throughput, Mmsg/s

| Publication | Send | Bytes | Luas | Java | aeron-go |
|---|---|---:|---:|---:|---:|
| concurrent | offer | 32 | 21.5 | 30.1 | 9.92 |
| concurrent | offer | 16384 | 0.305 | 0.420 | 0.150 |
| concurrent | offer | 65536 | 0.0523 | 0.0742 | 0.0365 |
| exclusive | try-claim | 1024 | 4.10 | 5.60 | - |

### ipc-ping-pong, us

| Publication | Send | Bytes | Percentile | Luas | Java | aeron-go |
|---|---|---:|---|---:|---:|---:|
| concurrent | offer | 256 | p50 | 0.312 | 0.285 | 0.455 |
| concurrent | offer | 256 | p90 | 0.345 | 0.305 | 0.495 |
| concurrent | offer | 256 | p99 | 0.430 | 0.395 | 0.620 |
| concurrent | offer | 256 | p99.9 | 1.20 | 0.950 | 1.85 |
| concurrent | offer | 256 | p99.99 | 1.20 | 0.950 | 1.85 |
| concurrent | offer | 256 | p99.999 | 1.20 | 0.950 | 1.85 |

### udp-ping-pong, us

| Publication | Send | Bytes | Percentile | Luas | Java | aeron-go |
|---|---|---:|---|---:|---:|---:|
| concurrent | offer | 32 | p50 | 9.21 | 8.95 | 9.65 |
| concurrent | offer | 32 | p90 | 9.90 | 9.65 | 10.4 |
| concurrent | offer | 32 | p99 | 12.5 | 11.9 | 13.7 |
| concurrent | offer | 32 | p99.9 | 26.1 | 23.1 | 31.0 |
| concurrent | offer | 32 | p99.99 | 26.1 | 23.1 | 31.0 |
| concurrent | offer | 32 | p99.999 | 26.1 | 23.1 | 31.0 |

### ipc-fixed-rate, us

| Publication | Send | Bytes | Rate | Percentile | Luas | Java | aeron-go |
|---|---|---:|---:|---|---:|---:|---:|
| exclusive | try-claim | 32 | 501000 | p50 | 0.523 | 0.466 | 0.920 ! |
| exclusive | try-claim | 32 | 501000 | p90 | 0.605 | 0.546 | 1.07 ! |
| exclusive | try-claim | 32 | 501000 | p99 | 8.89 | 8.45 | 16.7 ! |
| exclusive | try-claim | 32 | 501000 | p99.9 | 11.6 | 10.5 | 20.7 ! |
| exclusive | try-claim | 32 | 501000 | p99.99 | 12.3 | 10.9 | 21.1 ! |
| exclusive | try-claim | 32 | 501000 | p99.999 | 12.3 | 10.9 | 21.1 ! |

### udp-fixed-rate, us

| Publication | Send | Bytes | Rate | Percentile | Luas | Java | aeron-go |
|---|---|---:|---:|---|---:|---:|---:|
| exclusive | try-claim | 1344 | 101000 | p50 | 11.8 | 11.4 | - |
| exclusive | try-claim | 1344 | 101000 | p90 | 13.6 | 13.2 | - |
| exclusive | try-claim | 1344 | 101000 | p99 | 190 | 177 | - |
| exclusive | try-claim | 1344 | 101000 | p99.9 | 227 | 250 | - |
| exclusive | try-claim | 1344 | 101000 | p99.99 | 227 | 269 | - |
| exclusive | try-claim | 1344 | 101000 | p99.999 | 227 | 269 | - |

### archive-record, Mmsg/s

| Publication | Send | Bytes | Luas | Java | aeron-go |
|---|---|---:|---:|---:|---:|
| concurrent | offer | 256 | 6.00 | 7.00 | 4.50 |

### archive-replay, Mmsg/s

| Publication | Send | Bytes | Luas | Java | aeron-go |
|---|---|---:|---:|---:|---:|
| concurrent | offer | 4096 | 1.10 | 1.30 | 0.900 |

### cluster-ping-pong, 1 member, us

| Publication | Send | Bytes | Percentile | Luas | Java | aeron-go |
|---|---|---:|---|---:|---:|---:|
| exclusive | offer | 256 | p50 | 18.5 | - | - |
| exclusive | offer | 256 | p90 | 20.5 | - | - |
| exclusive | offer | 256 | p99 | 24.5 | - | - |
| exclusive | offer | 256 | p99.9 | 38.5 | - | - |
| exclusive | offer | 256 | p99.99 | 38.5 | - | - |
| exclusive | offer | 256 | p99.999 | 419 | - | - |

### cluster-ping-pong, 3 members, us

| Publication | Send | Bytes | Percentile | Luas | Java | aeron-go |
|---|---|---:|---|---:|---:|---:|
| exclusive | offer | 32 | p50 | 42.3 | 40.1 | 46.1 |
| exclusive | offer | 32 | p90 | 44.3 | 42.1 | 48.1 |
| exclusive | offer | 32 | p99 | 48.3 | 46.1 | 52.1 |
| exclusive | offer | 32 | p99.9 | 62.3 | 60.1 | 66.1 |
| exclusive | offer | 32 | p99.99 | 62.3 | 60.1 | 66.1 |
| exclusive | offer | 32 | p99.999 | 442 | 440 | 446 |

## Results

Each cell shows the median, then the min-max spread. `n=` marks a cell with fewer runs. A dash marks a cell with no run. A latency cell shows the percentile of the merged histogram, then the min-max spread of the runs. `!` marks a fixed-rate cell where a run missed its send rate or lost replies.

| Scenario | Variant | Metric | Luas | Java | aeron-go |
|---|---|---|---|---|---|
| ipc-throughput | concurrent-offer-32 | Mmsg/s | 21.5 (20.1-22.3) | 30.1 (29.9-31.0) | 9.92 (9.80-10.1) |
| ipc-throughput | concurrent-offer-16384 | Mmsg/s | 0.305 (0.298-0.310) | 0.420 (0.415-0.431) | 0.150 (0.149-0.152) |
| ipc-throughput | concurrent-offer-65536 | Mmsg/s | 0.0523 (0.0518-0.0531) | 0.0742 (0.0738-0.0751) | 0.0365 (0.0359-0.0372) |
| ipc-throughput | exclusive-try-claim-1024 | Mmsg/s | 4.10 (4.01-4.23) | 5.60 (5.55-5.72) | - |
| ipc-ping-pong | concurrent-offer-256 | p50 us | 0.312 (0.305-0.312) | 0.285 (0.280-0.285) n=2 | 0.455 (0.448-0.455) |
| ipc-ping-pong | concurrent-offer-256 | p90 us | 0.345 (0.335-0.345) | 0.305 (0.300-0.305) n=2 | 0.495 (0.488-0.495) |
| ipc-ping-pong | concurrent-offer-256 | p99 us | 0.430 (0.415-0.430) | 0.395 (0.390-0.395) n=2 | 0.620 (0.605-0.620) |
| ipc-ping-pong | concurrent-offer-256 | p99.9 us | 1.20 (1.05-1.20) | 0.950 (0.900-0.950) n=2 | 1.85 (1.79-1.85) |
| ipc-ping-pong | concurrent-offer-256 | p99.99 us | 1.20 (1.05-1.20) | 0.950 (0.900-0.950) n=2 | 1.85 (1.79-1.85) |
| ipc-ping-pong | concurrent-offer-256 | p99.999 us | 1.20 (1.05-1.20) | 0.950 (0.900-0.950) n=2 | 1.85 (1.79-1.85) |
| ipc-ping-pong | concurrent-offer-256 | max us | 31.0 (22.0-31.0) | 41.0 (38.0-41.0) n=2 | 61.0 (50.5-61.0) |
| udp-ping-pong | concurrent-offer-32 | p50 us | 9.21 (9.05-9.21) | 8.95 (8.87-8.95) | 9.65 (9.58-9.65) |
| udp-ping-pong | concurrent-offer-32 | p90 us | 9.90 (9.75-9.90) | 9.65 (9.58-9.65) | 10.4 (10.3-10.4) |
| udp-ping-pong | concurrent-offer-32 | p99 us | 12.5 (12.0-12.5) | 11.9 (11.8-11.9) | 13.7 (13.4-13.7) |
| udp-ping-pong | concurrent-offer-32 | p99.9 us | 26.1 (24.8-26.1) | 23.1 (21.9-23.1) | 31.0 (29.9-31.0) |
| udp-ping-pong | concurrent-offer-32 | p99.99 us | 26.1 (24.8-26.1) | 23.1 (21.9-23.1) | 31.0 (29.9-31.0) |
| udp-ping-pong | concurrent-offer-32 | p99.999 us | 26.1 (24.8-26.1) | 23.1 (21.9-23.1) | 31.0 (29.9-31.0) |
| udp-ping-pong | concurrent-offer-32 | max us | 143 (98.0-143) | 210 (180-210) | 171 (155-171) |
| ipc-fixed-rate | exclusive-try-claim-32-rate-501000 | p50 us | 0.523 (0.518-0.531) | 0.466 (0.459-0.474) | 0.920 (0.903-0.946) ! |
| ipc-fixed-rate | exclusive-try-claim-32-rate-501000 | p90 us | 0.605 (0.599-0.615) | 0.546 (0.539-0.552) | 1.07 (1.06-1.08) ! |
| ipc-fixed-rate | exclusive-try-claim-32-rate-501000 | p99 us | 8.89 (7.43-9.75) | 8.45 (8.20-8.54) | 16.7 (15.0-17.2) ! |
| ipc-fixed-rate | exclusive-try-claim-32-rate-501000 | p99.9 us | 11.6 (8.96-12.3) | 10.5 (10.4-10.9) | 20.7 (19.0-21.1) ! |
| ipc-fixed-rate | exclusive-try-claim-32-rate-501000 | p99.99 us | 12.3 (8.96-12.3) | 10.9 (10.4-10.9) | 21.1 (19.0-21.1) ! |
| ipc-fixed-rate | exclusive-try-claim-32-rate-501000 | p99.999 us | 12.3 (8.96-12.3) | 10.9 (10.4-10.9) | 21.1 (19.0-21.1) ! |
| ipc-fixed-rate | exclusive-try-claim-32-rate-501000 | max us | 12.3 (8.96-12.3) | 10.9 (10.4-10.9) | 21.1 (19.0-21.1) ! |
| udp-fixed-rate | exclusive-try-claim-1344-rate-101000 | p50 us | 11.8 (11.7-11.8) | 11.4 (11.4-11.5) n=2 | - |
| udp-fixed-rate | exclusive-try-claim-1344-rate-101000 | p90 us | 13.6 (13.5-13.7) | 13.2 (13.2-13.3) n=2 | - |
| udp-fixed-rate | exclusive-try-claim-1344-rate-101000 | p99 us | 190 (181-191) | 177 (175-186) n=2 | - |
| udp-fixed-rate | exclusive-try-claim-1344-rate-101000 | p99.9 us | 227 (215-227) | 250 (228-269) n=2 | - |
| udp-fixed-rate | exclusive-try-claim-1344-rate-101000 | p99.99 us | 227 (215-227) | 269 (228-269) n=2 | - |
| udp-fixed-rate | exclusive-try-claim-1344-rate-101000 | p99.999 us | 227 (215-227) | 269 (228-269) n=2 | - |
| udp-fixed-rate | exclusive-try-claim-1344-rate-101000 | max us | 227 (215-227) | 269 (228-269) n=2 | - |
| archive-record | concurrent-offer-256 | Mmsg/s | 6.00 (5.80-6.20) | 7.00 (6.90-7.20) | 4.50 (4.40-4.70) |
| archive-replay | concurrent-offer-4096 | Mmsg/s | 1.10 (1.05-1.12) | 1.30 (1.28-1.33) | 0.900 (0.880-0.930) |
| cluster-ping-pong | exclusive-offer-256-members-1 | p50 us | 18.5 (18.1-18.5) | - | - |
| cluster-ping-pong | exclusive-offer-256-members-1 | p90 us | 20.5 (20.1-20.5) | - | - |
| cluster-ping-pong | exclusive-offer-256-members-1 | p99 us | 24.5 (24.1-24.5) | - | - |
| cluster-ping-pong | exclusive-offer-256-members-1 | p99.9 us | 38.5 (38.1-38.5) | - | - |
| cluster-ping-pong | exclusive-offer-256-members-1 | p99.99 us | 38.5 (38.1-38.5) | - | - |
| cluster-ping-pong | exclusive-offer-256-members-1 | p99.999 us | 419 (418-419) | - | - |
| cluster-ping-pong | exclusive-offer-256-members-1 | max us | 419 (418-419) | - | - |
| cluster-ping-pong | exclusive-offer-32-members-3 | p50 us | 42.3 (40.5-42.3) | 40.1 (39.5-40.1) | 46.1 (44.8-46.1) |
| cluster-ping-pong | exclusive-offer-32-members-3 | p90 us | 44.3 (42.5-44.3) | 42.1 (41.5-42.1) | 48.1 (46.8-48.1) |
| cluster-ping-pong | exclusive-offer-32-members-3 | p99 us | 48.3 (46.5-48.3) | 46.1 (45.5-46.1) | 52.1 (50.8-52.1) |
| cluster-ping-pong | exclusive-offer-32-members-3 | p99.9 us | 62.3 (60.5-62.3) | 60.1 (59.5-60.1) | 66.1 (64.8-66.1) |
| cluster-ping-pong | exclusive-offer-32-members-3 | p99.99 us | 62.3 (60.5-62.3) | 60.1 (59.5-60.1) | 66.1 (64.8-66.1) |
| cluster-ping-pong | exclusive-offer-32-members-3 | p99.999 us | 442 (441-442) | 440 (440-440) | 446 (445-446) |
| cluster-ping-pong | exclusive-offer-32-members-3 | max us | 442 (441-442) | 440 (440-440) | 446 (445-446) |
