# Demand-Driven Ingestion Pipeline — Final Design

**Covers:** H3 grid & freshness tracking, cold-start synchronous ingestion, cascade pre-warming, dispatch mechanism
**Status:** Design settled for MVP, not implemented
**Updated:** 2026-09-08

---

## 1. Purpose

Populate the `benches` table from OpenStreetMap `amenity=bench` data, driven by real user demand rather than a bulk upfront import of an entire country.

The serving layer is always PostGIS. Overpass is an *ingestion-time* dependency only — a user's search never proxies through to Overpass as its data source. The pipeline's job is to make sure that by the time a search runs, PostGIS already knows about the benches in that area, or to fill that gap in-band the first time someone asks.

---

## 2. Guiding constraints

- **No bulk seed.** Ingest only areas someone has actually looked at, plus a small predictive halo around them.
- **PostGIS is the single source of truth for reads.** Overpass is never on the read path for a warm cell.
- **Overpass is scoped to per-cell demand fills only, never bulk.** Any bulk backfill or country-wide refresh uses Geofabrik static extracts via the separate batch job in the deployment-shape design. This pipeline never bulk-imports from Overpass, and the batch job never serves a live per-cell request — see decision D12.
- **Idempotency is non-negotiable.** Every write is an upsert keyed on OSM identity, so re-polling a cell is always safe.
- **Density varies ~20x across the target geography.** Any fixed spatial constant has to survive that spread.

### Density reference (Dutch context, from prior research)

| Area type | Benches / km² |
|---|---|
| Dense historic centre | 95–107 |
| Residential | ~49 |
| Car-oriented city | ~28 |
| Parks | ~20 |
| Remote nature | ~5 |

This table is the empirical basis for the grid resolution choice, the cascade ring size, and (in the nearby-search design) the decision to use KNN rather than a fixed radius.

---

## 3. End-to-end flow

```
Client GET /benches/nearby (lat, lon)
        │
        ▼
  latLngToCell(lat, lon, res 8)
        │
        ▼
  Freshness check against polled_cells
        │
   ┌────┴─────────────────────┐
   │                          │
 FRESH                      STALE / UNPOLLED
   │                          │
   │                    Synchronous Overpass fetch
   │                          │
   │                    Chunked transactional upsert
   │                          │
   │                    Mark cell polled
   │                          │
   └────────────┬─────────────┘
                ▼
        PostGIS KNN query
                ▼
        Response returned to client
                │
                ▼ (after response, non-blocking)
        Cascade pre-warm k=2..3 ring
                ▼
        In-process worker pool
                ▼
        NOTIFY bench_cell_updated
```

---

## 4. Spatial grid and freshness tracking

### Grid model

**H3 hexagonal grid**, resolution 8 as the primary tracking grain.

H3 was chosen over a projected square grid, geohash, and a density-adaptive quadtree. Hexagons have uniform adjacency (all six neighbours share an edge and sit at equal centre distance), which makes ring-based cascade expansion well-defined, and H3 cells don't distort with latitude the way a naive lat/lon square grid does.

### `polled_cells` table

| Column | Notes |
|---|---|
| `h3_index` | H3 cell identifier |
| `resolution` | Currently always 8; column exists so res 9 can be added without migration |
| `polled_at` | Timestamp of last successful ingestion for this cell |

Freshness check: compute `latLngToCell(lat, lng, 8)` for the request point, look up the row, compare `polled_at` against the freshness window.

### `h3_index` on `benches`

The `benches` table also carries an `h3_index`. Important scoping note: **this column exists for ingestion-side reconciliation, not for spatial search.** It lets the ingestion job diff "benches OSM currently reports in cell X" against "benches we already have in cell X" so deletions and moves can be handled. Search queries use PostGIS geography indexes directly and never touch this column.

Storage rule: **store the finest resolution only, derive coarser resolutions on read via `cellToParent()`.** No column-per-resolution.

### Dual resolution

Res 9 support is seeded into the schema now (the `resolution` column, the store-finest rule) but nothing writes res 9 at MVP. The algorithm deciding when a res-8 cell *graduates* to res-9 tracking — presumably triggered by bench count or query volume in a dense centre — is explicitly deferred post-MVP.

### Tooling

`uber/h3-go`. This is CGo-based, which has knock-on effects for the build and deploy story in the deployment-shape design (no trivially static cross-compiled binary; the Docker image needs a toolchain at build time).

---

## 5. Cold-start ingestion

The first-ever request in an unpolled cell.

- Overpass query dispatched **synchronously**, blocking the triggering HTTP request, with a **5s client-side timeout** on the Overpass HTTP call itself — separate from and in addition to the 3s PostGIS query deadline in the nearby-search design. Worst-case cold-start request latency is therefore ~8s, not 3s; that composed budget isn't stated anywhere else.
- Results written via **chunked, transactional upsert**, deduped on `source` / `source_id` (OSM node/way ID).
- Cell marked polled — **only on a successful fetch.**
- Search runs and results are returned **inline in the same request/response cycle**. The user who triggers a cold start does not see a "pending" state.
- If Overpass returns nothing, retry with a smaller radius before concluding the area is genuinely empty.
- **On Overpass failure or timeout:** return `200` with an empty `benches` array, consistent with the no-results contract in the nearby-search design. The cell is **not** marked polled, so the next request retries the fetch instead of caching a false negative as fresh.

**Working assumption:** Overpass latency is low enough (ms-scale for a bounded bbox query) that blocking is acceptable UX. This assumption is written down deliberately so it can be falsified — if real-world latency turns out to be seconds, the design flips to the async + push path that the realtime-push design already provides. The 5s timeout above is the trip-wire that actually catches the falsification; previously nothing did.

---

## 6. Cascade pre-warming

After any cell finishes ingesting, pre-warm its neighbours so the next request nearby hits a warm cache.

- Compute `gridRing` / `gridDisk` neighbours at **k = 2 to 3** at res 8, sized on the assumption that a person looking for a bench may plausibly walk that far (roughly 900m–1400m outward).
- Dispatched as **background jobs that never block the triggering request**.
- **Fan-out is capped at depth 1: a cascade-warmed cell never itself triggers cascade.** Only a cell reached via a direct cold-start user query originates a cascade. Jobs dispatched by cascade pre-warm carry a flag the worker pool checks before deciding whether to re-dispatch — without this, a single query could in principle ripple outward with no bound.
- This is the path that makes real-time push meaningful: a client whose viewport overlaps a neighbour cell still being pre-warmed is the one that gets a push when it commits.

---

## 7. Dispatch mechanism

**MVP: in-process async.** A goroutine-based worker pool with a bounded queue inside the API process.

A real message broker (NATS, SQS, Redis-backed queue) is explicitly deferred. At MVP scale the added operational surface buys nothing. The trigger to revisit is a *reliability* requirement rather than a throughput one — specifically, needing pre-warm jobs to survive a process restart, or needing retry semantics with backoff and a dead-letter path.

---

## 8. Data model summary

```
benches
  id
  source            -- 'osm'
  source_id         -- OSM node/way ID    ← upsert key with source
  geog              -- PostGIS geography(Point)  ← GIST indexed, used by KNN
  h3_index          -- finest resolution; ingestion reconciliation only
  has_backrest
  is_covered
  is_accessible
  tags              -- raw OSM tags, internal only, not exposed via API at MVP
  ...timestamps

polled_cells
  h3_index
  resolution        -- 8 at MVP
  polled_at
```

Uniqueness constraint on `(source, source_id)` is what makes the upsert idempotent.

---

## 9. Explicitly deferred

- **H3 resolution graduation algorithm** (res 8 → res 9 for dense areas). Schema is ready; the logic is post-MVP.
- **Real message queue** for background dispatch.
- **Scheduled/automated re-polling.** Freshness is currently only re-evaluated when a user happens to query a stale cell. A background refresh sweep is post-MVP (see the deployment-shape design).
- **OpenBenches as an enrichment source.** Rejected as a primary source; may return later as a supplementary layer.

---

## 10. Open items

**Freshness window:** 30 days (D14). **Overpass/Geofabrik split:** stated explicitly in §2 above (D12). **Cascade fan-out cap:** depth 1, stated in §6 above (D13). **Cold-start failure behaviour:** §5 above (D15, D16). All prior open items are resolved — see decision log D12–D16.

None outstanding at time of writing.
