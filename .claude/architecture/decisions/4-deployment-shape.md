# Deployment Shape — Decision Log

Decisions leading to `../designs/4-deployment-shape.md`.

**Updated:** 2026-10-08

---

## D1 — Managed Postgres over self-hosting

**Decision:** Use a managed provider with PostGIS support. Do not self-host Postgres.

**Rationale:** Self-hosting means owning backups, restore testing, version upgrades, connection tuning, disk monitoring and the PostGIS extension lifecycle. None of that is what this project is learning. The learning goals are the Symfony backend, Flutter, and the spatial and ingestion design. Managed hosting buys those hours back at near-zero cost at MVP scale.

**Note on the learning-value principle:** over-engineering is encouraged selectively, where it teaches something about the target stack or the domain (the queue, the push design). Database administration is not in that category.

**Status:** Settled.

---

## D2 — PostGIS availability as a hard filter on providers

**Decision:** Only providers offering PostGIS are candidates.

**Rationale:** Obvious, but it removes several attractive serverless Postgres options and explains why the candidate list is short. D14 adds h3-pg to the same filter.

**Status:** Settled.

---

## D3 — Provider candidates, weighed on the realtime-push constraint

**Decision (original):** Not yet decided. The candidates were Supabase, Neon, DigitalOcean Managed, and Railway or Render Postgres. The decisive constraint was the LISTEN/NOTIFY requirement of the push design, which ruled out providers that front connections with a transaction-mode pooler.

**Status:** ⚠ Superseded by D14 (2026-10-08). The LISTEN/NOTIFY requirement no longer exists (`5-realtime-push` D12).

---

## D4 — Stateless service on a PaaS, no orchestration

**Decision (original):** Deploy the API as a small container image on a platform-as-a-service. No Kubernetes, no service mesh, no orchestration layer.

**Rationale (original):** The API is stateless and small. Kubernetes would be ceremony that teaches infrastructure rather than the target stack.

**Status:** ⚠ Superseded by D15 (2026-10-08). The principle of no orchestration layer carries into D15, which uses docker compose on one host.

---

## D5 — Ingestion as a batch job, not a service

**Decision (original):** The ingestion job is a separate batch execution, not a long-running service. Because the worker is a separate process from the API, the push design needed a cross-process signal.

**Status:** ⚠ Superseded by D15 (2026-10-08). Per-cell ingestion now runs in a messenger worker process (`1-4-dispatch-mechanism`). Bulk backfill remains a manual Console command (D7, D9).

---

## D6 — Geofabrik extracts preferred over Overpass as a production dependency

**Decision:** Prefer static Geofabrik extracts. Treat Overpass as acceptable for scripted, fine-grained queries, not as a hard production dependency.

**Rationale:** Overpass is a shared community resource with no SLA, variable latency, and rate limits set at the operators' discretion. A pipeline that depends on it hard accepts an availability ceiling set by someone else's donated infrastructure. Geofabrik extracts are static files, with no runtime dependency.

**Status:** Settled. Reconciled with the ingestion pipeline's Overpass usage by D9 (2026-09-08). The cold-start failure behaviour is in `1-2-cell-ingestion` D2 and D8.

---

## D7 — Manual trigger at MVP, scheduled later

**Decision:** Manual trigger at MVP. Graduate to a scheduled trigger later.

**Rationale:** OSM bench data changes slowly, so there is no urgency to automate a refresh. A manual trigger keeps a human in the loop while the ingestion logic is young and its failure modes are unknown.

**Graduation mechanism:** the options named at the time were a platform scheduled machine, a platform cron, or GitHub Actions scheduled workflows. The graduation mechanism is now Symfony Scheduler (D16).

**Status:** ⚠ Partially superseded by D16 (2026-10-08); rest Settled. The graduation-mechanism clause is replaced by D16.

---

## D8 — Idempotent upserts prioritised over scheduling sophistication

**Decision:** Idempotent upserts keyed on OSM identity come before any scheduling work.

**Rationale:** Idempotency makes every other operational question cheap. A job that is safe to run twice can be retried, re-run after a failure, triggered by overlapping requests, and scheduled loosely. A job that is not idempotent turns each of those into a data-corruption incident. Correctness first, automation second. The same property is `1-ingestion-pipeline` D3.

**Status:** Settled.

---

## D9 — Geofabrik and Overpass serve different jobs

**Decision:** Geofabrik extracts serve this deployment's bulk backfill, through a Console command. Overpass serves the per-cell demand fills of the ingestion pipeline only. D6's preference for Geofabrik was correct for bulk coverage, and was incomplete as a ban on Overpass anywhere.

**Rationale:** See `1-ingestion-pipeline` D6, which holds the full reasoning.

**Status:** Settled.

---

## D10 — Postgres provider: Neon

**Decision (original):** Neon, with a pooled and a direct connection string, and branching for testing destructive ingestion changes.

**Alternatives considered (original):**

| Option | Why rejected (at the time) |
|---|---|
| Supabase | Its default transaction-mode pooler breaks LISTEN/NOTIFY, the strongest filter on the provider. |
| DigitalOcean Managed | No decisive advantage over Neon at the time, and it loses branching. |
| Railway or Render Postgres | Compelling only if bundled with app hosting on the same platform. |

**Trade-off accepted (original):** scale-to-zero may add latency to the first request after idle.

**Status:** ⚠ Superseded by D14 (2026-10-08). The direct-connection requirement it was chosen for no longer exists (`5-realtime-push` D12).

---

## D11 — API hosting platform: a PaaS with persistent VMs (Fly.io)

**Decision (original):** Fly.io, as a container image in the same region as the database.

**Alternatives considered (original):**

| Option | Why rejected (at the time) |
|---|---|
| Render | A real option, with no specific colocation edge over Fly.io. |
| Single VPS behind a reverse proxy | Cheapest, with the most ops ownership. Rejected because ops ownership is what managed hosting was chosen to avoid. |

**Rationale (original):** colocation with the database cuts round-trip latency. Persistent VMs suited the long-lived listener connection.

**Status:** ⚠ Superseded by D15 (2026-10-08). The long-lived listener it was chosen for no longer exists.

---

## D12 — Ingestion scheduling stays manual at MVP, with a named revisit trigger

**Decision:** No calendar date for moving off manual triggering. The revisit trigger is observed evidence that the 30-day freshness window (`1-1-h3-grid-freshness` D4) is wrong in practice for some area: stale cells flagged as fresh.

**Rationale:** A fixed timeline would tune a knob with no signal behind it. Usage data is the only thing that can show whether benches in the area change faster than 30 days, and that is the only reason to want a scheduled sweep rather than demand-driven fills.

**Status:** Settled, explicitly deferred with a named trigger. The trigger is unchanged. The mechanism is in D16.

---

## D13 — Neon verification fallback: DigitalOcean Managed

**Decision (original):** If Neon's direct connection did not hold a long-lived LISTEN session reliably, fall back to DigitalOcean Managed.

**Status:** ⚠ Superseded by D14 (2026-10-08). DigitalOcean Managed is now the primary choice, and the LISTEN constraint no longer applies.

---

## D14 — Postgres provider: DigitalOcean Managed Postgres

**Decision:** DigitalOcean Managed Postgres, in the same region and VPC as the application host. Hard filter: PostGIS and h3-pg. Pre-implementation check: confirm h3-pg is available on the managed instance. If not, switch providers, checking Neon first, and keep the H3 design (`1-1-h3-grid-freshness`).

**Alternatives considered:**

| Option | Why rejected |
|---|---|
| Neon | Chosen earlier for a direct connection that the LISTEN design needed, which no longer applies. Kept as the first fallback if h3-pg is unavailable on DigitalOcean. |
| Supabase | Its pooler constraint was the reason for rejecting it earlier, and that reason no longer applies. Not re-evaluated in this decision. |

**Rationale:** Same region and VPC as the application host, so the app and worker reach the database over a private network. The earlier reasons for Neon (the direct connection, and branching for testing ingestion) are either gone or weaker: the direct connection is no longer needed, and branching is given up.

**Trade-off accepted:** no branching. Testing destructive ingestion changes against a branch of real data is no longer a built-in option.

**Supersedes:** D3, D10, D13.

**Status:** Settled. The h3-pg check is pre-implementation and open (design Open item 1).

---

## D15 — Application host: a single Droplet running docker compose

**Decision:** One DigitalOcean Droplet, running docker compose with three services: the app (FrankenPHP, with the embedded Mercure hub), the worker (`messenger:consume` and the Scheduler), and RabbitMQ. Same region and VPC as the database.

**Alternatives considered:**

| Option | Why rejected |
|---|---|
| DigitalOcean App Platform with CloudAMQP for RabbitMQ | Rejected in planning. The 2026-10-08 design discussion does not give a separate reason. |
| Fly.io with CloudAMQP for RabbitMQ | Rejected in planning. The 2026-10-08 design discussion does not give a separate reason. |

**Rationale:** The queue is a learning target. Same region and VPC as the database keeps the database on a private network. This reverses D11's rejection of a single VPS, on learning-target grounds.

**Trade-off accepted:** the project owns OS patching and RabbitMQ operations. This sits against D1's managed-services principle. Accepted because the queue is a learning target and the database is not.

**Supersedes:** D4, D5, D11.

**Status:** Settled.

---

## D16 — Scheduling on Symfony Scheduler; ingestion stays a manual Console command

**Decision:** Scheduled work runs on Symfony Scheduler, consumed by the worker. Manual ingestion is a Symfony Console command. Graduation of ingestion to a scheduled trigger uses Scheduler, under the trigger in D12. The visitor purge is a Scheduler recurring message (`3-visitor-identity` D14).

**Alternatives considered:** the earlier graduation options (a platform scheduled machine, platform cron, GitHub Actions scheduled workflows), in D7.

**Rationale:** Scheduler is a newer Symfony component, and learning it is a project goal. The worker that runs the jobs also runs the schedule, so there is no separate scheduling host and no CI secret with database access.

The visitor purge is scheduled from day one while ingestion stays manual because the two risk profiles differ. A monthly DELETE of rows inactive for 12 months is trivial and low-blast-radius. A re-poll sweep against Overpass has rate-limit exposure and failure modes that are poorly understood. Idempotency makes automating ingestion safe, but not yet warranted.

**Supersedes:** the graduation-mechanism clause of D7.

**Status:** Settled.

---

## Unresolved at time of writing

- h3-pg availability on DigitalOcean Managed Postgres (D14; design Open item 1).
- Droplet OS patching cadence, RabbitMQ backup, and Droplet sizing (design Open items 2 to 4).
- Caddy access-log filter for `X-Visitor-Id` (design Open item 5; `3-visitor-identity` D13).
