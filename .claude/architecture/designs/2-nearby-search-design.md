# BEN-2 — `GET /benches/nearby`: Final Design

**Epic:** [BEN-2](https://benchfinder.youtrack.cloud/issue/BEN-2)
**Status:** Design settled for MVP, two conventions still open

---

## 1. Purpose

The single read endpoint the Flutter client depends on. Given a coordinate, return the benches a person could plausibly walk to, ordered nearest first.

---

## 2. The core problem

Bench density varies roughly 20x across the target geography — from ~100/km² in a dense historic centre down to ~5/km² in remote nature. A fixed-radius query is wrong at both ends of that spread: a 500m radius returns 80 benches in Amsterdam city centre and zero in a rural area; a 3000m radius returns a useless flood in the city.

The endpoint has to self-adapt to local density.

---

## 3. Resolved approach: KNN with a hard cutoff

PostGIS's `<->` distance operator with `ORDER BY ... LIMIT`, combined with an `ST_DWithin` bound.

```sql
SELECT id, lat, lon, has_backrest, is_covered, is_accessible,
       ST_Distance(geog, :point) AS distance_meters
FROM benches
WHERE ST_DWithin(geog, :point, :cutoff)
ORDER BY geog <-> :point
LIMIT :n
```

**Why this shape works:** `<->` uses the GIST index to walk outward from the query point, returning the N nearest rows regardless of how tightly or sparsely they're packed. Density adaptation falls out of the query structure itself — no ladder, no retry loop, no client-side radius negotiation, one round trip.

**Why the `ST_DWithin` bound is still needed:** Without it, a query in an empty region forces the index walk to keep expanding until it finds an Nth-nearest bench, potentially degenerating into something close to a full scan to surface a bench 40km away that nobody wants anyway. The cutoff bounds the work *and* encodes the product judgement that a bench 6km away is not "nearby."

---

## 4. Constants

| Constant | Value | Notes |
|---|---|---|
| `LIMIT` | 15–20 | Server-side constant, **not** a client-exposed parameter at MVP |
| Distance cutoff | 5000m | Hard bound via `ST_DWithin` |
| Query timeout | 3s | `context.WithTimeout` |

Both spatial constants are reasonable defaults chosen for tuning later against real usage, not derived values.

---

## 5. Request contract

```
GET /benches/nearby?lat=<float>&lon=<float>
```

**Validation** (runs before the query is issued):
- `lat` ∈ [-90, 90]
- `lon` ∈ [-180, 180]
- Missing or non-numeric → 400

**Headers:** `X-Visitor-Id` per BEN-3. Optional — a missing or malformed value never causes rejection.

---

## 6. Response contract

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
  ]
}
```

- `distance_meters` computed per row via `ST_Distance`.
- Raw OSM tags are **excluded** — internal and ingestion-only at MVP.
- **Zero results is a 200 with an empty array**, not a 404 and not an error. "No benches near you" is a valid, correct answer to a well-formed question.

---

## 7. Layering

```
HTTP handler          parse, validate, serialise, set status
      ↓
Search service        query orchestration, applies constants
      ↓
Repository (interface) KNN query execution
      ↓
PostGIS
```

The repository is interface-based so the service layer is testable without a database. The handler holds no query knowledge; the repository holds no HTTP knowledge.

**Router:** `chi` — a thin, idiomatic layer over `net/http` that composes cleanly with the visitor-ID middleware from BEN-3/BEN-6, without dragging in a framework's own conventions.

---

## 8. Interaction with the ingestion pipeline

This endpoint is the trigger point for BEN-1. Before the KNN query runs, the handler path performs the H3 freshness check (BEN-8). A cold cell means a synchronous Overpass fetch (BEN-9) completes first, so the KNN query always runs against a populated cell. Cascade pre-warming (BEN-10) fires after the response is sent.

---

## 9. Explicitly deferred

**Deterministic tiebreak.** Adding `, id` as a secondary sort key: `ORDER BY geog <-> :point, id`.

Without it, when rows are tied on exact distance at the `LIMIT` boundary, it isn't only the *ordering* that can vary between identical repeated queries — the returned *set* can differ, since which of the tied rows makes the cut is unspecified. Real-world impact is very low (exact float distance ties between distinct benches are rare), and it's cheap to add later because `id` is already indexed. Tracked as a follow-up, not MVP-blocking.

---

## 10. Superseded from original scope

These were part of the epic before KNN was adopted, and are recorded so the reasoning isn't lost:

- **Adaptive radius ladder** (300m → 750m → 1500m → 2000m, expanding until enough results). Replaced by KNN + fixed cutoff.
- **Minimum-results-before-stopping threshold.** No longer meaningful — KNN always returns up to `LIMIT` if the rows exist within the cutoff.
- **"Ladder vs. single max-radius query" open question.** Resolved by KNN, which was the deeper fix for what both options were trying to work around.

---

## 11. Still open

1. **Error response body shape** for validation failures — e.g. `{ "error": "..." }` vs. something richer with a machine-readable code. Worth settling deliberately since it becomes the convention every future endpoint inherits.
2. **Response envelope beyond `benches`** — whether to include a `count` field. Undecided, not blocking. (Arguably redundant with array length, but useful if pagination or a truncation flag ever appears.)
3. **Final `LIMIT` and cutoff values** — defaults chosen, may tune after real usage.
