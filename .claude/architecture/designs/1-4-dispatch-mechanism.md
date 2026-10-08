# Background Dispatch Mechanism — Final Design

**Issue:** [BEN-11](https://benchfinder.youtrack.cloud/issue/BEN-11)  **Status:** settled, not implemented  **Updated:** 2026-10-08

**Parent:** `1-ingestion-pipeline`

---

## Purpose

Run background ingestion work without blocking the request that triggered it. Currently that work is the single ingestion job for a cold or stale cell (`1-2-cell-ingestion`).

---

## Constraints

- **Jobs must survive a process restart.** A queued or in-flight job must not be lost when a container restarts or is redeployed.
- **Failures must be retryable and inspectable.** A failed job goes to a failure transport rather than disappearing.
- **Dispatch must be cheap on the request path.** The request enqueues one message and returns.
- **Low throughput.** One small deployment. The queue is chosen for reliability and learning value, not for volume (see D2).
- A broker is overkill for this volume; chosen for learning value (D2).

---

## Design

### Transport

Symfony Messenger, with an AMQP transport to RabbitMQ. Messages are durable. Failed messages are retried, and a failure transport holds those that exhaust their retries.

### Process layout

```
GET /benches/nearby  (app process, FrankenPHP)
        │  dispatch one message
        ▼
RabbitMQ  (durable queue)
        │
        ▼
messenger:consume  (worker process, same host, docker compose)
        │  runs the ingestion job (1-2-cell-ingestion)
        ▼
PostGIS upserts, then Mercure publish (5-realtime-push)
```

The request process and the worker are separate processes. Ingestion no longer runs inside a request process.

### Message

One message per cold or stale cell. The message carries the res-8 cell index. Its exact shape is an open item.

### Failure path

A failed job is retryable under the transport's retry policy, whose values are not set (Open item 1). A job that exhausts its retries, or is not retried, goes to the failure transport. Cell freshness is not changed by any failure (`1-2-cell-ingestion` D8), so a later request dispatches again.

### Reliability notes

- The failure transport gives failed jobs a place to be inspected rather than silently dropped.

---

## Contracts & data model

No schema of its own. Messenger transport configuration: AMQP to RabbitMQ on the same Droplet (`4-deployment-shape` D15).

Retry count, backoff and failure handling values are not set (open items).

---

## Dependencies

- `1-ingestion-pipeline`: parent.
- `1-2-cell-ingestion`: the job this mechanism dispatches.
- `2-nearby-search`: the request that dispatches.
- `4-deployment-shape`: RabbitMQ is self-hosted on the Droplet. The worker is a compose service.
- `5-realtime-push`: the job's completion triggers the push.
- `6-architecture-setup`: tests use the Messenger in-memory transport (D16 there).

---

## Explicitly deferred

- None beyond the open items.

---

## Superseded

- Dispatch in memory inside the API process, no external broker, no durability across restarts: replaced by Messenger on RabbitMQ (D2).

---

## Open items

1. **Retry policy values:** maximum retries, backoff, and whether a retry of an Overpass failure is wanted at all, given that the next request also re-dispatches (`1-2-cell-ingestion` open item 1).
2. **Message shape:** whether the message carries the cell index only, or the cell plus request context.
3. **Failure transport handling:** how failed messages are inspected and re-driven. Not designed.
