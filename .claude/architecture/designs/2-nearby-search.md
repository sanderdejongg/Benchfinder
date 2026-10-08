# `GET /benches/nearby` — Final Design

**Issue:** [BEN-2](https://benchfinder.youtrack.cloud/issue/BEN-2)  **Status:** settled, not implemented  **Updated:** 2026-10-08

---

## Purpose

The single read endpoint the Flutter client depends on. Given a coordinate, return the benches a person could plausibly walk to, nearest first. The response always comes from PostGIS. The endpoint never calls Overpass. When the area around the request is cold or stale, the response says so (`pending_cells`) and ingestion runs in the background (`1-2-cell-ingestion`).

---

## Constraints

- **Density varies about 20x** across the target geography: roughly 100 benches/km² in a dense historic centre, about 5/km² in remote nature. A fixed radius is wrong at both ends.
- **Bounded work.** A KNN walk with no distance cutoff can degrade toward a scan when the query point is in an empty region. The cutoff is 5000 m (D2, D3).
- **Request budget.** Each database statement is capped at 3 s by `statement_timeout`; the request has no overall deadline. No Overpass call is on the request path; a RabbitMQ publish is (`1-4-dispatch-mechanism`).
- **Known tension, recorded not resolved.** The 5000 m cutoff reaches further than the roughly 2 km warm footprint that ingestion fills (`1-2-cell-ingestion` D9). Benches in cold cells beyond that footprint stay invisible until someone queries there. This is acceptable at rural density: about 5 benches/km² gives about 70 benches within the 19-cell footprint (about 14 km²). The ~2 km figure is measured from the request cell's centre; from an arbitrary point inside that cell the guaranteed covered radius is about 1.4 km (`1-2-cell-ingestion` D9).
- **Build order.** This endpoint is built first, against seeded fixtures, with no freshness hook (D17).

---

## Design

### Request flow

1. Validate `lat` and `lon`. Failure returns `400` with `invalid_coordinates` (D6, D11).
2. Read the optional `X-Visitor-Id` header and upsert the visitor row (`3-visitor-identity` D6, D11). A missing or malformed ID never rejects the request.
3. Run the KNN query (below), each statement capped at 3 s, with `LIMIT` 20 and the 5000 m cutoff.
4. Check freshness for the request point's res-8 cell (`1-1-h3-grid-freshness`). If the cell is cold or stale, add it to `pending_cells` (provisional, see open item 3) and dispatch one ingestion job (`1-2-cell-ingestion` D7–D8, `1-4-dispatch-mechanism` D2).
5. Respond `200`. Stale or cold cells are still answered from whatever PostGIS holds for them. When `pending_cells` is non-empty, the response also carries the Mercure discovery `Link` header (`5-realtime-push`). The Link header scope is provisional (open item 2).

### Layering

```
Controller           parse, validate, serialise, set status
     ↓
Search service       query orchestration, freshness check, applies constants
     ↓
Repository interface KNN query contract
     ↓
DBAL implementation  native SQL, geography types via jsor/doctrine-postgis
     ↓
PostGIS
```

The controller holds no query knowledge. The repository holds no HTTP knowledge. The service is unit-tested against an in-memory fake of the repository (D7). The SQL itself is covered by integration tests against real PostGIS (`6-architecture-setup`).

### Client states

The endpoint's `pending_cells` contract requires the client to have an explicit "empty now, may arrive shortly" state, distinct from a loading spinner. See `7-flutter-client` D6.

---

## Contracts & data model

### Constants

| Constant | Value | Notes |
|---|---|---|
| `LIMIT` | 20 | Server-side constant, not client-exposed at MVP (D3) |
| Distance cutoff | 5000 m | Hard bound via `ST_DWithin` (D2, D3) |
| DB query deadline | 3 s | Enforced by Postgres `statement_timeout`; database query only (D8) |
| Freshness window | 30 days | Owned by `1-1-h3-grid-freshness` |
| Result grain | H3 res 8 | Owned by `1-1-h3-grid-freshness` |

Both spatial constants are defaults chosen for tuning against real usage, not derived values. Final tuning is a named post-launch task (D13).

### Query shape

```sql
SELECT id, lat, lon, has_backrest, is_covered, is_accessible,
       ST_Distance(geog, :point) AS distance_meters
FROM benches
WHERE ST_DWithin(geog, :point, :cutoff)
ORDER BY geog <-> :point
LIMIT :n
```

`<->` walks the GIST index outward from the query point, so the result is the N nearest rows within the cutoff, in one round trip. Density adaptation falls out of the query itself: no ladder, no retry loop, no client-side radius negotiation.

### Request

```
GET /benches/nearby?lat=<float>&lon=<float>
```

- `lat` in [-90, 90]; `lon` in [-180, 180]. Missing or non-numeric returns `400`.
- Header `X-Visitor-Id`: optional. Never causes rejection.

### Error body

```json
{ "error": { "code": "invalid_coordinates", "message": "lat must be between -90 and 90" } }
```

### Response (`200`)

```json
{
  "benches": [
    {
      "id": "...",
      "lat": 53.2194,
      "lon": 6.5665,
      "has_backrest": true,
      "is_covered": false,
      "is_accessible": true,
      "distance_meters": 84.2
    }
  ],
  "pending_cells": ["<h3-res8-index>"]
}
```

- `pending_cells` is `[]` when every cell in play is fresh. Otherwise it lists the cold or stale res-8 cells (D14, D15).
- No-results is `200` with `"benches": []`, never `404`.
- `distance_meters` is computed per row by `ST_Distance`.
- Raw OSM tags are not exposed.
- No `count` field.

### Response headers (provisional: sent when `pending_cells` is non-empty)

```
Link: <hub-url>; rel="mercure"
```

The hub URL is the Mercure discovery link. Topic names are defined in `5-realtime-push`.

---

## Dependencies

- `1-1-h3-grid-freshness`: the freshness check and the res-8 cell.
- `1-2-cell-ingestion`: what a pending cell triggers.
- `1-4-dispatch-mechanism`: how the trigger is dispatched.
- `5-realtime-push`: the push that tells the client the cell is ready, and the `Link` header.
- `3-visitor-identity`: the optional header and the upsert.
- `4-deployment-shape`: the database provider, which must support h3-pg.
- `7-flutter-client`: the client states that consume this response.
- `6-architecture-setup`: test approach for the repository seam.

---

## Explicitly deferred

- **Deterministic tiebreak** (D5). Add `, id` as a secondary sort key: `ORDER BY geog <-> :point, id`. Not MVP-blocking.
- **Pagination and `count`** (D12). Added when pagination is actually built.
- **Cutoff and `LIMIT` tuning** (D13). Named post-launch task, pending real usage data.

---

## Superseded

- Adaptive radius ladder (300 m to 2000 m expansion): replaced by KNN with a cutoff (D1).
- Minimum-results-before-stopping threshold: no longer meaningful under KNN (D1).
- Synchronous ingestion inside the request before the query: replaced by the async flow in D14 (see `1-2-cell-ingestion` D1, superseded).
- Cascade pre-warm after the response: folded into the single ingestion job (`1-2-cell-ingestion` D8).

---

## Open items

1. **Failure responses.** Each statement is capped at 3 s (D8), but the status code and error code are not chosen for: a KNN statement that times out or fails; a dispatch failure (RabbitMQ unreachable, so no ingestion job is queued); and a visitor-upsert failure, which sits against the never-reject rule for the visitor ID (`3-visitor-identity` D6, open item 1).
2. **`Link` header scope.** Whether the header is sent on every `200` or only when `pending_cells` is non-empty.
3. **Scope of the freshness check.** Whether `pending_cells` lists only the request point's res-8 cell, or every cell the KNN results fall in. The plural field does not settle it. Shared with `1-2-cell-ingestion`. Under the wider reading, a client re-call can dispatch jobs for cells outside the job's disk, so fan-out is bounded only by the KNN result set. Resolve together with the re-call policy in `5-realtime-push` and `7-flutter-client`.
