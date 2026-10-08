# Background Dispatch Mechanism — Decision Log

Decisions leading to `../designs/1-4-dispatch-mechanism.md`.

**Updated:** 2026-10-08

---

## D1 — In-process async dispatch; real message queue deferred

**Decision:** Background ingestion jobs run in memory inside the API process, with no external broker at MVP.

**Alternatives considered:** a managed or self-hosted broker (NATS, SQS, Redis-backed queue), deferred.

**Rationale:** At MVP scale (one small deployment, one region, no meaningful concurrency), a broker adds infrastructure to run and monitor for guarantees nothing currently needs.

**Revisit trigger:** a reliability requirement, not a throughput one. Specifically: jobs must survive a process restart, or retries with backoff and a dead-letter path are needed. Throughput alone would not force the change.

**Status:** ⚠ Superseded by D2 (2026-10-08). The revisit trigger was met: jobs must survive a restart, and retries and a failure path are needed.

---

## D2 — Symfony Messenger on RabbitMQ (AMQP) for background jobs

**Supersedes:** D1.

**Decision:** Background ingestion jobs are Symfony Messenger messages on an AMQP transport to RabbitMQ. Messages are durable and retryable, with retry values open. A failure transport holds messages that fail for good.

**Alternatives considered:**

| Option | Why rejected |
|---|---|
| Doctrine transport with `use_notify` | Needs a direct, non-pooled database connection for LISTEN, which constrains the provider choice. |
| Doctrine transport, polled | Roughly 1 s of latency on every async job. That delays the backfill the pending signal depends on (`2-nearby-search` D14). |
| Redis as the queue | New infrastructure that exists only for the queue. |

**Rationale:** Durable messages and retries close the gap where queued jobs were lost on restart (D1). A failure transport makes failures inspectable. Honest reason: chosen for learning value. The job volume at MVP does not require a broker. A broker here is overkill for the volume, and it was chosen anyway.

**Trade-off accepted:** a self-hosted RabbitMQ on the Droplet, with its patching and operation falling to this project (`4-deployment-shape` D15). This sits in tension with the managed-services principle (`4-deployment-shape` D1). Accepted because the queue is a learning target and the database is not.

**Status:** Settled.

---

## Unresolved at time of writing

- Retry values, backoff, and failure-transport handling (design Open items 1 and 3).
- Message shape (design Open item 2).
