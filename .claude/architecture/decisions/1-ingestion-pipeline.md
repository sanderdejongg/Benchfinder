# Ingestion Pipeline — Decision Log (pipeline-wide)

Decisions leading to `../designs/1-ingestion-pipeline.md`.

**This log was split.** Component-specific decisions (D3, D4, D5, D6, D7, D9, D13, D14, D15, D16) were relocated to the decision logs of the four sub-designs, where they now carry fresh local numbering. Each original entry is kept below as a stub pointing to its new home — nothing is deleted, per this project's append-only rule for decision history. Cross-cutting decisions that apply to the pipeline as a whole (D1, D2, D8, D10, D11, D12) stay here in full, unchanged, so external references to them by number remain valid.

---

## D1 — Demand-driven ingestion over bulk upfront seed

**Decision:** Ingest OSM bench data lazily, per area, triggered by real user queries. Do not import a full country extract upfront.

**Alternatives considered:**
- Bulk import of a full Geofabrik NL extract at deploy time.
- Hybrid: seed major cities, lazy-load elsewhere.

**Rationale:** A full seed front-loads work for coverage that mostly never gets read, and it makes the refresh story harder (you now own re-importing everything). Demand-driven ingestion keeps the dataset proportional to actual usage and makes the pipeline itself the interesting engineering problem, which matters given the project's learning goal.

**Trade-off accepted:** The first user in any given area pays a latency cost. Mitigated by cascade pre-warming (`1-3-cascade-prewarm` D1) and bounded by the cold-start latency assumption (`1-2-cold-start-ingestion` D1).

**Status:** Settled.

---

## D2 — PostGIS as the sole read-path source; Overpass never queried live for search

**Decision:** User search always reads from PostGIS. Overpass is an ingestion-time dependency only.

**Rationale:** Overpass is a shared community resource with no availability guarantees and variable latency. Making it a hard dependency of every user search would put an unowned third-party service directly on the critical path of the app's core function. It also makes rate limiting somebody else's problem to enforce and ours to get blocked by.

**Status:** Settled.

---

## D3 — H3 hex grid over square grid, geohash, and quadtree

**→ Relocated to `1-1-h3-grid-freshness` decision log as D1.** See that log for the full decision, alternatives table, and rationale.

---

## D4 — Dual resolution seeded into schema, graduation algorithm deferred

**→ Relocated to `1-1-h3-grid-freshness` decision log as D2.**

---

## D5 — Cold start is synchronous and blocking

**→ Relocated to `1-2-cold-start-ingestion` decision log as D1.**

---

## D6 — Cascade pre-warming at k=2..3, non-blocking

**→ Relocated to `1-3-cascade-prewarm` decision log as D1.** Includes the still-open "known issue" about the dangling walking-distance rationale — now tracked as an open item on that design doc, not just a decision-log footnote.

---

## D7 — In-process async dispatch; real message queue deferred

**→ Relocated to `1-4-dispatch-mechanism` decision log as D1.**

---

## D8 — Idempotent upserts keyed on OSM identity

**Decision:** All writes are upserts deduped on `(source, source_id)` — i.e. `('osm', <node/way id>)`. Writes are chunked and transactional.

**Rationale:** Re-polling a cell must always be safe. This is the single property that makes everything else in the pipeline tolerable to get wrong: cascade overlap, retries, a manual re-run, a partial failure mid-ingest. Without it, every one of those becomes a duplicate-data bug. `4-deployment-shape`'s decisions independently flag this as the highest-priority property of the ingestion job, above scheduling sophistication (its D8).

**Status:** Settled. Kept here (not relocated) because it applies across both `1-2-cold-start-ingestion` and `1-3-cascade-prewarm`, and is cited externally by number from `4-deployment-shape`.

---

## D9 — `benches.h3_index` is for reconciliation, not search

**→ Relocated to `1-1-h3-grid-freshness` decision log as D3.**

---

## D10 — OpenBenches rejected as a primary data source

**Decision:** OSM `amenity=bench` is the sole primary source. OpenBenches is not used at MVP.

**Rationale:** OpenBenches covers memorial benches specifically — a niche subset that would give badly skewed coverage for a "where can I sit" utility. It's also a small hobby project, which makes it a fragile production dependency.

**Repositioned, not discarded:** Retained as a possible future *enrichment* layer, adding dedication text or history to benches already sourced from OSM.

**Status:** Settled.

---

## D11 — Licensing position

**Decision:** OSM data is ODbL. Attribution is required and will live on a settings/about screen. Share-alike obligations attach to bulk database redistribution, which the MVP does not do.

**Rationale:** Serving query results from an OSM-derived database is not redistribution of the database itself. The obligation that does bind is attribution, and that's cheap to satisfy properly.

**Status:** Settled.

---

## D12 — Overpass and Geofabrik serve different jobs, stated explicitly

**Decision:** This pipeline's Overpass usage is scoped to bounded, per-cell demand fills only. Any bulk backfill, country-wide seed, or scheduled refresh uses Geofabrik static extracts, run by the separate batch job in `4-deployment-shape`. Neither source substitutes for the other's job.

**Alternatives considered:**

| Option | Why rejected |
|---|---|
| Drop Overpass, use only Geofabrik | Geofabrik extracts are static snapshots; they can't answer "what's near this exact point right now" without a bulk import first — the thing D1 rejected. |
| Drop Geofabrik, use only Overpass | Leaves `4-deployment-shape`'s objection to Overpass as a hard production dependency unaddressed for any future bulk refresh. |

**Rationale:** The two designs each had half of a true answer and neither wrote down the other half. Overpass's bounded-bbox, per-request shape is exactly what this pipeline needs and exactly what `4-deployment-shape`'s objection doesn't cover — that objection is about depending on Overpass for coverage at bulk scale, not about one small query per cold cell. Geofabrik remains correct for the job it was proposed for (bulk backfill), which this pipeline was never trying to do.

**Status:** Settled. Kept here (not relocated) because it's cited externally by number from `4-deployment-shape` (its D9). Also recorded there in full.

---

## D13 — Cascade fan-out capped at depth 1

**→ Relocated to `1-3-cascade-prewarm` decision log as D2.**

---

## D14 — Freshness window set to 30 days

**→ Relocated to `1-1-h3-grid-freshness` decision log as D4.**

---

## D15 — Cold-start failure returns empty, does not mark the cell polled

**→ Relocated to `1-2-cold-start-ingestion` decision log as D2.**

---

## D16 — Explicit 5s timeout on the Overpass HTTP call

**→ Relocated to `1-2-cold-start-ingestion` decision log as D3.**

---

## Unresolved at time of writing

None outstanding pipeline-wide. See each sub-design's decision log for component-specific open items (notably `1-3-cascade-prewarm`'s dangling rationale reference).
