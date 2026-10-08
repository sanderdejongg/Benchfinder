# Ingestion Pipeline — Decision Log (pipeline-wide)

Decisions leading to `../designs/1-ingestion-pipeline.md`.

**Updated:** 2026-10-08

Component-specific decisions live in the sub-design logs: `1-1-h3-grid-freshness`, `1-2-cell-ingestion`, `1-4-dispatch-mechanism`. This log holds the pipeline-wide decisions only.

---

## D1 — Demand-driven ingestion over bulk upfront seed

**Decision:** Ingest OSM bench data lazily, per area, triggered by real user queries. Do not import a full country extract upfront.

**Alternatives considered:**

| Option | Why rejected |
|---|---|
| Bulk import of a full Geofabrik extract at deploy time | Front-loads work for coverage that mostly never gets read. Makes refresh harder, because the whole dataset must be re-imported. |
| Hybrid: seed major cities, lazy-load elsewhere | Rejected in the original decision. No separate argument was recorded. |

**Rationale:** Demand-driven ingestion keeps the dataset proportional to actual use. It also makes the pipeline itself the engineering problem worth learning, which matters for a learning project.

**Trade-off accepted:** the first user in any area pays a latency cost. This is mitigated by the k=2 halo fill that the same background job performs (`1-2-cell-ingestion` D8).

**Status:** Settled.

---

## D2 — PostGIS as the sole read-path source; Overpass never queried live for search

**Decision:** User search always reads from PostGIS. Overpass is an ingestion-time dependency only.

**Rationale:** Overpass is a shared community resource with no availability guarantee and variable latency. Making it a hard dependency of every search would put an unowned third-party service on the critical path of the app's core function. It would also make rate limiting someone else's problem to enforce, and ours to get blocked by.

**Status:** Settled.

---

## D3 — Idempotent upserts keyed on OSM identity

**Decision:** All writes are upserts deduplicated on `(source, source_id)`, i.e. `('osm', <node or way id>)`. Writes are chunked and transactional.

**Rationale:** Re-ingesting a cell must always be safe. This property makes everything else tolerable: overlapping jobs, retries, a manual re-run, a partial failure mid-ingest. Without it, each of those becomes a duplicate-data bug. `4-deployment-shape` D8 ranks it above scheduling sophistication for the same reason.

**Status:** Settled. Cited by `1-2-cell-ingestion` and `4-deployment-shape`.

---

## D4 — OpenBenches rejected as a primary data source

**Decision:** OSM `amenity=bench` is the sole primary source. OpenBenches is not used at MVP.

**Rationale:** OpenBenches covers memorial benches specifically, a niche subset that would skew coverage badly for a "where can I sit" utility. It is also a small hobby project, which makes it a fragile production dependency.

**Repositioned, not discarded:** it may return as an enrichment layer that adds dedication text or history to benches already sourced from OSM.

**Status:** Settled.

---

## D5 — Licensing: ODbL attribution in-app

**Decision:** OSM data is ODbL. Attribution is required and lives on a settings or about screen. Share-alike obligations attach to bulk database redistribution, which MVP does not do.

**Rationale:** Serving query results from an OSM-derived database is not redistribution of the database. The obligation that does bind is attribution, which is cheap to satisfy properly.

**Status:** Settled.

---

## D6 — Overpass and Geofabrik serve different jobs

**Decision:** The pipeline's Overpass usage is scoped to bounded, per-cell demand fills only. Bulk backfill, country-wide seed or scheduled refresh uses Geofabrik static extracts, through the batch command in `4-deployment-shape`. Neither source substitutes for the other's job.

**Alternatives considered:**

| Option | Why rejected |
|---|---|
| Overpass only | Leaves bulk refresh dependent on a shared community service with no SLA. |
| Geofabrik only | Extracts are static snapshots. They cannot answer "what is near this exact point now" without the bulk import rejected in D1. |

**Rationale:** Overpass's bounded, per-request shape is exactly what a cold cell needs. The objection to Overpass in `4-deployment-shape` is about depending on it for coverage at bulk scale, not about one bounded query per cold cell. Geofabrik remains right for bulk work.

**Status:** Settled. Recorded in full in `4-deployment-shape` D9.

---

## D7 — One ingestion job per cold or stale cell

**Decision:** A cold or stale cell dispatches one message. The job makes one Overpass bbox request covering the k=2 H3 disk around the cell (19 cells, about 2 km). It then assigns benches to cells, upserts, marks the cells polled, and publishes one push. Details: `1-2-cell-ingestion` D8.

**Alternatives considered:**

| Option | Why rejected |
|---|---|
| Two jobs: the user's cell first, then the surrounding ring | The user's cell arrives sooner, but costs two Overpass calls and keeps a depth cap. Under Overpass rate limits, one call is preferred. |
| Separate job per neighbouring ring cell, as in the earlier cascade design (up to 18 to 36 Overpass calls) | Multiplies the rate-limit exposure of the bounded fill. The coverage it adds is the same disk one bounded call already covers. |

**Rationale:** One Overpass call instead of up to 18 to 36 matters under Overpass rate limits. The single job also removes the need for a depth cap, since nothing re-dispatches. Holds only while `pending_cells` is limited to cells inside the job's disk (open item on `pending_cells` scope).

**Trade-off accepted:** the push for the user's own cell waits for the whole disk to complete, rather than arriving as soon as the user's own cell is done.

**Status:** Settled.

---

## Unresolved at time of writing

None at pipeline level. Sub-design open items are listed in their own design docs.
