# Demand-Driven Ingestion Pipeline — Overview

**Split into four focused sub-designs:** `1-1-h3-grid-freshness`, `1-2-cold-start-ingestion`, `1-3-cascade-prewarm`, `1-4-dispatch-mechanism`
**Status:** Pipeline-wide purpose, constraints, flow and cross-cutting decisions settled; each sub-design individually settled for MVP; nothing implemented
**Updated:** 2026-09-08

---

## 1. Purpose

Populate the `benches` table from OpenStreetMap `amenity=bench` data, driven by real user demand rather than a bulk upfront import of an entire country.

The serving layer is always PostGIS. Overpass is an *ingestion-time* dependency only — a user's search never proxies through to Overpass as its data source. The pipeline's job is to make sure that by the time a search runs, PostGIS already knows about the benches in that area, or to fill that gap in-band the first time someone asks.

---

## 2. Constraints (pipeline-wide)

- **No bulk seed.** Ingest only areas someone has actually looked at, plus a small predictive halo around them.
- **PostGIS is the single source of truth for reads.** Overpass is never on the read path for a warm cell.
- **Overpass is scoped to per-cell demand fills only, never bulk.** Any bulk backfill or country-wide refresh uses Geofabrik static extracts via the separate batch job in `4-deployment-shape`. This pipeline never bulk-imports from Overpass, and the batch job never serves a live per-cell request — see D12 below.
- **Idempotency is non-negotiable.** Every write is an upsert keyed on OSM identity, so re-polling a cell is always safe — see D8 below.
- **Density varies ~20x across the target geography.** Any fixed spatial constant has to survive that spread; see the density table in `1-1-h3-grid-freshness`.

---

## 3. End-to-end flow

```
Client GET /benches/nearby (lat, lon)
        │
        ▼
  latLngToCell(lat, lon, res 8)                    ─┐
        │                                            │
        ▼                                            │
  Freshness check against polled_cells               │  1-1-h3-grid-freshness
        │                                            │
   ┌────┴─────────────────────┐                     ─┘
   │                          │
 FRESH                      STALE / UNPOLLED
   │                          │
   │                    Synchronous Overpass fetch   ─┐
   │                          │                        │
   │                    Chunked transactional upsert   │  1-2-cold-start-ingestion
   │                          │                        │
   │                    Mark cell polled              ─┘
   │                          │
   └────────────┬─────────────┘
                ▼
        PostGIS KNN query          (2-nearby-search)
                ▼
        Response returned to client
                │
                ▼ (after response, non-blocking)
        Cascade pre-warm k=2..3 ring                 ─┐  1-3-cascade-prewarm
                ▼                                      │
        In-process worker pool                        ─┘  1-4-dispatch-mechanism
                ▼
        NOTIFY bench_cell_updated    (5-realtime-push)
```

---

## 4. Contracts & data model

```
benches
  id
  source            -- 'osm'
  source_id         -- OSM node/way ID    ← upsert key with source
  geog              -- PostGIS geography(Point)  ← GIST indexed, used by KNN
  h3_index          -- finest resolution; ingestion reconciliation only — see 1-1-h3-grid-freshness
  has_backrest
  is_covered
  is_accessible
  tags              -- raw OSM tags, internal only, not exposed via API at MVP
  ...timestamps
```

Uniqueness constraint on `(source, source_id)` is what makes the upsert idempotent (D8). `polled_cells` schema lives in `1-1-h3-grid-freshness`.

---

## 5. Dependencies

- `1-1-h3-grid-freshness`, `1-2-cold-start-ingestion`, `1-3-cascade-prewarm`, `1-4-dispatch-mechanism` — the four sub-designs this overview points into.
- `2-nearby-search` — the trigger point for the whole pipeline.
- `4-deployment-shape` — batch job (Geofabrik) vs. this pipeline's live Overpass usage; process/build implications.
- `5-realtime-push` — consumes cascade pre-warm completion events.

---

## 6. Explicitly deferred

- **Scheduled/automated re-polling.** Freshness is currently only re-evaluated when a user happens to query a stale cell. A background refresh sweep is post-MVP (see `4-deployment-shape`). This is pipeline-wide, not specific to one phase.
- **OpenBenches as an enrichment source.** Rejected as a primary source (D10); may return later as a supplementary layer.

Phase-specific deferrals (H3 resolution graduation; real message queue) now live in `1-1-h3-grid-freshness` and `1-4-dispatch-mechanism` respectively.

---

## 7. Superseded

The original combined design doc's H3-grid, cold-start, cascade-prewarm, and dispatch-mechanism sections were split out into the four sub-designs listed above. This file retains only pipeline-wide purpose, constraints, flow, data model, and cross-cutting decisions (D1, D2, D8, D10, D11, D12 in the decision log). Component-specific decisions were relocated to each sub-design's own decision log — see the decision log for pointers.

---

## 8. Open items

None outstanding at time of writing.
