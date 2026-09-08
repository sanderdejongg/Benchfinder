# `GET /benches/nearby` — Decision Log

Decisions leading to `../designs/2-nearby-search.md`.

---

## D1 — KNN (`<->`) over an adaptive radius ladder

**Decision:** Use PostGIS's KNN operator with `ORDER BY ... LIMIT` as the search primitive.

**The problem being solved:** Density varies ~20x across the target geography (dense historic centre ~95–107 benches/km², remote nature ~5/km²). Any single fixed radius is simultaneously too wide for cities and too narrow for the countryside.

**Alternatives considered:**

| Option | Assessment |
|---|---|
| Fixed radius | Rejected outright — fails at both ends of the density spread. |
| **Adaptive radius ladder** (300 → 750 → 1500 → 2000m, expand until N results found) | This was the original design. Works, but requires either multiple sequential queries or ladder logic that has to decide when "enough" results have been found. Each rung is a separate index traversal. |
| **KNN + hard cutoff** | Chosen. |

**Rationale:** The ladder treats density adaptation as a control-flow problem to be solved in application code. KNN makes it a property of the query itself — the GIST index walks outward from the query point and stops when it has N rows, which *is* the adaptive behaviour, achieved in one round trip with no state machine.

The key realisation was that the ladder and the "single max-radius query" option under debate were both workarounds for the same underlying constraint, and KNN removed the constraint rather than choosing between the workarounds. Three items were superseded at once: the ladder, the minimum-results threshold, and the open ladder-vs-max-radius question.

---

## D2 — Keep a hard `ST_DWithin` cutoff alongside KNN

**Decision:** Bound the KNN query with `WHERE ST_DWithin(geog, point, 5000)`.

**Rationale:** Two distinct reasons, and both matter.

*Performance:* Unbounded KNN in a sparse or empty region keeps expanding the index walk looking for an Nth-nearest row. In the worst case — a query somewhere with no benches for tens of kilometres — this degenerates toward a full scan. The cutoff caps the work.

*Product correctness:* KNN's answer to "nearest 20 benches" in the middle of nowhere is 20 benches spread across half a province. Those are the nearest benches, and they are also not an answer to "where can I sit?" The cutoff encodes the judgement that beyond a certain distance the honest answer is "nothing nearby."

---

## D3 — 5000m cutoff, `LIMIT` 15–20

**Decision:** Cutoff 5000m; `LIMIT` a server-side constant in the 15–20 range.

**Rationale:** Both are defaults chosen for plausibility, explicitly flagged for tuning against real usage rather than presented as derived. 5000m is generously beyond walking distance, so it acts as a backstop rather than a routine constraint. 15–20 results fills a map view and a scrollable list without overwhelming either.

**`LIMIT` is not client-exposed at MVP.** Keeping it server-side means it can be tuned without an API contract change or a client release, and avoids the whole class of questions around clamping and abuse that a client-supplied `limit` parameter introduces.

---

## D4 — Empty result set is a 200, not a 404

**Decision:** Zero benches within the cutoff returns HTTP 200 with an empty `benches` array.

**Rationale:** The request was well-formed and the query ran correctly. "There are no benches near this location" is a successful, accurate answer — it's information, not a failure. A 404 would mean the *endpoint* wasn't found, and would push the client into treating a normal outcome as an error path.

---

## D5 — Deterministic tiebreak deferred

**Decision:** Ship without `, id` as a secondary sort key. Track as post-MVP.

**Rationale for taking it seriously at all:** The subtle part is that this isn't only a cosmetic ordering issue. When rows tie on exact distance at the `LIMIT` boundary, *which* tied rows fall inside the limit is unspecified — so the returned set, not just its order, can differ between two identical queries.

**Rationale for deferring anyway:** Exact float-distance ties between distinct benches are rare in practice, the user-visible consequence is negligible, and the fix is a two-word change against an already-indexed column. Low impact, cheap later — a clean deferral rather than a risky one.

---

## D6 — `chi` as the router

**Decision:** `chi`.

**Alternatives considered:** stdlib `net/http` with `ServeMux` alone; Gin; Echo; Fiber.

**Rationale:** `chi` is a thin layer over `net/http` rather than a framework with its own request/response abstractions. Handlers stay `http.HandlerFunc`, and middleware stays standard `func(http.Handler) http.Handler` — which is what the visitor-ID middleware from the visitor-identity design needs to compose cleanly.

**Learning-goal reasoning:** Gin or Fiber would teach their own conventions. `chi` keeps the Go standard library's HTTP model front and centre while adding the routing ergonomics that plain `ServeMux` lacks. For learning Go rather than learning a Go framework, that's the right side of the trade.

---

## D7 — Layered handler → service → repository, with an interface at the repository boundary

**Decision:** Three layers, repository behind an interface.

**Rationale:** More structure than a single-endpoint MVP strictly needs — a deliberate instance of the project's "over-engineering for learning value" principle. The interface boundary makes the service layer unit-testable without a live PostGIS instance, and the separation is the idiomatic Go shape that becomes genuinely necessary as soon as a second endpoint appears.

---

## D8 — Validate before querying

**Decision:** Range-check `lat` and `lon` in the handler before the query is issued.

**Rationale:** An out-of-range coordinate is a client bug, not a database question. Failing fast avoids burning a connection and a 3s timeout budget on a query that cannot produce a meaningful answer, and it keeps the error attributable to the right layer.

---

## D9 — 3-second per-query timeout via `context.WithTimeout`

**Decision:** Every query carries a 3s context deadline.

**Rationale:** Bounds worst-case latency for the client and ensures a slow or stuck query releases its connection rather than tying up the pool. Also a natural fit for practising Go's context propagation, which is the idiomatic mechanism for exactly this.

---

## D10 — Raw OSM tags excluded from the response

**Decision:** Expose only the derived boolean attributes (`has_backrest`, `is_covered`, `is_accessible`) plus coordinates and distance. Keep the raw tag blob internal.

**Rationale:** OSM tags are inconsistent, free-form, and occasionally junk. Exposing them would leak ingestion-layer messiness into a public API contract and invite the client to start interpreting tag semantics. Deriving a small stable set of booleans server-side keeps the contract clean and the interpretation logic in one place.

---

## D11 — Bench detail sheet reads from the list cache

**Decision:** No separate `GET /benches/{id}` endpoint at MVP. The detail view uses the object already held in the client's list-response cache.

**Rationale:** The list response already carries every field the detail sheet displays. A second round trip would fetch data the client demonstrably already has. If the detail view later grows fields the list doesn't carry (photos, OSM history, dedication text from an OpenBenches enrichment layer), the endpoint gets added then.

---

## Unresolved at time of writing

- **Error response body convention** for validation failures. Flagged as important beyond this endpoint — whatever shape is chosen becomes the project-wide convention.
- **`count` field** in the response envelope.
- Final tuning of `LIMIT` and cutoff.
