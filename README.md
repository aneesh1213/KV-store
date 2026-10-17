# Distributed KV Store

A sharded, durable, single-node key-value store written in Go.

Built from scratch to understand how real databases work under the hood — the primitives that Redis, LevelDB, and SQLite are made of.

## What it does

- **Sharded** — 32 partitions, routed by FNV-1a hash of the key.
- **Concurrent-safe** — `sync.RWMutex` per shard, race-tested.
- **Durable** — write-ahead log (WAL) with `fsync` on every write.
- **Fast restart** — atomic snapshots and log compaction.
- **HTTP API** — `PUT`, `GET`, and `DELETE` over `/kv/{key}`.
- **Graceful shutdown** — drains in-flight requests and flushes a final snapshot.

## Architecture

```text
HTTP client
    |
    v
+--------------------------------+
| api.Server (net/http)           |
| PUT    /kv/{key}                |
| GET    /kv/{key}                |
| DELETE /kv/{key}                |
| GET    /health                  |
| Middleware: request logging     |
+---------------+----------------+
                |
                v
+--------------------------------+
| store.Store                     |
|                                |
|  +--------------------------+  |
|  | 32 shards                |  |
|  | map + RWMutex per shard  |  |
|  +--------------------------+  |
|                                |
| writesMu (RWMutex)              |
| Set/Delete -> RLock             |
| Snapshot -> Lock                |
|                                |
| writesSinceSnapshot (atomic)    |
| snapshotThreshold = 10,000      |
+---------------+----------------+
                |
        +-------+-------+
        |               |
        v               v
+---------------+ +----------------+
| WAL           | | Snapshot       |
| append + fsync| | atomic write   |
| JSON lines    | | tmp -> fsync   |
|               | | -> rename      |
+-------+-------+ +--------+-------+
        |                  |
        +--------+---------+
                 |
                 v
+--------------------------------+
| data/                          |
|   kvstore.wal                  |
|   kvstore.snapshot             |
+--------------------------------+
```

**Write path:** `Set` -> `writesMu.RLock` -> WAL append + `fsync` -> shard write -> counter increment -> maybe snapshot.

**Read path:** `Get` -> shard `RLock` -> return value. Reads do not touch the WAL or `writesMu`.

**Recovery:** load snapshot -> replay WAL on top -> restore the latest recoverable state.

## Quick start

Requires **Go 1.22+** for `net/http.ServeMux` method-and-path patterns.

```bash
git clone https://github.com/aneesh1213/kvstore
cd kvstore
mkdir -p data
go build ./...
go run ./backend/cmd/kvstore
```

The server listens on `:8080`.

### Try the API

Store a value. The request body is the raw value, with no JSON wrapper:

```bash
curl -X PUT http://localhost:8080/kv/user:1 --data-binary "alice"
```

Read it back:

```bash
curl http://localhost:8080/kv/user:1
# alice
```

Request a missing key:

```bash
curl -i http://localhost:8080/kv/nobody
# HTTP/1.1 404 Not Found
```

Delete a key:

```bash
curl -i -X DELETE http://localhost:8080/kv/user:1
# HTTP/1.1 204 No Content
```

Check server health:

```bash
curl http://localhost:8080/health
# ok
```

**Windows PowerShell:** use `curl.exe` rather than `curl`, because PowerShell aliases `curl` to `Invoke-WebRequest`, which uses different flags. If PowerShell interprets special characters in an argument, prefix the command with `--%`.

Stop the server with `Ctrl+C`. The server logs its shutdown, writes a final snapshot, and truncates the WAL. Restart it and request the same key to verify that the data persists.

## Benchmarks

Run from the module root:

```bash
go test -bench=. -benchmem -cpu=1,2,4,8 ./backend/internals/store
```

In PowerShell:

```powershell
go test --% -bench=. -benchmem -cpu=1,2,4,8 ./backend/internals/store
```

### Unsharded: single mutex

| Goroutines | ns/op |
|---:|---:|
| 1 | 101.3 |
| 2 | 102.5 |
| 4 | 152.6 |
| 8 | 164.0 |

With more goroutines, per-operation time rises. This is lock contention: concurrent operations queue behind a single mutex.

### Sharded: 32 partitions

| Goroutines | ns/op |
|---:|---:|
| 1 | 104.1 |
| 2 | 77.45 |
| 4 | 64.62 |
| 8 | 72.99 |

In this benchmark, sharding improves concurrent performance by letting operations on different shards proceed independently. Results depend on the workload, machine, and benchmark setup.

### Ablation: force every key into one shard

To check whether the speedup came from key distribution rather than code structure, every key was routed to shard `0` and the benchmark was rerun.

| Goroutines | ns/op |
|---:|---:|
| 1 | 353.5 |
| 4 | 314.6 |
| 8 | 458.4 |

Performance deteriorated when all operations contended on one shard. This supports the conclusion that distributing keys across shards is an important part of the observed improvement.

> **Note:** These are the benchmark figures from the current project notes. Rerun the benchmark on your target machine before treating them as representative or comparing them with other implementations.

### HTTP latency observations

| Operation | Approx. latency | Main bottleneck |
|---|---:|---|
| `PUT` | ~1 ms | WAL `fsync` |
| `GET` | ~0 µs | In-memory read |
| `DELETE` | ~1 ms | WAL `fsync` |

The `fsync` on each write is the durability cost. Group commit or batched `fsync` could improve throughput, but would change the durability/latency trade-off.

## Design decisions

### Why 32 shards?

The store uses 32 shards to distribute lock contention without creating an excessive number of maps and mutexes. Each shard owns its map and lock. Thirty-two is a starting point, not a universal optimum; the shard count can be tuned with `NewWithShards(path, n)`.

### Why FNV-1a?

FNV-1a provides deterministic routing for a given key, so the same key maps to the same shard across process restarts. Go's built-in map hash is an implementation detail and is randomized, so it should not be used as a stable partitioning function.

FNV-1a is chosen for simplicity and determinism, not cryptographic security. If keys can be attacker-controlled, consider whether predictable hash-based routing could be abused to create hot shards.

### Why JSON lines instead of a binary format?

The WAL uses newline-delimited JSON records. They are easy to inspect, debug, and reproduce with ordinary command-line tools. For a learning project, observability and simplicity are useful; a binary format could be considered if profiling shows encoding or storage size is a bottleneck.

### Why `fsync` on every write?

A successful write should mean the WAL record has been flushed to durable storage as far as the operating system and storage device honor the sync request. Without syncing, recently acknowledged writes can be lost after a crash or power failure.

Syncing every write adds latency. Batching writes can improve throughput, but it changes the failure window and acknowledgement semantics. That trade-off should be explicit.

### Why atomic rename for snapshots?

The snapshot process writes a temporary file, syncs it, renames it into place, and syncs the parent directory where supported. This avoids exposing a partially written snapshot as the current snapshot.

The exact durability guarantees depend on filesystem and operating-system behavior. Directory syncing is not portable in the same way on every platform, including Windows, so the implementation treats it as best-effort where necessary.

### Why use `writesMu` as an `RWMutex`?

- `Set` and `Delete` take `writesMu.RLock`, allowing multiple writes to proceed concurrently through the coordination lock.
- `snapshotNow` takes `writesMu.Lock`, excluding writes while the snapshot captures a consistent state.
- `Get` does not take `writesMu`, so reads do not wait for snapshot coordination.

Using an exclusive `writesMu.Lock` for every `Set` would serialize writes globally and undermine the concurrency benefit of sharding. The shard locks still protect each shard's map, while `writesMu` coordinates writes with snapshots.

### Why load the snapshot before replaying the WAL?

The snapshot represents a base state. The WAL contains subsequent operations that must be replayed over that base.

Recovery therefore loads the snapshot first and replays the WAL afterward. Reversing the order could allow older snapshot values to overwrite newer WAL updates.

## Bugs that taught me the most

### A false-positive snapshot test

`TestSnapshotAndRecover` was passing, but recovery was succeeding because WAL replay restored the data. The snapshot path had not actually been exercised: `Snapshot()` was returning early because the write counter was below the threshold.

The fix was to assert that the snapshot file exists and that the WAL has been truncated as expected. A test that passes for the wrong reason can hide a broken feature.

### Windows, `O_APPEND`, and truncation

On Windows, truncating a file opened with `O_APPEND` caused an access-denied failure in this implementation. The fix was to open the file with `O_RDWR` and seek to the end manually before each write.

This was a useful reminder that filesystem behavior and file-opening flags can differ across operating systems.

### `RLock` versus `Lock` in `writesMu`

Using `writesMu.Lock()` in `Set` would have serialized every write globally. That would defeat the intended parallelism across shards and make write performance largely dependent on the WAL sync path.

Using `RLock` for ordinary writes and `Lock` for snapshot coordination preserves the intended concurrency model.

More implementation notes are in [`NOTES.md`](NOTES.md).

## Testing

Run the store tests:

```bash
go test ./backend/internals/store
```

Repeat them to help expose intermittent failures:

```bash
go test -count=10 ./backend/internals/store
```

Run the race detector:

```bash
go test -race ./backend/internals/store
```

The race detector may require a working C compiler/toolchain for your platform.

Run benchmarks:

```bash
go test -bench=. -benchmem ./backend/internals/store
```

### Coverage includes

- Concurrent writes from 100 goroutines.
- WAL append, replay, partial-record recovery, and malformed-line rejection.
- Snapshot round-trip, missing-file behavior, and atomic replacement.
- End-to-end recovery using both a snapshot and the WAL.

## Project structure

```text
.
├── backend/
│   ├── cmd/
│   │   └── kvstore/
│   │       └── main.go          # Entry point, HTTP server, graceful shutdown
│   └── internals/
│       ├── api/
│       │   ├── server.go        # Handlers, router, mux
│       │   └── middleware.go    # Request logging
│       └── store/
│           ├── store.go        # Store type, sharding, snapshot coordination
│           ├── wal.go          # WAL append, replay, truncation
│           ├── snapshot.go     # Atomic snapshot save/load
│           ├── record.go       # JSON record encoding
│           └── *_test.go       # Tests
├── data/                        # Runtime state (gitignored)
│   ├── kvstore.wal
│   └── kvstore.snapshot
├── NOTES.md                     # Development notes
├── CHEATSHEET.md                # Commands reference
└── README.md
```

## What's next?

- **Replication:** ship WAL records from a leader to a follower over TCP.
- **Raft consensus:** build leader election and consensus on top of replication.
- **LLM inference server:** a separate project with a Go control plane and Python worker.

## Further reading

- *Understanding Distributed Systems* — Roberto Vitillo
- *Designing Data-Intensive Applications* — Martin Kleppmann
- [The Twelve-Factor App](https://12factor.net/)
- [TinyKV course](https://github.com/talent-plan/tinykv)

## License

MIT — see [`LICENSE`](LICENSE) 
