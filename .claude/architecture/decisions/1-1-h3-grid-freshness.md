# H3 Grid & Freshness Tracking — Decision Log

Decisions leading to `../designs/1-1-h3-grid-freshness.md`.

**Updated:** 2026-10-08

---

## D1 — H3 hex grid over square grid, geohash and quadtree

**Decision:** H3, resolution 8, as the spatial tracking grain.

**Alternatives considered:**

| Option | Why rejected |
|---|---|
| Fixed square grid in a projected CRS | Cells distort with latitude unless a projection is committed to. Adjacency is uneven: diagonal neighbours are about 1.41x further than edge neighbours, so "expand outward by one ring" is ill-defined. |
| Geohash | Same square-cell adjacency problems, plus edge artefacts where nearby points can have very different prefixes. |
| Density-adaptive quadtree | The most principled answer to a 20x density spread. Implementation and reconciliation complexity is large, and it front-loads the hardest part of the problem. |

**Rationale:** Hexagons have six equidistant, edge-sharing neighbours. That makes ring and disk expansion a clean primitive for the k=2 disk in `1-2-cell-ingestion`. H3 also handles latitude natively. It sits at a sensible point on the complexity curve: more principled than squares, much less work than a quadtree.

**Learning value noted:** H3 is widely used industry infrastructure, so the concepts transfer.

**Status:** Settled.

---

## D2 — Dual resolution seeded into the schema; graduation algorithm deferred

**Decision:** Include a `resolution` column in `polled_cells`. Store the finest resolution only in `benches.h3_index`, and derive coarser resolutions with `h3_cell_to_parent`. Write only res 8 at MVP. Defer the res-8 to res-9 graduation algorithm.

**Rationale:** Density research (95–107/km² in historic centres against about 5/km² in nature) makes it likely a single resolution will eventually be wrong at one end. When a cell should subdivide is a question that needs real usage data. The cheap part, letting the schema express several resolutions, costs almost nothing now and avoids a migration later. The expensive part is deferred and named.

**Status:** Settled.

---

## D3 — `benches.h3_index` is for reconciliation, not search

**Decision:** The `h3_index` column on `benches` exists so ingestion can diff OSM's current view of a cell against stored rows. Spatial search does not use it.

**Alternatives considered:**

| Option | Why rejected |
|---|---|
| Use the H3 index as a search accelerator: find the cell, return its benches | Reintroduces the cell-boundary problem. A bench 5 m away across a cell edge is missed. The PostGIS geography index does not have this problem. |

**Rationale:** The temptation was real because it looks efficient. Keeping the column's purpose narrow prevents that mistake from creeping in.

**Status:** Settled.

---

## D4 — Freshness window of 30 days

**Decision:** `polled_cells.polled_at` is fresh for 30 days.

**Rationale:** OSM bench data changes slowly. Thirty days is a conservative, tunable default, not a derived number. Nothing else in the design depends on the specific value.

**Status:** Settled. A default flagged for tuning against real usage.

---

## D5 — Grid computed by the h3-pg Postgres extension

**Decision:** Cell assignment, rings and parent cells are computed in Postgres by the h3-pg extension (`h3_lat_lng_to_cell`, `h3_grid_disk`, `h3_cell_to_parent`).

**Alternatives considered:**

| Option | Why rejected |
|---|---|
| PHP FFI binding to the native H3 library | Needs the native library in the application image, and a binding that this project would have to maintain itself. |
| Both: the database extension and an application-side library | Two sources of truth for the same grid. |

**Rationale:** One source of truth, in the database that holds the cells and the benches. No native library in the application image, and no self-maintained binding.

**Trade-off accepted:** freshness logic is no longer pure-function unit-testable. Its tests need PostGIS with h3-pg (`6-architecture-setup` D16).

**Pre-implementation check:** the database provider must offer h3-pg. See `4-deployment-shape` D14 and the design's Open item 1.

**Status:** Settled.

---

## Unresolved at time of writing

- h3-pg availability on the chosen provider (design Open item 1).
