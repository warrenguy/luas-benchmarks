# luas-benchmarks

Benchmarks that compare Aeron clients:

- [Luas](https://github.com/warrenguy/luas) (Go)
- [Aeron](https://github.com/aeron-io/aeron) (Java)
- [aeron-go](https://github.com/lirm/aeron-go) (Go)

Every client runs the same scenarios against one C media driver. The
driver, config, channels, stream ids, message sizes, counts, warmup, idle
strategy, fragment limit, and core layout match for every client.
[Differences between clients](#differences-between-clients) names each
difference that remains.

[RESULTS.md](RESULTS.md) compares the clients on the latest EC2 run.

## Scenarios

| Scenario | Measures |
| --- | --- |
| `ipc-throughput`, `udp-throughput` | Message rate from one publisher to one subscriber. |
| `ipc-ping-pong`, `udp-ping-pong` | Round-trip latency, one message in flight. |
| `ipc-fixed-rate`, `udp-fixed-rate` | Round-trip latency at a fixed send rate. |
| `archive-record` | Time for the archive to record one IPC publication. |
| `archive-replay` | Time to replay a finished recording to one subscriber. |
| `cluster-ping-pong` | Round-trip latency through a Java echo cluster of 1 or 3 members. |

The archive and cluster servers are Java for every client.

## Usage

    make driver    # build the C media driver, needs Docker
    make build     # build every client
    make smoke     # check delivery and output shape with small counts
    make matrix    # run every scenario of every client
    go run ./report results/<run-id>    # print the report of one run

`make help` lists every target. `make matrix` pins hot threads to five
isolated CPUs. On a host with fewer, set `DRIVER_CPUS` and `CLIENT_CPUS`.
The `scripts/run-matrix` header lists its environment variables.

### EC2

`scripts/ec2-matrix` launches one amd64 and one arm64 spot instance. Each
runs `make matrix` on isolated CPUs. The script downloads the results into
`results/ec2/`. The script uploads `git archive HEAD`. Commit the tree
first.

    scripts/ec2-matrix setup           # once per account
    scripts/ec2-matrix                 # about 6.5 hours
    ARCHES=amd64 scripts/ec2-matrix    # one instance type only

The script header lists its requirements and environment variables.

Write `RESULTS.md` from the downloaded run:

    go run ./report -comparison results/ec2/<run-id> > RESULTS.md

## Versions

- `go.mod` pins Luas, aeron-go, HdrHistogram, and Go.
- The Luas commit's `specs/versions.yaml` pins Aeron.
- `pom.xml` pins the Java libraries and the JDK release.

## Method

- The runner interleaves the clients: one run of each, then the next round.
- Each run starts a fresh client driver. The driver touches every term page
  when it creates a log.
- The drivers take the 2 MiB socket buffers and receiver window of Aeron's
  `low-latency.properties`.
- Every process starts on the housekeeping CPUs. Each hot thread then moves
  to a CPU of its own.
- `TryClaim` takes at most 1376 bytes at the default MTU. Throughput does not
  have a `TryClaim` cell at 4096 bytes.
- Each archive run starts a fresh Java archive on the same driver. The
  archive records to /dev/shm.
- Each cluster run starts a fresh Java echo cluster. Each member runs on its
  own C driver. Clients reach the members over UDP loopback. The clients
  send through `Offer`.
- Ping-pong runs ping and pong as two clients in one process. Closed-loop
  latency understates the tail under load. The fixed-rate scenarios measure
  latency under load.
- The fixed-rate scenarios port the send loop of the Aeron load test rig
  (`LoadTestRig` in [aeron-io/benchmarks](https://github.com/aeron-io/benchmarks)).
  Each message carries its scheduled send time, so the histogram does not
  hide coordinated omission.
- Fixed-rate warmup compared with the scripts in aeron-io/benchmarks,
  such as [`scripts/aeron/remote-echo-single-host-benchmarks`](https://github.com/aeron-io/benchmarks/blob/master/scripts/aeron/remote-echo-single-host-benchmarks):
  - Lengths: both warm up for 30 s, then send for 60 s.
  - Warmup rate: aeron-io/benchmarks warms up at 25K msg/s. These
    benchmarks warm up at the cell rate.
  - Reason: the Java JIT compiles the hot path for the warmup rate. A
    12M msg/s cell after a 25K msg/s warmup makes Java recompile during
    the measurement. The recompile stalls Java for over 80 ms.
- Ping and pong run in one process on one driver.
  `remote-echo-single-host-benchmarks` runs `LoadTestRig` and `EchoNode`
  as two processes on one driver.
- Each latency cell merges the histograms of its runs. A `!` marks a
  fixed-rate cell where a run missed its send rate or lost replies.
- `host.json` holds the full config of the driver, the archive, and the
  cluster members.

## Differences between clients

- Luas `Publication.Offer` takes `[]byte`. aeron-go `Publication.Offer`
  takes an `*atomic.Buffer`, an offset, and a length.
- aeron-go `AddExclusivePublication` returns the same `*Publication` type as
  `AddPublication`.
- In `ipc-throughput`, and in `udp-throughput` at 4096 bytes and above,
  aeron-go sends fewer messages per run than the other clients. It runs
  far slower, so the same count would make each run many times longer.
  Each client still reports its rate in messages per second.
  `scripts/run-matrix` holds the counts.
- Luas does not start a conductor thread. Each hot goroutine calls
  `Client.DoWork` once every 1024 polls, offers, or wait spins.
- Java runs with Agrona bounds checks off, as the Aeron benchmarks do. The
  Go clients do not have a switch for their bounds checks.
- Luas and Aeron (Java) pre-touch every page of a log buffer when they map it.
  aeron-go takes a minor fault at its first write to each page.
- Luas unlocks its hot goroutines from their pinned threads in the
  ping-pong, cluster, and fixed-rate scenarios. `LUAS_UNLOCKED=0` keeps
  them locked. aeron-go keeps its hot goroutines locked. The report header
  names the Luas layout.

Why Luas unlocks:

- The Go scheduler preempts a goroutine after 10 ms on a CPU.
- A preempted goroutine that is locked to a thread puts that thread to
  sleep. Another thread must wake it (golang/go#21827).
- An unlocked goroutine does not wait for that wake.
- The scheduler can preempt two unlocked goroutines at once. One of them
  then waits about 20 ms.
- Each unlocked goroutine yields every 5 ms. The goroutine therefore
  never reaches the 10 ms preemption.
- The scheduler can move an unlocked goroutine off its CPU. In the
  ping-pong and fixed-rate scenarios, each unlocked goroutine checks its
  CPU every 5 ms. A goroutine off its CPU moves back.
  `cluster-ping-pong` does not check.

## Contributing

Contribution policy: [CONTRIBUTING.md](CONTRIBUTING.md).

## License

MIT. See [LICENSE](LICENSE).
