# H3 Grid & Freshness Tracking — Decision Log

Decisions leading to `../designs/1-1-h3-grid-freshness.md`.

**Provenance note:** this log was split out of the pipeline-wide `1-ingestion-pipeline` decision log. The entries below carry fresh, locally-scoped numbering (D1–D4); their prior identities in that log were D3, D4, D9, and D14 respectively — see that log for pointers.

---

## D1 — H3 hex grid over square grid, geohash, and quadtree

**Decision:** H3, resolution 8, as the spatial tracking grain.

**Alternatives considered:**

| Option | Why rejected |
|---|---|
| Fixed square grid in a projected CRS | Cells distort with latitude unless you commit to a projection; adjacency is lumpy — diagonal neighbours are 1.41x further than orthogonal ones, which makes "expand outward by one ring" ill-defined. |
| Geohash | Same square-cell adjacency problems, plus the well-known edge artefact where nearby points can have very different prefixes. |
| Density-adaptive quadtree | Genuinely the most "correct" answer for a 20x density spread, but the implementation and reconciliation complexity is large, and it front-loads the hardest part of the problem. |

**Rationale:** Hexagons have uniform adjacency — six neighbours, all equidistant, all edge-sharing. That makes `gridRing`/`gridDisk` a clean primitive for the cascade in `1-3-cascade-prewarm`. H3 also handles the latitude problem natively. It sits at the right point on the complexity curve: more principled than squares, far less work than a quadtree.

**Learning value noted:** H3 is a widely used piece of industry infrastructure, so the concepts transfer.

**Status:** Settled.

---

## D2 — Dual resolution seeded into schema, graduation algorithm deferred

**Decision:** Include a `resolution` column in `polled_cells` and adopt the "store finest, derive coarser via `cellToParent()`" rule for `benches.h3_index` from day one. Write only res 8 at MVP. Defer the res-8 → res-9 graduation algorithm post-MVP.

**Rationale:** The density research (95–107/km² in historic centres vs ~5/km² in nature) makes it fairly clear that a single resolution will eventually be wrong at one end. But *when* a cell should subdivide is a genuinely open question that needs real usage data to answer well. The cheap part — making the schema able to express multiple resolutions — costs almost nothing now and avoids a migration later. The expensive part is deferred.

This is a deliberate instance of the project's "explicit deferral" pattern: named and written down rather than silently dropped.

**Status:** Settled.

---

## D3 — `benches.h3_index` is for reconciliation, not search

**Decision:** The `h3_index` column on `benches` exists so ingestion can diff OSM's current view of a cell against stored rows. Spatial search does not use it.

**Rationale:** There was a real temptation to use the H3 index as a search accelerator — "find the cell, return its benches" feels efficient. It isn't: it reintroduces the cell-boundary problem (a bench 5m away but across a cell edge gets missed) that PostGIS's GIST index solves properly. Keeping the column's purpose narrow prevents that mistake from creeping in later.

**Status:** Settled.

---

## D4 — Freshness window set to 30 days

**Decision:** `polled_cells.polled_at` is considered fresh for 30 days.

**Rationale:** OSM bench data changes slowly, so this is a conservative, tunable default rather than a derived number. Chosen for simplicity of reasoning about staleness during MVP; nothing else in the design depends on the specific value.

**Status:** Settled — a default flagged for tuning against real usage, not a derived value.

---

## Unresolved at time of writing

None.
