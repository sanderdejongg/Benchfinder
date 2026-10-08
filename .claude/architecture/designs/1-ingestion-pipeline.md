# Demand-Driven Ingestion Pipeline — Final Design

**Issue:** [BEN-1](https://benchfinder.youtrack.cloud/issue/BEN-1)  **Status:** settled, not implemented  **Updated:** 2026-10-08

**Sub-designs:** `1-1-h3-grid-freshness` (grid and freshness), `1-2-cell-ingestion` (cold and stale cell ingestion), `1-4-dispatch-mechanism` (background dispatch).

---

## Purpose

Populate the `benches` table from OpenStreetMap `amenity=bench` data, driven by real user demand rather than a bulk import of a whole country.

The serving layer is always PostGIS. Overpass is an ingestion-time dependency only. A search never reads from Overpass. The pipeline makes sure that PostGIS knows about the benches around a request, either already or by filling the gap in the background after the request is answered.

---

## Constraints

- **No bulk seed.** Ingest only areas someone has looked at, plus a small halo around them.
- **PostGIS is the only read source.** Overpass is never on the read path, warm or cold.
- **Overpass is scoped to per-cell demand fills.** Bulk backfill and country-wide refresh use Geofabrik static extracts through a separate batch command (`4-deployment-shape`).
- **Idempotency is non-negotiable.** Every write is an upsert on OSM identity, so re-ingesting a cell is always safe (D3).
- **Density varies about 20x** across the target geography. Any fixed spatial constant must survive that spread (`1-1-h3-grid-freshness`).

---

## Design

### End-to-end flow

```
Client GET /benches/nearby (lat, lon)                       [2-nearby-search]
        │
        ├─ h3_lat_lng_to_cell(lat, lon, res 8)                    [1-1-h3-grid-freshness]
        ├─ freshness check against polled_cells
        ├─ KNN query over PostGIS, answered now             [2-nearby-search]
        │
        ├─ FRESH  → respond with pending_cells = []
        └─ COLD / STALE → respond with pending_cells = [cell]   (provisional, see 2-nearby-search open item 3)
                          dispatch one ingestion message     [1-4-dispatch-mechanism]

Worker: ingestion job for the cell                           [1-2-cell-ingestion]
        ├─ one Overpass request, bbox around the k=2 disk (19 cells, ~2 km)
        ├─ on failure or timeout: stop. Cells stay unpolled. No push.
        ├─ assign each bench to its res-8 cell (h3-pg)
        ├─ chunked, transactional, idempotent upsert on (source, source_id)
        ├─ mark the 19 disk cells polled
        └─ publish one Mercure update covering those 19 cells  [5-realtime-push]
```

The client re-calls the endpoint when the push arrives, or falls back to pull-to-refresh after 15 s (`5-realtime-push`).

### Ownership

| Concern | Owned by |
|---|---|
| Grid, res-8 cell, freshness window, `polled_cells` | `1-1-h3-grid-freshness` |
| Request path response, pending signal, job contents, failure behaviour | `1-2-cell-ingestion` |
| Queue, worker, retries | `1-4-dispatch-mechanism` |
| Push to the client | `5-realtime-push` |
| Bulk Geofabrik batch, hosting, worker process | `4-deployment-shape` |

---

## Contracts & data model

```
benches
  id
  source            -- 'osm'
  source_id         -- OSM node or way ID         ← upsert key with source
  geog              -- PostGIS geography(Point)   ← GIST indexed, used by KNN
  h3_index          -- res 8; ingestion reconciliation only (1-1-h3-grid-freshness)
  has_backrest
  is_covered
  is_accessible
  tags              -- raw OSM tags, internal only, not exposed via API at MVP
  ...timestamps
```

Uniqueness on `(source, source_id)` makes the upsert idempotent (D3). The `polled_cells` table is defined in `1-1-h3-grid-freshness`.

---

## Dependencies

- `1-1-h3-grid-freshness`: grid, freshness check, `polled_cells`.
- `1-2-cell-ingestion`: the job itself and the request-path pending signal.
- `1-4-dispatch-mechanism`: the queue the job travels through.
- `2-nearby-search`: the trigger point. Every ingestion starts from a request.
- `4-deployment-shape`: the Geofabrik batch command, the hosting of the worker, and the Overpass/Geofabrik split.
- `5-realtime-push`: the push that follows a completed job.

---

## Explicitly deferred

- **Scheduled or automated re-polling.** Freshness is re-evaluated only when a user queries a stale cell. A background refresh sweep is post-MVP. Trigger and mechanism: `4-deployment-shape` D12 and D16.
- **OpenBenches as an enrichment source.** Rejected as a primary source (D4). May return later as a supplementary layer.

---

## Superseded

- The cascade pre-warm sub-design is folded into `1-2-cell-ingestion` (single job, k=2 disk). Its decisions are recorded there as superseded entries (`1-2-cell-ingestion` D4, D5).
- The in-process dispatch model is superseded by the messenger queue (`1-4-dispatch-mechanism` D1 superseded by D2).

---

## Open items

None outstanding at pipeline level. Sub-design open items are listed in each sub-design.
