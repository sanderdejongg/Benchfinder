# H3 Grid & Freshness Tracking — Final Design

**Parent:** `1-ingestion-pipeline`
**Status:** Settled for MVP, not implemented
**Updated:** 2026-09-08

---

## Purpose

Track which areas of the map have already been ingested from OSM, at a spatial grain fine enough to bound ingestion cost per query but coarse enough that neighbouring queries share a cache. This is the freshness layer the rest of the ingestion pipeline checks before deciding whether a query needs a live Overpass fetch (see `1-2-cold-start-ingestion`).

---

## Constraints

- **Density varies ~20x across the target geography.** Any fixed spatial constant has to survive that spread.

### Density reference (Dutch context, from prior research)

| Area type | Benches / km² |
|---|---|
| Dense historic centre | 95–107 |
| Residential | ~49 |
| Car-oriented city | ~28 |
| Parks | ~20 |
| Remote nature | ~5 |

This table is the empirical basis for the grid resolution choice. It also underpins the cascade ring size in `1-3-cascade-prewarm` and, in `2-nearby-search`, the decision to use KNN rather than a fixed radius.

- **Cell adjacency must be well-defined for ring-based expansion.** The cascade pre-warm design walks outward by "ring," which requires uniform neighbour distance — this rules out naive square/lat-lon grids.
- **No migration for a future denser resolution.** The schema has to be able to express a second resolution without a later migration, even though nothing writes it yet.

---

## Design

### Grid model

**H3 hexagonal grid**, resolution 8 as the primary tracking grain.

H3 was chosen over a projected square grid, geohash, and a density-adaptive quadtree. Hexagons have uniform adjacency (all six neighbours share an edge and sit at equal centre distance), which makes ring-based cascade expansion well-defined, and H3 cells don't distort with latitude the way a naive lat/lon square grid does.

### `polled_cells` table

| Column | Notes |
|---|---|
| `h3_index` | H3 cell identifier |
| `resolution` | Currently always 8; column exists so res 9 can be added without migration |
| `polled_at` | Timestamp of last successful ingestion for this cell |

**Freshness check:** compute `latLngToCell(lat, lng, 8)` for the request point, look up the row, compare `polled_at` against the freshness window (D4 below).

### `h3_index` on `benches`

The `benches` table also carries an `h3_index`. Important scoping note: **this column exists for ingestion-side reconciliation, not for spatial search.** It lets the ingestion job diff "benches OSM currently reports in cell X" against "benches we already have in cell X" so deletions and moves can be handled. Search queries use PostGIS geography indexes directly and never touch this column — see `2-nearby-search`.

Storage rule: **store the finest resolution only, derive coarser resolutions on read via `cellToParent()`.** No column-per-resolution.

### Dual resolution

Res 9 support is seeded into the schema now (the `resolution` column, the store-finest rule) but nothing writes res 9 at MVP. The algorithm deciding when a res-8 cell *graduates* to res-9 tracking — presumably triggered by bench count or query volume in a dense centre — is explicitly deferred post-MVP.

### Tooling

`uber/h3-go`. This is CGo-based, which has knock-on effects for the build and deploy story in `4-deployment-shape` (no trivially static cross-compiled binary; the Docker image needs a toolchain at build time).

---

## Contracts & data model

```
polled_cells
  h3_index
  resolution        -- 8 at MVP
  polled_at

benches
  ...
  h3_index          -- finest resolution; ingestion reconciliation only, not used by search
  ...
```

---

## Dependencies

- `1-ingestion-pipeline` — parent: end-to-end flow, guiding constraints, `benches` core schema.
- `1-2-cold-start-ingestion` — consumes the freshness check to decide whether a cell needs a live fetch.
- `1-3-cascade-prewarm` — consumes `gridRing`/`gridDisk` adjacency.
- `2-nearby-search` — explicit boundary: `h3_index` is *not* used for spatial search; PostGIS GIST index is.
- `4-deployment-shape` — CGo build implication of `h3-go`.

---

## Explicitly deferred

- **H3 resolution graduation algorithm** (res 8 → res 9 for dense areas). Schema is ready; the trigger logic (bench count? query volume?) needs real usage data and is post-MVP.

---

## Superseded

None.

---

## Open items

None outstanding at time of writing.
