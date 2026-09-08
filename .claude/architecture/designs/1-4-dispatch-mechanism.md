# Background Dispatch Mechanism — Final Design

**Parent:** `1-ingestion-pipeline`
**Status:** Settled for MVP, not implemented
**Updated:** 2026-09-08

---

## Purpose

Execute background ingestion work — currently: cascade pre-warm jobs from `1-3-cascade-prewarm` — without blocking the request that triggered it.

---

## Constraints

- MVP scale: one small deployment, no meaningful concurrency requirement yet.
- Should not require new operational infrastructure to solve a throughput problem that doesn't currently exist.

---

## Design

**MVP: in-process async.** A goroutine-based worker pool with a bounded queue inside the API process.

A real message broker (NATS, SQS, Redis-backed queue) is explicitly deferred. At MVP scale the added operational surface buys nothing. The trigger to revisit is a *reliability* requirement rather than a throughput one — specifically, needing pre-warm jobs to survive a process restart, or needing retry semantics with backoff and a dead-letter path.

Job payloads carry the depth-1 cascade flag from `1-3-cascade-prewarm` D2, checked before a worker decides whether to re-dispatch.

**Learning note:** Building a bounded worker pool from goroutines and channels directly, rather than pulling in a job library, is deliberate — it's a good vehicle for Go's concurrency primitives.

---

## Contracts & data model

None — pure execution mechanism, no schema of its own.

---

## Dependencies

- `1-ingestion-pipeline` — parent.
- `1-3-cascade-prewarm` — the only consumer at MVP.
- `4-deployment-shape` — the API process's restart behaviour bears directly on the "jobs lost on restart" gap below.
- `5-realtime-push` — this design's in-process, single-instance dispatch is *why* that design needs cross-instance fan-out via `LISTEN/NOTIFY` rather than an in-process event bus (see that design's §3 "Cross-process signal" and decision D9).

---

## Explicitly deferred

- **Real message queue** for background dispatch — see revisit trigger above.
- **Job durability across a process restart.** A known gap: an in-flight or queued job is lost if the API process restarts. Not addressed until the broker deferral above is revisited. `5-realtime-push`'s open items reference this as a source of "ingestion never completes" cases its socket timeout has to handle.

---

## Superseded

None.

---

## Open items

None outstanding at time of writing.
