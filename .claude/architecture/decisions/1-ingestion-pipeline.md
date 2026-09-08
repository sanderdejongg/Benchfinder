# Ingestion Pipeline — Decision Log

Decisions leading to `../designs/1-ingestion-pipeline.md`, including alternatives that were considered and rejected. Covers the pipeline as a whole and its H3 grid, cold-start, cascade pre-warming, and dispatch mechanism components.

---

## D1 — Demand-driven ingestion over bulk upfront seed

**Decision:** Ingest OSM bench data lazily, per area, triggered by real user queries. Do not import a full country extract upfront.

**Alternatives considered:**
- Bulk import of a full Geofabrik NL extract at deploy time.
- Hybrid: seed major cities, lazy-load elsewhere.

**Rationale:** A full seed front-loads work for coverage that mostly never gets read, and it makes the refresh story harder (you now own re-importing everything). Demand-driven ingestion keeps the dataset proportional to actual usage and makes the pipeline itself the interesting engineering problem, which matters given the project's learning goal.

**Trade-off accepted:** The first user in any given area pays a latency cost. Mitigated by D6 (cascade pre-warming) and bounded by D5's assumption about Overpass latency.

---

## D2 — PostGIS as the sole read-path source; Overpass never queried live for search

**Decision:** User search always reads from PostGIS. Overpass is an ingestion-time dependency only.

**Rationale:** Overpass is a shared community resource with no availability guarantees and variable latency. Making it a hard dependency of every user search would put an unowned third-party service directly on the critical path of the app's core function. It also makes rate limiting somebody else's problem to enforce and ours to get blocked by.

---

## D3 — H3 hex grid over square grid, geohash, and quadtree

**Decision:** H3, resolution 8, as the spatial tracking grain.

**Alternatives considered:**

| Option | Why rejected |
|---|---|
| Fixed square grid in a projected CRS | Cells distort with latitude unless you commit to a projection; adjacency is lumpy — diagonal neighbours are 1.41x further than orthogonal ones, which makes "expand outward by one ring" ill-defined. |
| Geohash | Same square-cell adjacency problems, plus the well-known edge artefact where nearby points can have very different prefixes. |
| Density-adaptive quadtree | Genuinely the most "correct" answer for a 20x density spread, but the implementation and reconciliation complexity is large, and it front-loads the hardest part of the problem. |

**Rationale:** Hexagons have uniform adjacency — six neighbours, all equidistant, all edge-sharing. That makes `gridRing`/`gridDisk` a clean primitive for the cascade in D6. H3 also handles the latitude problem natively. It sits at the right point on the complexity curve: more principled than squares, far less work than a quadtree.

**Learning value noted:** H3 is a widely used piece of industry infrastructure, so the concepts transfer.

---

## D4 — Dual resolution seeded into schema, graduation algorithm deferred

**Decision:** Include a `resolution` column in `polled_cells` and adopt the "store finest, derive coarser via `cellToParent()`" rule for `benches.h3_index` from day one. Write only res 8 at MVP. Defer the res-8 → res-9 graduation algorithm post-MVP.

**Rationale:** The density research (95–107/km² in historic centres vs ~5/km² in nature) makes it fairly clear that a single resolution will eventually be wrong at one end. But *when* a cell should subdivide is a genuinely open question that needs real usage data to answer well. The cheap part — making the schema able to express multiple resolutions — costs almost nothing now and avoids a migration later. The expensive part is deferred.

This is a deliberate instance of the project's "explicit deferral" pattern: named and written down rather than silently dropped.

---

## D5 — Cold start is synchronous and blocking

**Decision:** The first request in an unpolled cell blocks while Overpass is queried, and returns real results inline. No "pending" state for the triggering user.

**Alternatives considered:**
- Return an empty/partial result immediately with a "we're fetching, check back" state, and backfill asynchronously.
- Return cached-adjacent data with a staleness marker.

**Rationale:** "Where can I sit?" is a question with a short patience window. Returning an empty list to the very first user in an area is a bad first impression, and a "check back later" state in a utility app this simple is worse than a two-second wait. Blocking keeps the mental model simple: you ask, you get benches.

**Assumption made explicit:** Overpass latency for a bounded bbox query is ms-scale. This is written into the cold-start design as a falsifiable assumption. If real latency proves to be seconds, the design flips to async + the real-time push channel — which is one of the reasons that design exists at all.

---

## D6 — Cascade pre-warming at k=2..3, non-blocking

**Decision:** After a cell ingests, dispatch background pre-warm jobs for its k=2 to k=3 H3 ring neighbours. Never block the triggering request on these.

**Rationale:** Turns the cold-start penalty from a per-cell cost into a roughly one-time cost per *region visited*. A user who walks or scrolls outward from where they first searched should be moving into already-warm cells. Roughly 900m–1400m out at res 8, sized on walking distance.

**Known issue:** The original justification referenced the 750m/1500m radius ladder from the original nearby-search scope, which was subsequently superseded by KNN. The k value is still defensible on walking-distance grounds, but the stated rationale now points at a design that no longer exists.

---

## D7 — In-process async dispatch; real message queue deferred

**Decision:** Background ingestion jobs run in a goroutine-based worker pool inside the API process. No NATS, SQS, or Redis-backed queue at MVP.

**Rationale:** At MVP scale (one small deployment, one region, no meaningful concurrency), a broker adds an entire piece of infrastructure to run, monitor, and pay for, in exchange for guarantees nothing currently needs.

**Explicit revisit trigger:** The trigger is reliability, not throughput. Specifically: needing pre-warm jobs to survive a process restart, or needing retry-with-backoff and dead-lettering. Throughput alone will not force this change for a long time.

**Learning note:** Building a bounded worker pool from goroutines and channels directly, rather than pulling in a job library, is deliberate — it's a good vehicle for Go's concurrency primitives.

---

## D8 — Idempotent upserts keyed on OSM identity

**Decision:** All writes are upserts deduped on `(source, source_id)` — i.e. `('osm', <node/way id>)`. Writes are chunked and transactional.

**Rationale:** Re-polling a cell must always be safe. This is the single property that makes everything else in the pipeline tolerable to get wrong: cascade overlap, retries, a manual re-run, a partial failure mid-ingest. Without it, every one of those becomes a duplicate-data bug. The deployment-shape decisions independently flag this as the highest-priority property of the ingestion job, above scheduling sophistication.

---

## D9 — `benches.h3_index` is for reconciliation, not search

**Decision:** The `h3_index` column on `benches` exists so ingestion can diff OSM's current view of a cell against stored rows. Spatial search does not use it.

**Rationale:** There was a real temptation to use the H3 index as a search accelerator — "find the cell, return its benches" feels efficient. It isn't: it reintroduces the cell-boundary problem (a bench 5m away but across a cell edge gets missed) that PostGIS's GIST index solves properly. Keeping the column's purpose narrow prevents that mistake from creeping in later.

---

## D10 — OpenBenches rejected as a primary data source

**Decision:** OSM `amenity=bench` is the sole primary source. OpenBenches is not used at MVP.

**Rationale:** OpenBenches covers memorial benches specifically — a niche subset that would give badly skewed coverage for a "where can I sit" utility. It's also a small hobby project, which makes it a fragile production dependency.

**Repositioned, not discarded:** Retained as a possible future *enrichment* layer, adding dedication text or history to benches already sourced from OSM.

---

## D11 — Licensing position

**Decision:** OSM data is ODbL. Attribution is required and will live on a settings/about screen. Share-alike obligations attach to bulk database redistribution, which the MVP does not do.

**Rationale:** Serving query results from an OSM-derived database is not redistribution of the database itself. The obligation that does bind is attribution, and that's cheap to satisfy properly.

---

## D12 — Overpass and Geofabrik serve different jobs, stated explicitly

**Decision:** This pipeline's Overpass usage is scoped to bounded, per-cell demand fills only. Any bulk backfill, country-wide seed, or scheduled refresh uses Geofabrik static extracts, run by the separate batch job in the deployment-shape design. Neither source substitutes for the other's job.

**Alternatives considered:**

| Option | Why rejected |
|---|---|
| Drop Overpass, use only Geofabrik | Geofabrik extracts are static snapshots; they can't answer "what's near this exact point right now" without a bulk import first — the thing D1 rejected. |
| Drop Geofabrik, use only Overpass | Leaves the deployment-shape design's objection to Overpass as a hard production dependency unaddressed for any future bulk refresh. |

**Rationale:** The two designs each had half of a true answer and neither wrote down the other half. Overpass's bounded-bbox, per-request shape is exactly what this pipeline needs and exactly what the deployment-shape design's objection doesn't cover — that objection is about depending on Overpass for coverage at bulk scale, not about one small query per cold cell. Geofabrik remains correct for the job it was proposed for (bulk backfill), which this pipeline was never trying to do.

**Status:** Settled. Also recorded in the deployment-shape decision log as D9.

---

## D13 — Cascade fan-out capped at depth 1

**Decision:** A cell warmed via cascade pre-warm never itself triggers a further cascade. Only a cold-start cell (reached by a direct user query) originates one.

**Rationale:** Without a cap, cascade pre-warming has no stated bound — a single query could in principle warm a ring whose warming warms further rings, with no stopping condition short of running out of unpolled cells. A depth-1 cap keeps the cost of any single user query bounded to at most one ring of neighbours, while still delivering the "walk outward into warm cells" benefit the design is going for.

**Trade-off accepted:** A user who walks several cells outward from their original query may still hit an occasional cold cell beyond the pre-warmed ring, rather than a fully pre-warmed corridor. Preferred over the alternative, which trades a rare cold cell for an unbounded worst case.

**Status:** Settled.

---

## D14 — Freshness window set to 30 days

**Decision:** `polled_cells.polled_at` is considered fresh for 30 days.

**Rationale:** OSM bench data changes slowly, so this is a conservative, tunable default rather than a derived number. Chosen for simplicity of reasoning about staleness during MVP; nothing else in the design depends on the specific value.

**Status:** Settled — a default flagged for tuning against real usage, not a derived value.

---

## D15 — Cold-start failure returns empty, does not mark the cell polled

**Decision:** If the synchronous Overpass fetch fails or times out during cold start, the request returns `200` with an empty `benches` array. The cell is **not** marked polled.

**Alternatives considered:**

| Option | Why rejected |
|---|---|
| Return an error (5xx) | Breaks the "no-results is 200" contract already established in the nearby-search design for a case that's operationally identical from the client's point of view — a search that comes back with nothing. |
| Mark the cell polled with a short TTL, retry later | Adds state (a "polled but low-confidence" flag) for a problem D16's timeout already bounds tightly; not worth the schema complexity at MVP. |

**Rationale:** Treating an Overpass failure as "no results" keeps the client's error-handling surface small and consistent with the existing empty-result contract. Not marking the cell polled is the part that matters most: caching a failure as a fresh, empty cell would silently hide real benches from every subsequent query in that area until the freshness window (D14) expires.

**Status:** Settled.

---

## D16 — Explicit 5s timeout on the Overpass HTTP call

**Decision:** The Overpass fetch during cold start carries its own 5s client-side timeout, independent of the nearby-search design's 3s PostGIS query deadline.

**Rationale:** The ms-scale Overpass latency assumption (D5) was written down as deliberately falsifiable, but nothing actually caught the falsification — a hung Overpass call had no stated bound. This closes that gap. It also means worst-case cold-start latency is the sum of both timeouts (~8s), not the 3s a reader of the nearby-search design alone might assume; that composed number wasn't previously stated anywhere.

**Status:** Settled.

---

## Unresolved at time of writing

None outstanding — the four items below from the prior revision are resolved above.

~~Freshness window duration for `polled_cells.polled_at`.~~ → D14
~~Reconciling this pipeline's Overpass-centric model with the deployment-shape design's stated preference for Geofabrik extracts.~~ → D12
~~Cascade depth cap — whether a cascade-warmed cell may itself cascade.~~ → D13
~~Cold-start failure behaviour when Overpass is unavailable.~~ → D15, D16
