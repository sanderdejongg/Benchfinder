# Deployment Shape — Final Design

**Issue:** [BEN-4](https://benchfinder.youtrack.cloud/issue/BEN-4)  **Status:** settled, not implemented; one pre-implementation check open  **Updated:** 2026-10-08

---

## 1. Purpose

A practical, low-overhead way to run the MVP: no accounts, one region of bench data, and a small backend that serves REST, pushes events and runs background ingestion. The goal is to stand it up without maintaining infrastructure as a side project inside a side project, while still using the infrastructure the learning goals call for.

---

## 2. Constraints

- **Low overhead.** Running the platform must not become a project in its own right. The exception is the queue, which is a learning target (D15).
- **Single region.** One region of bench data. The application host and the database share a region and a VPC, so they reach each other on a private network.
- **No accounts.** No identity provider or authentication infrastructure to host.
- **Small stateless API.** A REST service, an embedded push hub, and one background worker. No orchestration layer (D15).

---

## 3. Overall shape

```
DigitalOcean Droplet (docker compose)                DigitalOcean Managed Postgres
┌─────────────────────────────────────┐              ┌──────────────────────────┐
│ app: FrankenPHP                     │              │ PostgreSQL + PostGIS      │
│   REST API, Mercure hub (embedded)  │─────────────►│ + h3-pg                   │
│ worker: messenger:consume           │   same region│                          │
│   ingestion jobs, Scheduler         │   and VPC    └──────────────────────────┘
│ rabbitmq                            │
└─────────────────────────────────────┘
        ▲                       │
        │ REST, SSE             │ push events
        │                       ▼
              Mobile app (Flutter)

Bulk backfill: a Symfony Console command reading a Geofabrik extract, run manually.
```

Three services on one Droplet, one managed database. Nothing else.

Scope notes:

- The **app** container serves REST and hosts the Mercure hub (`5-realtime-push` D13). The hub is embedded; there is no separate hub service.
- The **worker** container runs `messenger:consume`. It executes per-cell ingestion jobs (`1-2-cell-ingestion`) and the Symfony Scheduler, which runs the visitor purge (`3-visitor-identity` D14).
- The **rabbitmq** container is the AMQP broker for Messenger (`1-4-dispatch-mechanism` D2).
- The bulk Geofabrik backfill is a Symfony Console command, separate from the per-cell path (decision log D6, D9).

---

## 4. Postgres / PostGIS hosting

Managed, not self-hosted (D1).

Hard filter: PostGIS and h3-pg (D2, D14). Also verify the `h3_postgis` extension.

**Chosen: DigitalOcean Managed Postgres** (D14). Same region and VPC as the Droplet, so the app and worker reach it on a private network.

**Pre-implementation check, binary:** confirm h3-pg is available on the managed instance before provisioning. If it is not, switch provider (check Neon first) and keep the H3 design (`1-1-h3-grid-freshness`). This does not fork the design.

Trade-off accepted: DigitalOcean Managed Postgres does not offer Neon's branching. Testing destructive ingestion changes against a branch of real data is no longer a built-in option.

---

## 5. Application host

**Chosen: a single DigitalOcean Droplet running docker compose** (D15). Services: app (FrankenPHP with the embedded Mercure hub), worker (`messenger:consume` and the Scheduler), and RabbitMQ. Same region and VPC as the database.

Trade-off accepted: the project owns OS patching and RabbitMQ operations. This is in tension with the managed-services principle (D1). It is accepted because the queue is a learning target and the database is not.

Single app instance at MVP. Adding a second app instance is the trigger for a standalone Mercure hub (`5-realtime-push` D13).

---

## 6. Ingestion

Per-cell ingestion is a Messenger message consumed by the worker (`1-2-cell-ingestion`, `1-4-dispatch-mechanism`). It is not a batch job.

Bulk backfill from Geofabrik extracts is a Symfony Console command, run manually. Geofabrik is preferred for bulk coverage. Overpass is used only for per-cell fills (D9).

Ingestion scheduling stays manual until its named trigger (D12). When automated, it uses Symfony Scheduler (D16).

Priority ordering: idempotent upserts before scheduling sophistication (D8).

Why the purge is scheduled from day one while ingestion stays manual: the two risk profiles differ. A monthly DELETE of rows inactive for 12 months is trivial and low-blast-radius. A re-poll sweep against Overpass has rate-limit exposure and failure modes that are poorly understood. Idempotency makes automating ingestion safe, but not yet warranted.

---

## 7. Related deployment concerns from other designs

- **Visitor purge.** A Symfony Scheduler recurring message, consumed by the worker (`3-visitor-identity` D14). It needs no CI secret and no path from CI to the database.
- **Push.** The Mercure hub runs inside the app container (`5-realtime-push` D13). It needs no separate connection from the database.
- **Access logs.** The app container's access log must omit the `X-Visitor-Id` request header (open item 5), or access logging is off. This is what keeps the visitor ID out of logs (`3-visitor-identity` D13).
- **CI.** GitHub Actions runs lint, static analysis and tests (`6-architecture-setup` D10, D16, D17).

---

## 8. Contracts & data model

No schema. Compose services: app (FrankenPHP + embedded Mercure hub), worker (messenger:consume), rabbitmq. One managed database.

---

## 9. Dependencies

- `1-ingestion-pipeline`: parent. The Overpass/Geofabrik split and the batch command.
- `1-2-cell-ingestion`: the per-cell job that runs in the worker.
- `1-4-dispatch-mechanism`: RabbitMQ and Messenger, both on the Droplet.
- `3-visitor-identity`: the purge on Scheduler, and the access-log requirement.
- `5-realtime-push`: the embedded hub and single-instance topology.
- `6-architecture-setup`: CI on GitHub Actions, and the test database.

---

## 10. Explicitly deferred

- **Scheduled ingestion**, until the trigger in D12 is observed (stale-but-fresh cells in real usage). Then it runs on Symfony Scheduler (D16).

---

## 11. Superseded

- Neon as the database provider (D10, D13): replaced by DigitalOcean Managed Postgres (D14).
- The LISTEN-driven provider constraint (D3): no longer applies; provider choice is D14.
- PaaS hosting on Fly.io (D4, D11): replaced by a single Droplet with docker compose (D15).
- Ingestion as a separate batch process (D5): per-cell ingestion now runs in the worker (D15).
- Platform-cron graduation clause of D7: replaced by Symfony Scheduler (D16).

---

## 12. Open items

1. **h3-pg availability on DigitalOcean Managed Postgres.** Pre-implementation check (section 4). Fallback: switch providers, starting with Neon. Also verify the `h3_postgis` extension.
2. **Droplet OS patching cadence.** Not decided.
3. **RabbitMQ durability and backup.** Queue durability is set at the transport (`1-4-dispatch-mechanism`). Whether RabbitMQ state is backed up, and how, is not decided.
4. **Droplet sizing.** Not decided. No load figures exist yet.
5. **Caddy access-log configuration must filter `X-Visitor-Id`.** FrankenPHP is Caddy. Caddy's JSON access log records request headers and the full URI, which carries lat/lon. The log filter must delete `request>headers>X-Visitor-Id`, or access logging must be off. Until this is configured and checked, the no-visitor-ID-in-logs rule is not met (`3-visitor-identity` D13).

Tension, recorded: a self-hosted RabbitMQ sits against the managed-services principle (section 5). Accepted, not resolved.
