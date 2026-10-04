# Devlog

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