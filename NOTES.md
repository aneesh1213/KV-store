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