# AutOps — Engineering Log

This is a running log of real architecture decisions, incidents, and benchmarks as the project is built — not a plan, a record of what actually happened and why. Each entry follows the same shape: **what we tried → what broke → why → what fixed it → the number that proves it.**

This file grows over time. New entries go at the top (most recent first). Cursor: when asked to log a milestone here, follow the existing entry format below rather than inventing a new one.

---

## Entry format (for new entries)

```
## [Date] — Short title

**What we were building:** one sentence.

**What happened:** the observed problem, in plain terms.

**Why it happened:** the root cause, named clearly.

**The fix:** what changed, architecturally.

**Before / after:**
| Metric | Before | After |
|---|---|---|
| ... | ... | ... |

**The principle this reinforces:** one sentence tying it back to a core architecture rule.
```

---

## 2026-09-12 — Redis on the hot path

**What we were building:** replacing the in-memory "is this tenant in shaping mode?" flag with Redis, so the mode survives a proxy restart and can eventually be shared across multiple proxy instances.

**What happened:** after wiring the proxy to call Redis directly on every incoming request, p95 latency jumped from ~7ms (shaping) to 118ms, with some requests taking almost a full second.

**Why it happened:** this violated the core architecture rule set at the very start of the project — *the hot request path must never make a synchronous network call*. Checking Redis on every request adds a real network round-trip (through Docker Desktop's virtualization layer on Windows, which is slower than "local" sounds) directly into the critical path every single client waits on. Under concurrent load, requests also started competing for a limited pool of Redis connections, which is what caused the worst outliers.

**The fix:** moved back to the original design — the proxy reads mode from a **local in-memory map** on every request (a plain, fast memory read, no network involved). A background goroutine refreshes that local map from Redis every 500ms, completely decoupled from any individual request. Writes (an admin flipping a tenant to shaping mode) still go straight to Redis, but *also* update the local map immediately, so a change is visible right away without waiting for the next refresh cycle. If Redis is slow or down, the background refresh just logs a warning and keeps serving the last-known values — the hot path is never affected either way.

**Before / after:**
| Metric | Before (sync Redis) | After (local cache + async refresh) |
|---|---|---|
| Shaping p95 | 118ms | ~15ms |
| Normal p95 | (not isolated separately) | ~21ms |
| Shaping median | — | ~1.0ms |

**The principle this reinforces:** the data plane (proxy, hot path) and control plane (Redis, the future ML forecaster) must stay strictly separated. The data plane only ever reads a local cache; anything that talks to the network happens asynchronously, on its own schedule, never in the path of a request a real user is waiting on. This is the same pattern used by SDN and Envoy — and it's a design principle proven by a real regression, not just a diagram.

---

## Milestone benchmarks (running summary)

| Milestone | Normal mode p95 | Shaping mode p95 | Notes |
|---|---|---|---|
| Bare proxy, no Redis | ~4ms | ~7ms | First working baseline |
| Redis, synchronous (buggy) | — | 118ms | Regression — see entry above |
| Redis, local cache + async refresh | ~21ms | ~15ms | Current |

cd proxy
ADMIN_TOKEN=test123 make bench