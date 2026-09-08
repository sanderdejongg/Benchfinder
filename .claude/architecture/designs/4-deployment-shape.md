# Deployment Shape — Final Design

**Status:** Shape settled, specific providers not yet chosen

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

Not yet chosen.

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

**Data source preference:** Geofabrik static extracts over live Overpass queries where possible. Overpass is acceptable for finer-grained scripted queries but is not desirable as a hard production dependency.

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

1. **Choose a specific managed Postgres provider.** Note the realtime-push design's `LISTEN/NOTIFY` constraint above — it likely narrows this more than any other factor.
2. **Choose a specific API hosting platform.**
3. **Decide when to graduate from manual to scheduled ingestion.**
4. **Reconcile the Geofabrik preference with the ingestion pipeline design.** That design's entire model is demand-driven live Overpass queries per H3 cell, which is directly at odds with this design's stated preference for static extracts. The two probably serve different jobs — Overpass for per-cell demand fills, Geofabrik for bulk refresh — but that split is currently implied rather than written down anywhere.
