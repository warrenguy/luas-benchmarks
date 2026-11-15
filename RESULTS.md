# Benchmark results

`go run ./report -comparison` generates this file. See the README for the method and the differences between clients.

Instances: [amd64 m8azn.6xlarge](#amd64-m8azn6xlarge) | [Arm c9g.8xlarge](#arm-c9g8xlarge)

- Each cell shows the median.
- A dash marks a client with no run.
- A latency cell shows the percentile of the merged histogram of all runs.
- `!` marks a fixed-rate cell where a run missed its send rate or lost replies.

## amd64 m8azn.6xlarge

- Run: `20261111T123648Z` at commit `838dc88d69641f8508781b13d0c9fb148f3aef1a`. Runs per cell: 15 in ipc-throughput, udp-throughput; 5 in ipc-ping-pong, udp-ping-pong, ipc-fixed-rate, udp-fixed-rate, archive-record, archive-replay, cluster-ping-pong, interleaved.
- Kernel: 7.0.0-1014-aws
- CPU: AMD EPYC 9R05
- Governor: unknown
- SMT: notsupported
- Isolated CPUs: 1-17
- Driver: aeronmd 1.53.3, commit 377943bcb8+guilty
- Luas: v0.0.0-20261110170533-5d8f75734b75, commit 5d8f75734b75
- Aeron (Java): 1.53.3, commit 377943bcb8
- aeron-go: v0.0.0-20240606170339-8b05ad14e456, commit 8b05ad14e456
- Go: go1.26.4, GOMAXPROCS 9, GOGC unset, GOMEMLIMIT unset, GODEBUG unset
- Luas layout: hot goroutines unlocked, yield every 5 ms, GOMAXPROCS 2, in cluster-ping-pong, ipc-fixed-rate, ipc-ping-pong, udp-fixed-rate, udp-ping-pong
- JDK: 26.0.2.1+1 OpenJDK 64-Bit Server VM (Eclipse Adoptium), flags: -XX:+UnlockExperimentalVMOptions -XX:+TrustFinalNonStaticFields -XX:+UseParallelGC -Dagrona.disable.bounds.checks=true -Daeron.client.idle.sleep.duration=1ms -Daeron.pre.touch.mapped.memory=true --add-opens java.base/jdk.internal.misc=ALL-UNNAMED

Scenarios: [ipc-throughput](#ipc-throughput-mmsgs) | [udp-throughput](#udp-throughput-mmsgs) | [ipc-ping-pong](#ipc-ping-pong-us) | [udp-ping-pong](#udp-ping-pong-us) | [ipc-fixed-rate](#ipc-fixed-rate-us) | [udp-fixed-rate](#udp-fixed-rate-us) | [archive-record](#archive-record-mmsgs) | [archive-replay](#archive-replay-mmsgs) | [cluster-ping-pong, 1 member](#cluster-ping-pong-1-member-us) | [cluster-ping-pong, 3 members](#cluster-ping-pong-3-members-us)

### ipc-throughput, Mmsg/s

| Publication | Send | Bytes | Luas | Aeron (Java) | aeron-go |
|---|---|---:|---:|---:|---:|
| concurrent | offer | 32 | 30.5 | 33.6 | 11.6 |
| concurrent | offer | 256 | 27.6 | 26.4 | 6.39 |
| concurrent | offer | 1024 | 25.1 | 23.4 | 3.00 |
| concurrent | offer | 4096 | 5.05 | 5.56 | 0.169 |
| concurrent | offer | 16384 | 1.20 | 1.53 | 0.0427 |
| concurrent | offer | 65536 | 0.291 | 0.304 | 0.0107 |
| concurrent | try-claim | 32 | 26.5 | 33.6 | 11.7 |
| concurrent | try-claim | 256 | 23.1 | 25.6 | 6.50 |
| concurrent | try-claim | 1024 | 21.6 | 22.8 | 3.05 |
| exclusive | offer | 32 | 132 | 79.3 | 11.6 |
| exclusive | offer | 256 | 57.2 | 56.2 | 6.41 |
| exclusive | offer | 1024 | 34.8 | 34.7 | 3.01 |
| exclusive | offer | 4096 | 5.25 | 5.59 | 0.169 |
| exclusive | offer | 16384 | 1.23 | 1.51 | 0.0427 |
| exclusive | offer | 65536 | 0.252 | 0.305 | 0.0107 |
| exclusive | try-claim | 32 | 77.3 | 80.8 | 11.7 |
| exclusive | try-claim | 256 | 53.8 | 57.3 | 6.54 |
| exclusive | try-claim | 1024 | 33.0 | 35.8 | 3.04 |

### udp-throughput, Mmsg/s

| Publication | Send | Bytes | Luas | Aeron (Java) | aeron-go |
|---|---|---:|---:|---:|---:|
| concurrent | offer | 32 | 18.5 | 18.8 | 15.2 |
| concurrent | offer | 256 | 3.22 | 3.27 | 3.23 |
| concurrent | offer | 1024 | 0.868 | 0.868 | 0.866 |
| concurrent | offer | 4096 | 0.268 | 0.261 | 0.169 |
| concurrent | offer | 16384 | 0.0686 | 0.0688 | 0.0427 |
| concurrent | offer | 65536 | 0.0175 | 0.0175 | 0.0107 |
| concurrent | try-claim | 32 | 18.5 | 18.6 | 15.5 |
| concurrent | try-claim | 256 | 3.24 | 3.24 | 3.24 |
| concurrent | try-claim | 1024 | 0.851 | 0.860 | 0.852 |
| exclusive | offer | 32 | 18.3 | 18.6 | 15.2 |
| exclusive | offer | 256 | 3.23 | 3.25 | 3.21 |
| exclusive | offer | 1024 | 0.853 | 0.855 | 0.853 |
| exclusive | offer | 4096 | 0.265 | 0.259 | 0.170 |
| exclusive | offer | 16384 | 0.0684 | 0.0684 | 0.0428 |
| exclusive | offer | 65536 | 0.0176 | 0.0176 | 0.0107 |
| exclusive | try-claim | 32 | 18.3 | 18.5 | 15.5 |
| exclusive | try-claim | 256 | 3.24 | 3.27 | 3.26 |
| exclusive | try-claim | 1024 | 0.863 | 0.862 | 0.860 |

### ipc-ping-pong, us

| Publication | Send | Bytes | Percentile | Luas | Aeron (Java) | aeron-go |
|---|---|---:|---|---:|---:|---:|
| concurrent | offer | 32 | p50 | 0.170 | 0.170 | 0.270 |
| concurrent | offer | 32 | p90 | 0.200 | 0.190 | 0.340 |
| concurrent | offer | 32 | p99 | 0.270 | 0.280 | 1.07 |
| concurrent | offer | 32 | p99.9 | 0.360 | 0.790 | 2.53 |
| concurrent | offer | 32 | p99.99 | 0.720 | 1.12 | 3.63 |
| concurrent | offer | 32 | p99.999 | 3.13 | 5.76 | 26.7 |
| concurrent | offer | 256 | p50 | 0.200 | 0.190 | 0.460 |
| concurrent | offer | 256 | p90 | 0.260 | 0.230 | 0.530 |
| concurrent | offer | 256 | p99 | 0.370 | 0.590 | 1.43 |
| concurrent | offer | 256 | p99.9 | 0.460 | 0.940 | 3.45 |
| concurrent | offer | 256 | p99.99 | 1.08 | 2.36 | 4.33 |
| concurrent | offer | 256 | p99.999 | 3.55 | 12.0 | 32.6 |
| concurrent | offer | 1024 | p50 | 0.250 | 0.210 | 0.830 |
| concurrent | offer | 1024 | p90 | 0.310 | 0.250 | 0.880 |
| concurrent | offer | 1024 | p99 | 0.450 | 0.400 | 1.73 |
| concurrent | offer | 1024 | p99.9 | 0.630 | 0.630 | 3.39 |
| concurrent | offer | 1024 | p99.99 | 2.21 | 1.14 | 4.71 |
| concurrent | offer | 1024 | p99.999 | 6.61 | 9.46 | 36.5 |

### udp-ping-pong, us

| Publication | Send | Bytes | Percentile | Luas | Aeron (Java) | aeron-go |
|---|---|---:|---|---:|---:|---:|
| concurrent | offer | 32 | p50 | 2.71 | 2.68 | 2.73 |
| concurrent | offer | 32 | p90 | 2.95 | 2.94 | 2.96 |
| concurrent | offer | 32 | p99 | 3.02 | 3.00 | 3.60 |
| concurrent | offer | 32 | p99.9 | 3.35 | 3.39 | 4.39 |
| concurrent | offer | 32 | p99.99 | 5.05 | 5.11 | 27.2 |
| concurrent | offer | 32 | p99.999 | 5.94 | 13.2 | 34.1 |
| concurrent | offer | 256 | p50 | 2.77 | 2.70 | 2.96 |
| concurrent | offer | 256 | p90 | 2.98 | 2.97 | 3.23 |
| concurrent | offer | 256 | p99 | 3.12 | 3.09 | 3.51 |
| concurrent | offer | 256 | p99.9 | 3.97 | 3.98 | 4.99 |
| concurrent | offer | 256 | p99.99 | 5.51 | 5.59 | 27.8 |
| concurrent | offer | 256 | p99.999 | 6.72 | 17.7 | 35.0 |
| concurrent | offer | 1024 | p50 | 2.90 | 2.85 | 3.47 |
| concurrent | offer | 1024 | p90 | 3.11 | 3.08 | 3.70 |
| concurrent | offer | 1024 | p99 | 3.35 | 3.33 | 3.93 |
| concurrent | offer | 1024 | p99.9 | 4.53 | 4.53 | 5.95 |
| concurrent | offer | 1024 | p99.99 | 6.35 | 6.58 | 33.0 |
| concurrent | offer | 1024 | p99.999 | 7.51 | 15.6 | 37.2 |

### ipc-fixed-rate, us

| Publication | Send | Bytes | Rate | Percentile | Luas | Aeron (Java) | aeron-go |
|---|---|---:|---:|---|---:|---:|---:|
| exclusive | offer | 16384 | 25000 | p50 | 1.86 | 1.50 | 63.6 |
| exclusive | offer | 16384 | 25000 | p90 | 2.13 | 1.72 | 67.9 |
| exclusive | offer | 16384 | 25000 | p99 | 2.41 | 1.95 | 69.4 |
| exclusive | offer | 16384 | 25000 | p99.9 | 3.46 | 2.96 | 114 |
| exclusive | offer | 16384 | 25000 | p99.99 | 12.8 | 11.8 | 401 |
| exclusive | offer | 16384 | 25000 | p99.999 | 31.1 | 28.5 | 558 |
| exclusive | try-claim | 32 | 501000 | p50 | 0.156 | 0.176 | 0.301 |
| exclusive | try-claim | 32 | 501000 | p90 | 0.202 | 0.222 | 0.357 |
| exclusive | try-claim | 32 | 501000 | p99 | 0.227 | 0.252 | 0.430 |
| exclusive | try-claim | 32 | 501000 | p99.9 | 0.349 | 0.320 | 22.0 |
| exclusive | try-claim | 32 | 501000 | p99.99 | 1.61 | 1.63 | 44.0 |
| exclusive | try-claim | 32 | 501000 | p99.999 | 2.48 | 3.76 | 50.8 |
| exclusive | try-claim | 32 | 12000000 | p50 | 0.289 | 0.273 | 13186892 ! |
| exclusive | try-claim | 32 | 12000000 | p90 | 0.540 | 0.397 | 24427626 ! |
| exclusive | try-claim | 32 | 12000000 | p99 | 0.725 | 0.581 | 27866956 ! |
| exclusive | try-claim | 32 | 12000000 | p99.9 | 4.48 | 4.29 | 28638708 ! |
| exclusive | try-claim | 32 | 12000000 | p99.99 | 9.19 | 9.38 | 28689039 ! |
| exclusive | try-claim | 32 | 12000000 | p99.999 | 18.6 | 147 | 28689039 ! |
| exclusive | try-claim | 288 | 301000 | p50 | 0.169 | 0.184 | 0.543 |
| exclusive | try-claim | 288 | 301000 | p90 | 0.219 | 0.226 | 0.588 |
| exclusive | try-claim | 288 | 301000 | p99 | 0.248 | 0.261 | 0.629 |
| exclusive | try-claim | 288 | 301000 | p99.9 | 0.438 | 0.439 | 20.3 |
| exclusive | try-claim | 288 | 301000 | p99.99 | 1.71 | 1.77 | 43.1 |
| exclusive | try-claim | 288 | 301000 | p99.999 | 2.88 | 9.58 | 51.0 |
| exclusive | try-claim | 288 | 8000000 | p50 | 0.235 | 0.223 | 12985565 ! |
| exclusive | try-claim | 288 | 8000000 | p90 | 0.294 | 0.275 | 23353885 ! |
| exclusive | try-claim | 288 | 8000000 | p99 | 0.428 | 0.375 | 26138903 ! |
| exclusive | try-claim | 288 | 8000000 | p99.9 | 3.98 | 3.56 | 26692551 ! |
| exclusive | try-claim | 288 | 8000000 | p99.99 | 6.95 | 6.40 | 26793214 ! |
| exclusive | try-claim | 288 | 8000000 | p99.999 | 15.3 | 16.6 | 26809991 ! |
| exclusive | try-claim | 1344 | 101000 | p50 | 0.240 | 0.209 | 1.08 |
| exclusive | try-claim | 1344 | 101000 | p90 | 0.307 | 0.260 | 1.13 |
| exclusive | try-claim | 1344 | 101000 | p99 | 0.451 | 0.398 | 1.24 |
| exclusive | try-claim | 1344 | 101000 | p99.9 | 0.608 | 0.563 | 23.8 |
| exclusive | try-claim | 1344 | 101000 | p99.99 | 1.82 | 1.94 | 45.2 |
| exclusive | try-claim | 1344 | 101000 | p99.999 | 3.14 | 8.62 | 57.6 |
| exclusive | try-claim | 1344 | 5000000 | p50 | 0.310 | 0.226 | 16986931 ! |
| exclusive | try-claim | 1344 | 5000000 | p90 | 0.392 | 0.339 | 30551310 ! |
| exclusive | try-claim | 1344 | 5000000 | p99 | 0.705 | 0.585 | 33772536 ! |
| exclusive | try-claim | 1344 | 5000000 | p99.9 | 6.01 | 5.24 | 34326184 ! |
| exclusive | try-claim | 1344 | 5000000 | p99.99 | 8.21 | 7.56 | 34460402 ! |
| exclusive | try-claim | 1344 | 5000000 | p99.999 | 19.3 | 21.0 | 34493956 ! |

### udp-fixed-rate, us

| Publication | Send | Bytes | Rate | Percentile | Luas | Aeron (Java) | aeron-go |
|---|---|---:|---:|---|---:|---:|---:|
| exclusive | try-claim | 32 | 501000 | p50 | 4.32 | 4.27 | 4.52 |
| exclusive | try-claim | 32 | 501000 | p90 | 5.16 | 5.13 | 5.28 |
| exclusive | try-claim | 32 | 501000 | p99 | 5.50 | 5.47 | 5.61 |
| exclusive | try-claim | 32 | 501000 | p99.9 | 6.37 | 6.23 | 18.0 |
| exclusive | try-claim | 32 | 501000 | p99.99 | 7.34 | 7.34 | 32.8 |
| exclusive | try-claim | 32 | 501000 | p99.999 | 8.54 | 11.4 | 36.7 |
| exclusive | try-claim | 288 | 301000 | p50 | 2.65 | 2.64 | 2.96 |
| exclusive | try-claim | 288 | 301000 | p90 | 2.89 | 2.90 | 3.15 |
| exclusive | try-claim | 288 | 301000 | p99 | 3.09 | 3.08 | 3.61 |
| exclusive | try-claim | 288 | 301000 | p99.9 | 4.12 | 4.09 | 23.6 |
| exclusive | try-claim | 288 | 301000 | p99.99 | 6.13 | 6.23 | 34.4 |
| exclusive | try-claim | 288 | 301000 | p99.999 | 8.54 | 12.3 | 38.3 |
| exclusive | try-claim | 1344 | 101000 | p50 | 2.96 | 2.92 | 3.70 |
| exclusive | try-claim | 1344 | 101000 | p90 | 3.21 | 3.15 | 4.00 |
| exclusive | try-claim | 1344 | 101000 | p99 | 3.49 | 3.39 | 4.26 |
| exclusive | try-claim | 1344 | 101000 | p99.9 | 4.61 | 4.61 | 22.1 |
| exclusive | try-claim | 1344 | 101000 | p99.99 | 6.25 | 6.78 | 34.7 |
| exclusive | try-claim | 1344 | 101000 | p99.999 | 10.3 | 13.4 | 39.0 |

### archive-record, Mmsg/s

| Publication | Send | Bytes | Luas | Aeron (Java) | aeron-go |
|---|---|---:|---:|---:|---:|
| concurrent | offer | 256 | 19.2 | 19.4 | 4.80 |
| concurrent | offer | 4096 | 0.975 | 0.976 | 0.828 |

### archive-replay, Mmsg/s

| Publication | Send | Bytes | Luas | Aeron (Java) | aeron-go |
|---|---|---:|---:|---:|---:|
| concurrent | offer | 256 | 48.8 | 48.2 | 48.8 |
| concurrent | offer | 4096 | 3.43 | 3.39 | 0.168 |

### cluster-ping-pong, 1 member, us

| Publication | Send | Bytes | Percentile | Luas | Aeron (Java) | aeron-go |
|---|---|---:|---|---:|---:|---:|
| exclusive | offer | 32 | p50 | 6.09 | 6.10 | 6.09 |
| exclusive | offer | 32 | p90 | 6.42 | 6.37 | 6.43 |
| exclusive | offer | 32 | p99 | 7.25 | 7.14 | 11.3 |
| exclusive | offer | 32 | p99.9 | 8.45 | 8.22 | 52.1 |
| exclusive | offer | 32 | p99.99 | 17.4 | 17.6 | 83.1 |
| exclusive | offer | 32 | p99.999 | 74.8 | 63.4 | 91.7 |
| exclusive | offer | 256 | p50 | 6.10 | 6.11 | 6.11 |
| exclusive | offer | 256 | p90 | 6.40 | 6.41 | 6.58 |
| exclusive | offer | 256 | p99 | 7.40 | 7.39 | 11.8 |
| exclusive | offer | 256 | p99.9 | 10.3 | 10.2 | 53.0 |
| exclusive | offer | 256 | p99.99 | 19.0 | 21.2 | 83.5 |
| exclusive | offer | 256 | p99.999 | 83.7 | 70.5 | 90.0 |
| exclusive | offer | 1024 | p50 | 6.30 | 6.31 | 6.43 |
| exclusive | offer | 1024 | p90 | 7.43 | 7.40 | 7.65 |
| exclusive | offer | 1024 | p99 | 8.46 | 8.42 | 13.7 |
| exclusive | offer | 1024 | p99.9 | 15.5 | 15.5 | 65.1 |
| exclusive | offer | 1024 | p99.99 | 33.0 | 32.0 | 83.8 |
| exclusive | offer | 1024 | p99.999 | 113 | 108 | 114 |

### cluster-ping-pong, 3 members, us

| Publication | Send | Bytes | Percentile | Luas | Aeron (Java) | aeron-go |
|---|---|---:|---|---:|---:|---:|
| exclusive | offer | 32 | p50 | 15.4 | 15.6 | 14.6 |
| exclusive | offer | 32 | p90 | 17.3 | 16.9 | 16.8 |
| exclusive | offer | 32 | p99 | 21.2 | 21.1 | 24.0 |
| exclusive | offer | 32 | p99.9 | 22.8 | 22.7 | 81.9 |
| exclusive | offer | 32 | p99.99 | 30.9 | 31.1 | 92.0 |
| exclusive | offer | 32 | p99.999 | 56.8 | 66.6 | 100 |
| exclusive | offer | 256 | p50 | 15.6 | 16.1 | 15.5 |
| exclusive | offer | 256 | p90 | 16.9 | 17.0 | 17.1 |
| exclusive | offer | 256 | p99 | 21.2 | 21.2 | 25.7 |
| exclusive | offer | 256 | p99.9 | 25.0 | 27.6 | 82.8 |
| exclusive | offer | 256 | p99.99 | 30.5 | 31.8 | 92.9 |
| exclusive | offer | 256 | p99.999 | 60.6 | 67.7 | 104 |
| exclusive | offer | 1024 | p50 | 14.4 | 14.8 | 15.9 |
| exclusive | offer | 1024 | p90 | 16.6 | 17.1 | 17.1 |
| exclusive | offer | 1024 | p99 | 18.3 | 20.9 | 28.0 |
| exclusive | offer | 1024 | p99.9 | 29.5 | 29.5 | 83.1 |
| exclusive | offer | 1024 | p99.99 | 49.1 | 50.0 | 93.3 |
| exclusive | offer | 1024 | p99.999 | 91.1 | 92.4 | 123 |

## Arm c9g.8xlarge

- Run: `20261111T123648Z` at commit `838dc88d69641f8508781b13d0c9fb148f3aef1a`. Runs per cell: 15 in ipc-throughput, udp-throughput; 5 in ipc-ping-pong, udp-ping-pong, ipc-fixed-rate, udp-fixed-rate, archive-record, archive-replay, cluster-ping-pong, interleaved.
- Kernel: 7.0.0-1014-aws
- CPU: unknown
- Governor: unknown
- SMT: notsupported
- Isolated CPUs: 1-17
- Driver: aeronmd 1.53.3, commit 377943bcb8+guilty
- Luas: v0.0.0-20261110170533-5d8f75734b75, commit 5d8f75734b75
- Aeron (Java): 1.53.3, commit 377943bcb8
- aeron-go: v0.0.0-20240606170339-8b05ad14e456, commit 8b05ad14e456
- Go: go1.26.4, GOMAXPROCS 17, GOGC unset, GOMEMLIMIT unset, GODEBUG unset
- Luas layout: hot goroutines unlocked, yield every 5 ms, GOMAXPROCS 2, in cluster-ping-pong, ipc-fixed-rate, ipc-ping-pong, udp-fixed-rate, udp-ping-pong
- JDK: 26.0.2.1+1 OpenJDK 64-Bit Server VM (Eclipse Adoptium), flags: -XX:+UnlockExperimentalVMOptions -XX:+TrustFinalNonStaticFields -XX:+UseParallelGC -Dagrona.disable.bounds.checks=true -Daeron.client.idle.sleep.duration=1ms -Daeron.pre.touch.mapped.memory=true --add-opens java.base/jdk.internal.misc=ALL-UNNAMED

Scenarios: [ipc-throughput](#ipc-throughput-mmsgs-1) | [udp-throughput](#udp-throughput-mmsgs-1) | [ipc-ping-pong](#ipc-ping-pong-us-1) | [udp-ping-pong](#udp-ping-pong-us-1) | [ipc-fixed-rate](#ipc-fixed-rate-us-1) | [udp-fixed-rate](#udp-fixed-rate-us-1) | [archive-record](#archive-record-mmsgs-1) | [archive-replay](#archive-replay-mmsgs-1) | [cluster-ping-pong, 1 member](#cluster-ping-pong-1-member-us-1) | [cluster-ping-pong, 3 members](#cluster-ping-pong-3-members-us-1)

### ipc-throughput, Mmsg/s

| Publication | Send | Bytes | Luas | Aeron (Java) | aeron-go |
|---|---|---:|---:|---:|---:|
| concurrent | offer | 32 | 8.46 | 8.58 | 6.49 |
| concurrent | offer | 256 | 6.08 | 8.36 | 4.80 |
| concurrent | offer | 1024 | 4.81 | 5.40 | 1.80 |
| concurrent | offer | 4096 | 1.35 | 1.72 | 0.0779 |
| concurrent | offer | 16384 | 0.364 | 0.444 | 0.0197 |
| concurrent | offer | 65536 | 0.0962 | 0.114 | 0.00492 |
| concurrent | try-claim | 32 | 8.53 | 7.47 | 6.48 |
| concurrent | try-claim | 256 | 6.06 | 8.18 | 4.60 |
| concurrent | try-claim | 1024 | 4.74 | 5.43 | 1.79 |
| exclusive | offer | 32 | 22.7 | 11.7 | 6.56 |
| exclusive | offer | 256 | 7.27 | 9.23 | 4.74 |
| exclusive | offer | 1024 | 5.31 | 6.66 | 1.81 |
| exclusive | offer | 4096 | 1.48 | 1.80 | 0.0785 |
| exclusive | offer | 16384 | 0.381 | 0.440 | 0.0196 |
| exclusive | offer | 65536 | 0.0948 | 0.106 | 0.00492 |
| exclusive | try-claim | 32 | 16.6 | 12.3 | 6.44 |
| exclusive | try-claim | 256 | 7.24 | 8.48 | 4.69 |
| exclusive | try-claim | 1024 | 5.13 | 6.57 | 1.80 |

### udp-throughput, Mmsg/s

| Publication | Send | Bytes | Luas | Aeron (Java) | aeron-go |
|---|---|---:|---:|---:|---:|
| concurrent | offer | 32 | 14.8 | 14.8 | 7.46 |
| concurrent | offer | 256 | 2.77 | 2.75 | 2.75 |
| concurrent | offer | 1024 | 0.699 | 0.701 | 0.703 |
| concurrent | offer | 4096 | 0.225 | 0.226 | 0.0746 |
| concurrent | offer | 16384 | 0.0577 | 0.0575 | 0.0197 |
| concurrent | offer | 65536 | 0.0145 | 0.0145 | 0.00492 |
| concurrent | try-claim | 32 | 14.9 | 14.8 | 7.29 |
| concurrent | try-claim | 256 | 2.75 | 2.75 | 2.76 |
| concurrent | try-claim | 1024 | 0.708 | 0.706 | 0.706 |
| exclusive | offer | 32 | 15.0 | 15.0 | 7.47 |
| exclusive | offer | 256 | 2.77 | 2.77 | 2.77 |
| exclusive | offer | 1024 | 0.704 | 0.699 | 0.706 |
| exclusive | offer | 4096 | 0.228 | 0.229 | 0.0793 |
| exclusive | offer | 16384 | 0.0576 | 0.0575 | 0.0199 |
| exclusive | offer | 65536 | 0.0145 | 0.0145 | 0.00494 |
| exclusive | try-claim | 32 | 14.7 | 14.9 | 7.32 |
| exclusive | try-claim | 256 | 2.68 | 2.70 | 2.70 |
| exclusive | try-claim | 1024 | 0.705 | 0.705 | 0.702 |

### ipc-ping-pong, us

| Publication | Send | Bytes | Percentile | Luas | Aeron (Java) | aeron-go |
|---|---|---:|---|---:|---:|---:|
| concurrent | offer | 32 | p50 | 0.286 | 0.269 | 0.476 |
| concurrent | offer | 32 | p90 | 0.317 | 0.306 | 0.526 |
| concurrent | offer | 32 | p99 | 0.359 | 0.347 | 1.46 |
| concurrent | offer | 32 | p99.9 | 0.440 | 0.407 | 2.49 |
| concurrent | offer | 32 | p99.99 | 1.19 | 0.689 | 10.4 |
| concurrent | offer | 32 | p99.999 | 17.6 | 17.2 | 19.3 |
| concurrent | offer | 256 | p50 | 0.478 | 0.418 | 0.710 |
| concurrent | offer | 256 | p90 | 0.510 | 0.492 | 0.752 |
| concurrent | offer | 256 | p99 | 0.552 | 0.559 | 1.73 |
| concurrent | offer | 256 | p99.9 | 0.651 | 1.01 | 3.38 |
| concurrent | offer | 256 | p99.99 | 9.55 | 10.1 | 13.9 |
| concurrent | offer | 256 | p99.999 | 18.1 | 18.9 | 19.8 |
| concurrent | offer | 1024 | p50 | 0.520 | 0.546 | 1.36 |
| concurrent | offer | 1024 | p90 | 0.603 | 0.616 | 1.41 |
| concurrent | offer | 1024 | p99 | 0.695 | 0.701 | 2.34 |
| concurrent | offer | 1024 | p99.9 | 0.820 | 0.815 | 3.88 |
| concurrent | offer | 1024 | p99.99 | 12.6 | 12.0 | 17.6 |
| concurrent | offer | 1024 | p99.999 | 19.6 | 19.7 | 22.0 |

### udp-ping-pong, us

| Publication | Send | Bytes | Percentile | Luas | Aeron (Java) | aeron-go |
|---|---|---:|---|---:|---:|---:|
| concurrent | offer | 32 | p50 | 4.38 | 4.31 | 4.46 |
| concurrent | offer | 32 | p90 | 4.67 | 4.62 | 4.79 |
| concurrent | offer | 32 | p99 | 4.95 | 4.93 | 5.33 |
| concurrent | offer | 32 | p99.9 | 13.1 | 12.7 | 13.2 |
| concurrent | offer | 32 | p99.99 | 23.3 | 22.8 | 23.2 |
| concurrent | offer | 32 | p99.999 | 27.3 | 27.4 | 27.1 |
| concurrent | offer | 256 | p50 | 4.62 | 4.58 | 4.91 |
| concurrent | offer | 256 | p90 | 4.99 | 4.92 | 5.19 |
| concurrent | offer | 256 | p99 | 5.28 | 5.21 | 5.62 |
| concurrent | offer | 256 | p99.9 | 12.8 | 13.1 | 13.8 |
| concurrent | offer | 256 | p99.99 | 22.8 | 22.6 | 23.0 |
| concurrent | offer | 256 | p99.999 | 26.2 | 26.5 | 26.7 |
| concurrent | offer | 1024 | p50 | 5.17 | 5.20 | 5.96 |
| concurrent | offer | 1024 | p90 | 5.47 | 5.51 | 6.32 |
| concurrent | offer | 1024 | p99 | 5.79 | 5.82 | 6.63 |
| concurrent | offer | 1024 | p99.9 | 17.2 | 16.6 | 17.9 |
| concurrent | offer | 1024 | p99.99 | 23.7 | 23.5 | 24.1 |
| concurrent | offer | 1024 | p99.999 | 26.9 | 27.6 | 27.5 |

### ipc-fixed-rate, us

| Publication | Send | Bytes | Rate | Percentile | Luas | Aeron (Java) | aeron-go |
|---|---|---:|---:|---|---:|---:|---:|
| exclusive | offer | 16384 | 25000 | p50 | 5.29 | 4.50 | 10544480 ! |
| exclusive | offer | 16384 | 25000 | p90 | 5.69 | 4.93 | 18958254 ! |
| exclusive | offer | 16384 | 25000 | p99 | 7.11 | 5.81 | 28454158 ! |
| exclusive | offer | 16384 | 25000 | p99.9 | 17.2 | 17.5 | 30198989 ! |
| exclusive | offer | 16384 | 25000 | p99.99 | 21.1 | 20.8 | 30383538 ! |
| exclusive | offer | 16384 | 25000 | p99.999 | 34.1 | 32.1 | 30400315 ! |
| exclusive | try-claim | 32 | 501000 | p50 | 0.278 | 0.329 | 0.539 |
| exclusive | try-claim | 32 | 501000 | p90 | 0.355 | 0.404 | 0.603 |
| exclusive | try-claim | 32 | 501000 | p99 | 0.413 | 0.474 | 0.648 |
| exclusive | try-claim | 32 | 501000 | p99.9 | 10.7 | 11.0 | 11.6 |
| exclusive | try-claim | 32 | 501000 | p99.99 | 17.1 | 17.7 | 17.9 |
| exclusive | try-claim | 32 | 501000 | p99.999 | 21.0 | 22.4 | 21.8 |
| exclusive | try-claim | 288 | 301000 | p50 | 0.361 | 0.391 | 0.833 |
| exclusive | try-claim | 288 | 301000 | p90 | 0.420 | 0.465 | 0.885 |
| exclusive | try-claim | 288 | 301000 | p99 | 0.545 | 0.567 | 0.932 |
| exclusive | try-claim | 288 | 301000 | p99.9 | 10.9 | 10.6 | 12.0 |
| exclusive | try-claim | 288 | 301000 | p99.99 | 17.2 | 17.1 | 18.4 |
| exclusive | try-claim | 288 | 301000 | p99.999 | 21.0 | 21.2 | 22.7 |
| exclusive | try-claim | 1344 | 101000 | p50 | 0.565 | 0.596 | 1.71 |
| exclusive | try-claim | 1344 | 101000 | p90 | 0.667 | 0.688 | 1.76 |
| exclusive | try-claim | 1344 | 101000 | p99 | 0.797 | 0.808 | 1.86 |
| exclusive | try-claim | 1344 | 101000 | p99.9 | 11.0 | 9.82 | 13.2 |
| exclusive | try-claim | 1344 | 101000 | p99.99 | 17.0 | 15.4 | 18.7 |
| exclusive | try-claim | 1344 | 101000 | p99.999 | 21.9 | 21.1 | 24.9 |

### udp-fixed-rate, us

| Publication | Send | Bytes | Rate | Percentile | Luas | Aeron (Java) | aeron-go |
|---|---|---:|---:|---|---:|---:|---:|
| exclusive | try-claim | 32 | 501000 | p50 | 6.63 | 6.66 | 6.92 |
| exclusive | try-claim | 32 | 501000 | p90 | 8.75 | 8.80 | 9.10 |
| exclusive | try-claim | 32 | 501000 | p99 | 10.3 | 10.7 | 11.4 |
| exclusive | try-claim | 32 | 501000 | p99.9 | 17.1 | 17.6 | 18.6 |
| exclusive | try-claim | 32 | 501000 | p99.99 | 22.2 | 22.3 | 23.4 |
| exclusive | try-claim | 32 | 501000 | p99.999 | 30.1 | 31.2 | 32.3 |
| exclusive | try-claim | 288 | 301000 | p50 | 5.83 | 5.83 | 6.23 |
| exclusive | try-claim | 288 | 301000 | p90 | 7.39 | 7.38 | 7.64 |
| exclusive | try-claim | 288 | 301000 | p99 | 7.83 | 7.86 | 8.12 |
| exclusive | try-claim | 288 | 301000 | p99.9 | 17.9 | 17.7 | 18.2 |
| exclusive | try-claim | 288 | 301000 | p99.99 | 23.5 | 24.3 | 25.9 |
| exclusive | try-claim | 288 | 301000 | p99.999 | 32.2 | 32.3 | 34.8 |
| exclusive | try-claim | 1344 | 101000 | p50 | 5.28 | 5.25 | 6.35 |
| exclusive | try-claim | 1344 | 101000 | p90 | 5.63 | 5.62 | 6.71 |
| exclusive | try-claim | 1344 | 101000 | p99 | 6.05 | 6.04 | 7.21 |
| exclusive | try-claim | 1344 | 101000 | p99.9 | 12.3 | 12.4 | 14.9 |
| exclusive | try-claim | 1344 | 101000 | p99.99 | 15.7 | 15.7 | 19.0 |
| exclusive | try-claim | 1344 | 101000 | p99.999 | 27.8 | 27.9 | 28.5 |

### archive-record, Mmsg/s

| Publication | Send | Bytes | Luas | Aeron (Java) | aeron-go |
|---|---|---:|---:|---:|---:|
| concurrent | offer | 256 | 9.73 | 9.81 | 4.65 |
| concurrent | offer | 4096 | 0.750 | 0.744 | 0.508 |

### archive-replay, Mmsg/s

| Publication | Send | Bytes | Luas | Aeron (Java) | aeron-go |
|---|---|---:|---:|---:|---:|
| concurrent | offer | 256 | 38.3 | 38.5 | 38.6 |
| concurrent | offer | 4096 | 2.84 | 2.85 | 0.0741 |

### cluster-ping-pong, 1 member, us

| Publication | Send | Bytes | Percentile | Luas | Aeron (Java) | aeron-go |
|---|---|---:|---|---:|---:|---:|
| exclusive | offer | 32 | p50 | 6.96 | 7.01 | 7.01 |
| exclusive | offer | 32 | p90 | 7.25 | 7.28 | 7.31 |
| exclusive | offer | 32 | p99 | 7.51 | 7.52 | 8.70 |
| exclusive | offer | 32 | p99.9 | 19.5 | 18.9 | 31.9 |
| exclusive | offer | 32 | p99.99 | 25.5 | 25.4 | 40.0 |
| exclusive | offer | 32 | p99.999 | 53.2 | 51.6 | 57.8 |
| exclusive | offer | 256 | p50 | 7.04 | 7.08 | 7.09 |
| exclusive | offer | 256 | p90 | 7.36 | 7.39 | 7.43 |
| exclusive | offer | 256 | p99 | 7.70 | 7.71 | 9.12 |
| exclusive | offer | 256 | p99.9 | 19.6 | 19.3 | 32.1 |
| exclusive | offer | 256 | p99.99 | 26.9 | 26.7 | 42.3 |
| exclusive | offer | 256 | p99.999 | 55.2 | 53.7 | 60.4 |
| exclusive | offer | 1024 | p50 | 7.44 | 7.50 | 7.57 |
| exclusive | offer | 1024 | p90 | 7.82 | 7.85 | 9.18 |
| exclusive | offer | 1024 | p99 | 11.0 | 11.0 | 11.6 |
| exclusive | offer | 1024 | p99.9 | 24.1 | 24.1 | 34.0 |
| exclusive | offer | 1024 | p99.99 | 46.7 | 46.8 | 48.8 |
| exclusive | offer | 1024 | p99.999 | 62.7 | 65.7 | 75.5 |

### cluster-ping-pong, 3 members, us

| Publication | Send | Bytes | Percentile | Luas | Aeron (Java) | aeron-go |
|---|---|---:|---|---:|---:|---:|
| exclusive | offer | 32 | p50 | 17.7 | 17.8 | 17.8 |
| exclusive | offer | 32 | p90 | 19.0 | 19.2 | 19.1 |
| exclusive | offer | 32 | p99 | 19.5 | 20.0 | 22.2 |
| exclusive | offer | 32 | p99.9 | 33.7 | 35.3 | 43.0 |
| exclusive | offer | 32 | p99.99 | 38.4 | 40.0 | 51.3 |
| exclusive | offer | 32 | p99.999 | 50.9 | 53.8 | 65.6 |
| exclusive | offer | 256 | p50 | 17.8 | 17.9 | 17.9 |
| exclusive | offer | 256 | p90 | 19.2 | 19.2 | 19.2 |
| exclusive | offer | 256 | p99 | 19.7 | 19.8 | 23.1 |
| exclusive | offer | 256 | p99.9 | 36.9 | 36.4 | 43.5 |
| exclusive | offer | 256 | p99.99 | 42.0 | 42.1 | 52.4 |
| exclusive | offer | 256 | p99.999 | 52.9 | 52.3 | 73.3 |
| exclusive | offer | 1024 | p50 | 18.4 | 18.5 | 18.5 |
| exclusive | offer | 1024 | p90 | 19.6 | 19.7 | 19.7 |
| exclusive | offer | 1024 | p99 | 20.8 | 20.7 | 27.7 |
| exclusive | offer | 1024 | p99.9 | 37.2 | 37.0 | 45.7 |
| exclusive | offer | 1024 | p99.99 | 49.9 | 50.1 | 56.4 |
| exclusive | offer | 1024 | p99.999 | 70.6 | 72.1 | 88.4 |
