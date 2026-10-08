# Cell Ingestion (Cold and Stale) — Final Design

**Issue:** [BEN-9](https://benchfinder.youtrack.cloud/issue/BEN-9)  **Status:** settled, not implemented  **Updated:** 2026-10-08

**Parent:** `1-ingestion-pipeline`

---

## Purpose

Make a cold or stale H3 cell (see `1-1-h3-grid-freshness`) fill in without blocking the request that found it. The request is answered at once from PostGIS. The cell is filled in the background, and the client is told when it is ready. This sub-design also covers the single ingestion job that fills the cell and its neighbours.

---

## Constraints

- **Request latency.** Each database statement is capped at 3 s by `statement_timeout` (`2-nearby-search` D8); the request has no overall deadline. No Overpass call is on the request path; a RabbitMQ publish is (`1-4-dispatch-mechanism`).
- **Overpass is rate-limited and unreliable.** One bounded bbox request per job, with a 5 s HTTP timeout. Fewer calls is better.
- **Density varies about 20x.** The k=2 ring size must be defensible against walking distance with correct numbers (D9).
- **Idempotent writes.** Any job can be re-run safely (`1-ingestion-pipeline` D3).
- **Known tension, recorded not resolved.** The nearby search cutoff (5000 m, `2-nearby-search` D3) reaches beyond the roughly 2 km footprint this job fills. Benches in cold cells further out are not ingested until a request lands there.

---

## Design

### Request path (cold or stale cell)

1. Compute the res-8 cell of the request point and check freshness (`1-1-h3-grid-freshness`).
2. Run the KNN query and answer from PostGIS, regardless of freshness (`2-nearby-search`).
3. If the cell is cold or stale: list it in `pending_cells` (provisional, see open item 4), dispatch one ingestion message (`1-4-dispatch-mechanism` D2), and attach the Mercure `Link` header.
4. Respond `200`.

A cold cell with no benches yet answers with an empty `benches` array and a non-empty `pending_cells`. This is the "empty now, may arrive shortly" state the client must render (`7-flutter-client` D6).

### Ingestion job (one per cold or stale cell)

1. Compute the k=2 disk of the cell: `h3_grid_disk(cell, 2)`, 19 cells, reach about 2 km.
2. Compute the bounding box enclosing the disk.
3. Issue one Overpass request for `amenity=bench` nodes and ways in that bbox. HTTP timeout 5 s. Ways are reduced to a point with `out center`, since `benches.geog` is a Point.
4. On failure or timeout: stop. No cell is marked polled. No push is published. The next request for the cell dispatches a new job.
5. On success: assign each returned bench to its res-8 cell with h3-pg.
6. Write benches in chunks, each chunk transactional, as an idempotent upsert on `(source, source_id)`.
7. Mark all 19 disk cells polled, with `polled_at` set to now.
8. Publish one Mercure update whose topics are the 19 cells (`5-realtime-push` D14).

The steps keep this order. Every bench returned by the bbox query is upserted, including benches outside the disk. Their cells are not marked polled (see Open items).

### Why the disk, not the user's cell alone

Under the single job, the disk is filled in one Overpass call. The user's own cell is one of the 19, so its push arrives when the whole disk is done, not earlier (`1-ingestion-pipeline` D7).

---

## Contracts & data model

No schema of its own. Writes land in `benches` (keyed on `(source, source_id)`) and `polled_cells` (`1-1-h3-grid-freshness`).

Overpass request shape (query shape only):

```
nwr["amenity"="bench"](<south>,<west>,<north>,<east>);
out center;
```

Constants:

| Constant | Value | Notes |
|---|---|---|
| Disk radius (k) | 2 | 19 cells, reach about 2.0 km from the request cell's centre (D9) |
| Overpass HTTP timeout | 5 s | Inside the job, not on the request path. Sized for a single small fetch; the merged bbox is about 19x larger, so 5 s is a default flagged for tuning against real Overpass latency (D3) |
| Freshness window | 30 days | `1-1-h3-grid-freshness` D4 |

---

## Dependencies

- `1-ingestion-pipeline`: parent. Demand-driven model, PostGIS-only reads, idempotent upserts, Overpass/Geofabrik split.
- `1-1-h3-grid-freshness`: freshness check, res-8 cell, disk function, `polled_cells`.
- `1-4-dispatch-mechanism`: dispatches the job and handles its retries.
- `2-nearby-search`: the request path, `pending_cells` and the query deadline.
- `5-realtime-push`: publishes the update when the job completes.
- `4-deployment-shape`: Overpass is ingestion-time only. The bulk Geofabrik batch is separate.

---

## Explicitly deferred

- Scheduled or background re-polling of stale cells outside demand. Pipeline-wide deferral (`1-ingestion-pipeline`).

---

## Superseded

- Synchronous Overpass fetch on the request path: replaced by the async flow above (D1, D7).
- Empty-array response on cold-start failure: replaced by the pending response (D2, D7). The rule that a failed cell is not marked polled stands (D2, D8).
- Cascade pre-warm of the surrounding ring after the response: folded into the single job (D4, D5, D8).
- Ring of k=2 to k=3 at about 900 to 1400 m: the figure was arithmetically wrong. Replaced by k=2 at about 2 km (D4, D9).
- Retry with a smaller radius when Overpass returns nothing: dropped (D6, D10).

---

## Open items

1. **Overpass retry policy.** The 2026-10-08 design discussion says the next request retries a failed cell. It does not say whether the messenger queue also retries a failed job automatically, or how many times. See `1-4-dispatch-mechanism`.
2. **Duplicate in-flight jobs.** Two requests for the same cold cell before the first job completes dispatch two jobs. Upserts keep the data correct, but the duplicate Overpass calls count against rate limits. No deduplication is designed. Pull-to-refresh during a slow job, and Messenger retries (`1-4-dispatch-mechanism` open item 1), also multiply Overpass calls.
3. **Reconciliation.** Deletes and moves are not part of the job steps. Cells only partly inside the bbox must be excluded from any diff.
4. **Scope of the freshness check.** Whether the pending signal covers only the request's cell, or every cell the KNN results fall in. Shared with `2-nearby-search` open item 3. Under the wider reading, a client re-call can dispatch jobs for cells outside the job's disk, so fan-out is bounded only by the KNN result set. Resolve together with the re-call policy in `5-realtime-push` and `7-flutter-client`.
5. **Subscribe-before-publish race.** A push published before the client has attached to its stream is not delivered. See `5-realtime-push` open items.
