# KV Store — Project Devlog

A sharded, durable, single-node key-value store written in Go.
Built from scratch over six stages, each solving a specific problem.

## Why I built this

I wanted to understand how real databases work — not the API, the internals.
Redis, LevelDB, SQLite all look like magic from the outside. After this
project, they don't anymore.

## The six stages

### Stage 1 — Basic in-memory KV
`Set / Get / Delete / Len` over a single `map[string]string`.
The point: get the shape right before adding complexity.

### Stage 2 — Concurrency
Bombarded the store with 100 goroutines and watched it crash with
`fatal error: concurrent map writes`. Added `sync.RWMutex`. Learned:
race = correctness bug, not performance. Fixed correctness first.

### Stage 3 — Sharding
Benchmark: 1 → 4 goroutines went from 101 ns/op to 152 ns/op.
More workers, worse throughput. That's contention.
Split into 32 shards, hashed by FNV-1a. Reran: 164 → 65 ns/op at 4 cores.
Then broke it on purpose (forced all keys to shard 0) and watched it drop
to 458 ns/op — proving the shard distribution was the cause, not the code shape.


## 2026-09-29 — Stage 4 complete: durable store

Built WAL with append/replay/truncate, wired into Store.
Proof: WRITE mode → Ctrl+C → READ mode. Data survived.

Noticed: WAL grows on every run because main.go re-Sets the same keys.
Replay is correct (last write wins) but the log bloats. That's what
snapshots will fix next.

Learned:
- WAL write must precede memory update or crash loses the change.
- Replay must NOT call Set (infinite loop).
- Windows can't truncate O_APPEND files. Use O_RDWR + seek.
- Replay's return offset is a truncate point, not a write position.


## 2026-10-01 — Stage 6 complete: HTTP API + logging

**Built:**
- `backend/internals/api/server.go` — Server struct wrapping a Store,
  with a mux routing PUT/GET/DELETE /kv/{key} and GET /health.
- Handlers as methods on *Server, using Go 1.22+ path patterns
  (`PUT /kv/{key}`) and r.PathValue("key").
- Graceful shutdown: signal.Notify catches SIGINT/SIGTERM, then
  srv.Shutdown(ctx) with a 5s deadline lets in-flight requests finish
  before the process exits.
- `backend/internals/api/middleware.go` — withLogging wraps the mux
  so every request logs `method path status latency`.
- statusRecorder wraps http.ResponseWriter to capture the status
  (defaulting to 200 when the handler never calls WriteHeader).

**Broke:**
- `curl -X PUT` in PowerShell fails — `curl` is an alias for
  Invoke-WebRequest, which has different flags. Fix: `curl.exe`.
- Postman imported `-d "alice"` as form data and re-serialized it,
  storing the wrong bytes. Fix: Body → raw → Text.

**Learned:**
- Router = method+path → handler lookup. Go's ServeMux does it
  natively since 1.22, no third-party library needed.
- Server struct pattern: handlers as methods, so they can reach
  the store via `s.store`. Idiomatic shape of every Go service.
- ListenAndServe blocks forever → must run in a goroutine so the
  main goroutine can wait on signals and orchestrate shutdown.
- Middleware wraps a Handler and returns a Handler. Wrapping the
  ResponseWriter is how you observe what the handler wrote.
- Latency numbers show the durability tax: PUT ~500µs (WAL fsync),
  GET ~0s (memory read).



## 2026-10-12 — Stage 5 complete: snapshots + log compaction

**Built:**
- snapshot.go: Save/Load with atomic write (tmp → fsync → rename → fsync dir).
- Store fields: snapshotPath, writesMu, writesSinceSnapshot (atomic int64),
  snapshotThreshold (10,000).
- snapshotNow: writesMu.Lock(), copy shards, Save, truncate WAL, reset counter.
- Set/Delete take writesMu.RLock() (parallel), increment counter, spawn
  snapshot goroutine at threshold.
- Constructor: load snapshot BEFORE WAL replay.
- main.go: forced snapshot on graceful shutdown.

**Verified end-to-end:**
- PUT two keys, Ctrl+C, check disk:
    kvstore.snapshot  64 bytes
    kvstore.wal        0 bytes
- Restart, GET key → survives.

**Learned:**
- TestSnapshotAndRecover initially passed for the WRONG reason — WAL replay
  masked a broken Snapshot that returned early on counter < threshold.
  Fixed by asserting the snapshot file exists and WAL is truncated.
- writesMu RLock vs Lock: with Lock in Set, all writes serialize and
  sharding becomes pointless (15M ops/s → 2k ops/s collapse).
- Recovery order matters: snapshot first, then WAL. Reversed = data loss.
- Atomic rename is the classic pattern for "replace a file safely."