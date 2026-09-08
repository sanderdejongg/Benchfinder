# Deployment Shape — Final Design

**Status:** Shape settled, Postgres provider chosen provisionally, API host still open
**Updated:** 2026-09-08

---

## 1. Purpose

A practical, low-overhead way to run the MVP. No accounts, one or few regions of bench data, a small stateless REST API. The goal is to stand something up quickly and not end up maintaining infrastructure as a side project within a side project.

---

## 2. Overall shape

```
GitHub Actions (scheduled) or manual CLI run
                │
                ▼
      Ingestion job (Go, batch)
                │
                ▼
   Managed Postgres + PostGIS  ◄──────┐
                │                     │
                ▼                     │
      Go REST API (Fly.io / Render) ──┘
                │
                ▼
          Mobile app (Flutter)
```

Three moving pieces: a database, a stateless service, and a batch job. Nothing else.

**Scoping note:** the batch job above is the Geofabrik-based bulk/backfill path only (see D9). The Go REST API box also runs two other ingestion-adjacent mechanisms defined in the ingestion-pipeline design — the synchronous per-cell Overpass cold-start fetch, and the in-process cascade pre-warm worker pool — neither of which is a separate process. This distinction is load-bearing for the realtime-push design's `LISTEN/NOTIFY` justification (see that design's decision D9).

---

## 3. Postgres / PostGIS hosting

**Decision: managed provider, not self-hosted.**

Candidates under consideration:

| Provider | Notes |
|---|---|
| **Supabase** | PostGIS enabled via a toggle. Generous free tier. Provides an easy later path to auth and storage if the product ever needs them. |
| **Neon** | Serverless, scales to zero when idle. Branching is genuinely attractive here — it allows testing destructive ingestion changes against a branch of real data. |
| **DigitalOcean Managed Databases** | Simple and predictable if the API is also on DO. |
| **Railway / Render Postgres** | Fine if the app already lives on that platform; one vendor, one bill. |

**Chosen, provisionally: Neon.** It offers a direct, non-pooled connection string alongside its pooled one — which the realtime-push design's long-lived `LISTEN` connection needs — plus branching, which is directly useful for testing the ingestion pipeline's destructive-ish upsert/reconciliation changes against real data. Supabase is ruled out: its default pgBouncer transaction-mode pooling breaks `LISTEN/NOTIFY` outright. **Provisional** pending explicit verification that Neon's direct connection is usable for a long-lived `LISTEN` session in practice, not just documented as available (see decision D10).

---

## 4. Go service deployment

Stateless small API. No orchestration platform needed.

| Option | Notes |
|---|---|
| **Fly.io** | Go binary in a Docker image, can be colocated near the managed Postgres. Cheap. |
| **Railway / Render** | Push-to-deploy from GitHub, minimal configuration. |
| **Single small VPS (Hetzner / DO) behind Caddy** | Cheapest by a margin, most ops ownership. |

**Leaning:** Fly.io or Render — Go binary in a small Docker image, managed Postgres reached via a connection string in an environment variable.

**Build note carried from the ingestion pipeline's H3 grid work:** `uber/h3-go` is CGo-based. That rules out the trivial `CGO_ENABLED=0` static binary and means the Docker build needs a C toolchain, with a multi-stage build to keep the final image small. Worth knowing before the base image is picked.

---

## 5. OSM ingestion job

A batch job, not a live service.

**Data source preference:** Geofabrik static extracts over live Overpass queries where possible. Overpass is acceptable for finer-grained scripted queries but is not desirable as a hard production dependency for this job. This is now explicitly scoped against the ingestion-pipeline design's own Overpass usage — see D9 there and D9 here: this batch job owns bulk/backfill via Geofabrik, the ingestion pipeline owns per-cell demand fills via Overpass, and neither substitutes for the other.

**Trigger, staged:**

- **MVP:** manual trigger. A CLI command or `make ingest`, run locally or as a one-off job on the deploy platform.
- **Next:** scheduled. Fly.io scheduled machine, a Render/Railway cron primitive, or a GitHub Actions `on: schedule` workflow connecting to the managed Postgres.

**Priority ordering, stated explicitly:** idempotent upserts keyed on OSM node/way ID matter more than scheduling sophistication. A job that can be safely run twice is worth more than a job that runs on a perfect schedule.

---

## 6. Related deployment concerns from other designs

These land here even though they originate elsewhere:

- **Visitor-identity retention job.** The 12-month `Visitor` purge needs a scheduled execution mechanism — the same one the ingestion job will use. This is why the ops work in the visitor-identity design is blocked on this one.
- **Realtime-push LISTEN connection.** Each API instance holds a long-lived Postgres `LISTEN` connection separate from its query pool. Any provider with an aggressive connection pooler in transaction mode (notably Supabase's pgBouncer default) will break `LISTEN/NOTIFY`. This is a real constraint on the provider choice, not a detail.
- **Architecture-setup CI.** The lint and test pipeline wires into whatever CI exists here.

---

## 7. Open items

1. **Choose a specific API hosting platform.** Leaning Fly.io or Render (§4), not committed.
2. **Decide when to graduate from manual to scheduled ingestion.**
3. **Verify Neon's direct-connection support for `LISTEN/NOTIFY`** before treating the Postgres provider choice (D10) as final rather than provisional.
