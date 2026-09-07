# BEN-4 — Deployment Shape: Decision Log

Decisions leading to the design in `BEN-4-deployment-shape-design.md`. This epic is the least settled of the six — most entries below are *shape* decisions with the specific vendor still open.

---

## D1 — Managed Postgres over self-hosting

**Decision:** Use a managed provider with PostGIS support. Do not self-host Postgres.

**Rationale:** Self-hosting Postgres means owning backups, restore testing, version upgrades, connection tuning, disk monitoring, and the PostGIS extension lifecycle. None of that is the thing being learned here — the learning goals are Go and Flutter, plus the spatial and ingestion design. Managed hosting buys those hours back at a cost that is effectively zero at MVP scale.

**Note on the learning-value principle:** the project's "over-engineering is encouraged" principle is applied selectively. It applies where the complexity teaches something about the target languages or the domain (see BEN-11's hand-rolled worker pool, BEN-5's WebSocket choice). Database administration isn't in that category.

---

## D2 — PostGIS availability as a hard filter on providers

**Decision:** Only providers offering PostGIS are candidates.

**Rationale:** Obvious but worth recording, because it eliminates several otherwise attractive serverless Postgres options and is the reason the candidate list is as short as it is.

---

## D3 — Provider candidates and what each is being weighed on

**Not yet decided.** The comparison as it stands:

| Provider | Attraction | Concern |
|---|---|---|
| Supabase | PostGIS via a toggle; generous free tier; a ready path to auth/storage if ever needed | Default connection pooling is pgBouncer in transaction mode, which **breaks `LISTEN/NOTIFY`** — a hard conflict with BEN-5 unless a direct connection is used |
| Neon | Scales to zero when idle; **branching** allows testing destructive ingestion changes against a branch of real data | Cold starts; same pooler question needs checking against BEN-5 |
| DigitalOcean Managed | Simple, predictable, good if colocated with a DO-hosted API | Less free-tier generosity |
| Railway / Render Postgres | One vendor for app and database | Tied to the app-hosting decision |

**The decisive constraint is likely BEN-5.** The design there requires each API instance to hold a long-lived `LISTEN` connection. Any provider that fronts everything with a transaction-mode pooler makes that impossible without bypassing the pooler. This is a stronger filter than price or free-tier size and should probably drive the choice.

**Neon's branching is called out specifically** because ingestion is destructive-ish (upserts, reconciliation, potential deletes) and being able to test a schema or reconciliation change against a branch of real data is directly useful for BEN-1's work.

---

## D4 — Stateless Go service, no orchestration platform

**Decision:** Deploy the Go binary in a small Docker image on a PaaS. No Kubernetes, no service mesh, no orchestration layer.

**Rationale:** The API is stateless and small. Kubernetes would be pure ceremony — and unlike the deliberate over-engineering elsewhere in this project, it would teach infrastructure rather than Go.

**Leaning:** Fly.io or Render. Fly.io's colocation with the database is a genuine latency benefit given the read path may include a synchronous Overpass fetch. Render's push-to-deploy is the lower-friction option.

**Kept on the table:** a single VPS behind Caddy, as the cheapest option with the most ops ownership. Rejected as the default because ops ownership is the thing D1 was trying to avoid.

---

## D5 — Ingestion as a batch job, not a service

**Decision:** The ingestion job is a separate batch execution, not a long-running service.

**Rationale:** It has no request-response responsibilities and no need to be continuously available. Running it as a job keeps it independently triggerable, independently failable, and independently observable.

**Consequence that shows up in BEN-5:** because the ingestion worker is a *separate process* from the API, an in-process event bus cannot signal from one to the other. That's the whole reason BEN-5 reaches for Postgres `LISTEN/NOTIFY`. This deployment decision directly forced that architectural one.

---

## D6 — Geofabrik extracts preferred over Overpass as a production dependency

**Decision:** Prefer static Geofabrik extracts. Treat Overpass as acceptable for scripted, fine-grained queries but not as a hard production dependency.

**Rationale:** Overpass is a shared community resource with no SLA, variable latency, and rate limits enforced at their discretion. Building a product whose data pipeline hard-depends on it means accepting an availability ceiling set by someone else's donated infrastructure. Geofabrik extracts are static files — download once, process locally, no runtime dependency at all.

**⚠ Unreconciled with BEN-1.** BEN-1's entire architecture is demand-driven live Overpass queries per H3 cell, including a *synchronous* one on the user's critical path at cold start. That is exactly the production dependency this decision says to avoid.

The two are probably reconcilable — Overpass for demand-driven per-cell fills where a bounded bbox query is the only sensible tool, Geofabrik for any bulk refresh or backfill sweep — but that division of labour is currently implicit. It should be written into one epic or the other, along with an answer to what the cold-start path does when Overpass is unavailable.

---

## D7 — Manual trigger at MVP, scheduled later

**Decision:** `make ingest` / a CLI command at MVP. Graduate to a scheduled trigger (Fly scheduled machine, Render/Railway cron, or GitHub Actions `on: schedule`) later.

**Rationale:** OSM bench data changes slowly. There is no urgency to automate a refresh that could reasonably run monthly. Manual triggering also keeps a human in the loop while the ingestion logic is still young and its failure modes are unknown.

**GitHub Actions noted as viable** because a scheduled workflow connecting to a managed Postgres needs no hosting-platform cron primitive at all — which usefully decouples this from the still-open D3 and D4 decisions.

---

## D8 — Idempotent upserts prioritised over scheduling sophistication

**Decision:** Stated as an explicit priority ordering: get idempotent upserts keyed on OSM node/way ID right before investing in scheduling.

**Rationale:** Idempotency is what makes every other operational question cheap. A job that is safe to run twice can be retried blindly, re-run manually after a failure, triggered by overlapping cascades, and scheduled sloppily. A job that is *not* idempotent turns every one of those into a data-corruption incident. Correctness first, automation second.

This is the same property BEN-1 independently identified as foundational (its D8), which is a good sign it's the right thing to prioritise.

---

## Unresolved at time of writing

- **Specific Postgres provider** — likely determined by the `LISTEN/NOTIFY` constraint from BEN-5 more than anything else.
- **Specific API hosting platform.**
- **When to graduate to scheduled ingestion.**
- **The Geofabrik / Overpass split** (see D6) — the most substantive open item in this epic, since it touches BEN-1's core architecture.
- **CGo build implications** of `uber/h3-go` on whichever base image and platform is chosen.
