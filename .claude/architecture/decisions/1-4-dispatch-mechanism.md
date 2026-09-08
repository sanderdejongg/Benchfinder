# Background Dispatch Mechanism — Decision Log

Decisions leading to `../designs/1-4-dispatch-mechanism.md`.

**Provenance note:** split out of the pipeline-wide `1-ingestion-pipeline` decision log. The entry below carries fresh, locally-scoped numbering (D1); its prior identity in that log was D7 — see that log for pointers.

---

## D1 — In-process async dispatch; real message queue deferred

**Decision:** Background ingestion jobs run in a goroutine-based worker pool inside the API process. No NATS, SQS, or Redis-backed queue at MVP.

**Rationale:** At MVP scale (one small deployment, one region, no meaningful concurrency), a broker adds an entire piece of infrastructure to run, monitor, and pay for, in exchange for guarantees nothing currently needs.

**Explicit revisit trigger:** The trigger is reliability, not throughput. Specifically: needing pre-warm jobs to survive a process restart, or needing retry-with-backoff and dead-lettering. Throughput alone will not force this change for a long time.

**Learning note:** Building a bounded worker pool from goroutines and channels directly, rather than pulling in a job library, is deliberate — it's a good vehicle for Go's concurrency primitives.

**Status:** Settled.

---

## Unresolved at time of writing

None.
