# Demand-Driven Ingestion Pipeline — Final Design

**Covers:** H3 grid & freshness tracking, cold-start synchronous ingestion, cascade pre-warming, dispatch mechanism
**Status:** Design settled for MVP, not implemented

---

## 1. Purpose

Populate the `benches` table from OpenStreetMap `amenity=bench` data, driven by real user demand rather than a bulk upfront import of an entire country.

The serving layer is always PostGIS. Overpass is an *ingestion-time* dependency only — a user's search never proxies through to Overpass as its data source. The pipeline's job is to make sure that by the time a search runs, PostGIS already knows about the benches in that area, or to fill that gap in-band the first time someone asks.

---

## 2. Guiding constraints

- **No bulk seed.** Ingest only areas someone has actually looked at, plus a small predictive halo around them.
- **PostGIS is the single source of truth for reads.** Overpass is never on the read path for a warm cell.
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

- Overpass query dispatched **synchronously**, blocking the triggering HTTP request.
- Results written via **chunked, transactional upsert**, deduped on `source` / `source_id` (OSM node/way ID).
- Cell marked polled.
- Search runs and results are returned **inline in the same request/response cycle**. The user who triggers a cold start does not see a "pending" state.
- If Overpass returns nothing, retry with a smaller radius before concluding the area is genuinely empty.

**Working assumption:** Overpass latency is low enough (ms-scale for a bounded bbox query) that blocking is acceptable UX. This assumption is written down deliberately so it can be falsified — if real-world latency turns out to be seconds, the design flips to the async + push path that the realtime-push design already provides.

---

## 6. Cascade pre-warming

After any cell finishes ingesting, pre-warm its neighbours so the next request nearby hits a warm cache.

- Compute `gridRing` / `gridDisk` neighbours at **k = 2 to 3** at res 8.
- Roughly 900m–1400m outward, sized on the assumption that a person looking for a bench may well walk that far.
- Dispatched as **background jobs that never block the triggering request**.
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

1. **Freshness window value.** How old is `polled_at` allowed to be before a cell is re-polled? Not yet chosen. OSM bench data changes slowly, so this can plausibly be weeks or months, but the number is undecided.
2. **Overpass vs. Geofabrik tension with the deployment-shape design.** This design is built entirely around live Overpass queries at ingestion time. The deployment-shape design states a preference for Geofabrik static extracts over Overpass as a production dependency. These two are not currently reconciled. The likely resolution is that they serve different jobs — Overpass for demand-driven per-cell fills, Geofabrik for any future bulk refresh — but this should be stated explicitly rather than left implicit.
3. **Stale reference in the cascade pre-warming section.** Its ring sizing is justified by reference to "the existing 750m/1500m steps in the radius fallback ladder." That ladder was superseded in the nearby-search design by KNN + hard cutoff. The k=2..3 choice is still defensible on its own terms (walking distance), but the stated justification now points at something that no longer exists on the read path.
4. **Cascade fan-out limits.** k=2..3 is chosen, but there's no stated cap on cascade depth — a cell pre-warmed by cascade should presumably *not* itself trigger a further cascade, or the whole country ingests from one query. This needs to be stated as an explicit rule.
5. **Failure handling for cold start.** If Overpass is down or times out during a synchronous cold start, what does the user see? Empty results, an error, or stale-but-present data?
