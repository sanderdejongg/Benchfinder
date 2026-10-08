# H3 Grid & Freshness Tracking — Final Design

**Issue:** [BEN-8](https://benchfinder.youtrack.cloud/issue/BEN-8)  **Status:** settled, not implemented; first build target within ingestion  **Updated:** 2026-10-08

**Parent:** `1-ingestion-pipeline`

---

## Purpose

Track which areas of the map have been ingested from OSM, at a spatial grain fine enough to bound ingestion cost per request and coarse enough that neighbouring requests share work. This is the freshness layer that `1-2-cell-ingestion` checks before deciding whether a cell needs a background fetch.

---

## Constraints

- **Density varies about 20x across the target geography.** Any fixed spatial constant has to survive that spread.

Density reference (Dutch context, from prior research):

| Area type | Benches / km² |
|---|---|
| Dense historic centre | 95–107 |
| Residential | ~49 |
| Car-oriented city | ~28 |
| Parks | ~20 |
| Remote nature | ~5 |

This table is the empirical basis for the grid resolution. It also underpins the k=2 ring in `1-2-cell-ingestion` and the KNN choice in `2-nearby-search`.

- **Uniform cell adjacency.** Ring-based expansion needs equal neighbour distances, which rules out square and lat/lon grids.
- **No migration for a denser resolution later.** The schema must be able to express a second resolution without a new migration, even though nothing writes it yet.

---

## Design

### Grid model

H3 hexagonal grid, resolution 8, as the primary tracking grain. Res 8 cells have an average area of about 0.74 km² and a centre-to-centre spacing of about 0.8 km.

Hexagons have six neighbours, all sharing an edge and all at equal centre distance. Ring and disk expansion is therefore well defined, and H3 does not distort with latitude the way a lat/lon square grid does.

### Extension

The grid is computed in Postgres by the h3-pg extension: `h3_lat_lng_to_cell` for the cell of a point, `h3_grid_disk` for rings, `h3_cell_to_parent` for coarser resolutions. Cell assignment and freshness logic run in the database, next to the data they describe (D5).

### `polled_cells` table

| Column | Notes |
|---|---|
| `h3_index` | H3 cell identifier |
| `resolution` | Always 8 at MVP. The column exists so res 9 can be added without a migration |
| `polled_at` | Timestamp of the last successful ingestion covering this cell |

**Freshness check:** compute the res-8 cell of the request point, look up its row, and compare `polled_at` with the freshness window (D4). Cold means no row. Stale means `polled_at` older than the window.

### `h3_index` on `benches`

The `benches` table carries an `h3_index`. It exists for ingestion-side reconciliation, not for spatial search (D3). It is intended to let ingestion diff "benches OSM reports in cell X" against "benches we hold in cell X", so deletions and moves can be handled. Reconciliation is not part of the ingestion job steps (`1-2-cell-ingestion` open item 3). Search uses the PostGIS geography index and never touches this column.

Storage rule: store the finest resolution only (res 8), and derive coarser resolutions on read with `h3_cell_to_parent`. There is no column per resolution.

### Dual resolution

Res 9 is seeded into the schema now (the `resolution` column and the store-finest rule). Nothing writes res 9 at MVP. The algorithm that graduates a res-8 cell to res 9 is deferred (see Explicitly deferred).

---

## Contracts & data model

```
polled_cells
  h3_index          -- H3 cell, res 8
  resolution        -- 8 at MVP
  polled_at         -- last successful ingestion covering the cell

benches
  ...
  h3_index          -- res 8; ingestion reconciliation only, not used by search
  ...
```

Freshness window: 30 days (D4).

Functions used (h3-pg, names as listed in the 2026-10-08 design discussion):

- `h3_lat_lng_to_cell(point, 8)`: cell of a request or bench point.
- `h3_grid_disk(cell, 2)`: the 19-cell k=2 disk (`1-2-cell-ingestion` D9).
- `h3_cell_to_parent(cell, 7)`: coarser grain on read, when needed. (On a res-8 cell, `h3_cell_to_parent(cell, 8)` is a no-op, so res 7 is the example.)

The `h3_postgis` extension, shipped with h3-pg, provides the geometry and geography overloads. Verify it alongside provider support (`4-deployment-shape` open item 1).

---

## Dependencies

- `1-ingestion-pipeline`: parent.
- `1-2-cell-ingestion`: consumes the freshness check and the disk function.
- `2-nearby-search`: the search path does not use `h3_index` (D3 here).
- `4-deployment-shape`: the database provider must offer h3-pg (pre-implementation check).
- `6-architecture-setup`: freshness logic is tested against PostGIS with h3-pg, not as pure functions.

---

## Explicitly deferred

- **Res 8 to res 9 graduation.** The schema is ready. The trigger (bench count, query volume, or both) needs real usage data. Post-MVP.

---

## Superseded

None.

---

## Open items

1. **h3-pg availability on the chosen provider.** Verify before provisioning. If h3-pg is unavailable on DigitalOcean Managed Postgres, switch providers (checking Neon first) and keep this design. See `4-deployment-shape`.
