# Cold-Start Ingestion — Final Design

**Parent:** `1-ingestion-pipeline`
**Status:** Settled for MVP, not implemented
**Updated:** 2026-09-08

---

## Purpose

Handle the first-ever request into an unpolled H3 cell (see `1-1-h3-grid-freshness`) so that user gets real results without requiring a bulk upfront import of the whole geography.

---

## Constraints

- "Where can I sit?" is a question with a short patience window — a "check back later" state is worse than a short wait, for an app this simple.
- **Working assumption, stated as falsifiable:** Overpass latency for a bounded bbox query is ms-scale. If real-world latency turns out to be seconds, the design should flip to the async + push path that `5-realtime-push` already provides for the cascade case.
- The composed worst-case latency budget must actually be bounded, not just assumed — see D3.

---

## Design

- Overpass query dispatched **synchronously**, blocking the triggering HTTP request, with a **5s client-side timeout** on the Overpass HTTP call itself — separate from and in addition to the 3s PostGIS query deadline in `2-nearby-search`. **Worst-case cold-start request latency is therefore ~8s, not 3s** — that composed budget isn't stated anywhere else.
- Results written via **chunked, transactional upsert**, deduped on `source` / `source_id` (OSM node/way ID) — the idempotency property is decided pipeline-wide in `1-ingestion-pipeline` D8.
- Cell marked polled in `polled_cells` — **only on a successful fetch.**
- Search runs and results are returned **inline in the same request/response cycle**. The user who triggers a cold start does not see a "pending" state.
- If Overpass returns nothing, retry with a smaller radius before concluding the area is genuinely empty.
- **On Overpass failure or timeout:** return `200` with an empty `benches` array, consistent with the no-results contract in `2-nearby-search`. The cell is **not** marked polled, so the next request retries the fetch instead of caching a false negative as fresh.

---

## Contracts & data model

No schema of its own — writes into the `benches` table (parent doc) keyed on `(source, source_id)`, and into `polled_cells` (`1-1-h3-grid-freshness`).

---

## Dependencies

- `1-ingestion-pipeline` — parent: demand-driven model (D1), PostGIS-only reads (D2), idempotent upsert (D8), Overpass/Geofabrik scoping (D12).
- `1-1-h3-grid-freshness` — the freshness check that routes a request into this path.
- `2-nearby-search` — composed timeout budget (3s PostGIS + 5s Overpass), and the `200`-with-empty-array no-results contract this design reuses for the failure case.
- `4-deployment-shape` — Overpass stays an ingestion-time dependency only, never a batch-job dependency (see that design's own Overpass/Geofabrik split, D9 there).

---

## Explicitly deferred

None specific to this sub-design. (Scheduled/automated re-polling is a pipeline-wide deferral — see `1-ingestion-pipeline`.)

---

## Superseded

None.

---

## Open items

None outstanding at time of writing.
