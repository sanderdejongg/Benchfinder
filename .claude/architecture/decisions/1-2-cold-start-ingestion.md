# Cold-Start Ingestion — Decision Log

Decisions leading to `../designs/1-2-cold-start-ingestion.md`.

**Provenance note:** split out of the pipeline-wide `1-ingestion-pipeline` decision log. The entries below carry fresh, locally-scoped numbering (D1–D3); their prior identities in that log were D5, D15, and D16 respectively — see that log for pointers, and note other designs may still cite those old numbers until updated.

---

## D1 — Cold start is synchronous and blocking

**Decision:** The first request in an unpolled cell blocks while Overpass is queried, and returns real results inline. No "pending" state for the triggering user.

**Alternatives considered:**
- Return an empty/partial result immediately with a "we're fetching, check back" state, and backfill asynchronously.
- Return cached-adjacent data with a staleness marker.

**Rationale:** "Where can I sit?" is a question with a short patience window. Returning an empty list to the very first user in an area is a bad first impression, and a "check back later" state in a utility app this simple is worse than a two-second wait. Blocking keeps the mental model simple: you ask, you get benches.

**Assumption made explicit:** Overpass latency for a bounded bbox query is ms-scale. This is written into the cold-start design as a falsifiable assumption. If real latency proves to be seconds, the design flips to async + the real-time push channel (`5-realtime-push`) — which is one of the reasons that design exists at all.

**Status:** Settled.

---

## D2 — Cold-start failure returns empty, does not mark the cell polled

**Decision:** If the synchronous Overpass fetch fails or times out during cold start, the request returns `200` with an empty `benches` array. The cell is **not** marked polled.

**Alternatives considered:**

| Option | Why rejected |
|---|---|
| Return an error (5xx) | Breaks the "no-results is 200" contract already established in `2-nearby-search` for a case that's operationally identical from the client's point of view — a search that comes back with nothing. |
| Mark the cell polled with a short TTL, retry later | Adds state (a "polled but low-confidence" flag) for a problem D3's timeout already bounds tightly; not worth the schema complexity at MVP. |

**Rationale:** Treating an Overpass failure as "no results" keeps the client's error-handling surface small and consistent with the existing empty-result contract. Not marking the cell polled is the part that matters most: caching a failure as a fresh, empty cell would silently hide real benches from every subsequent query in that area until the freshness window (`1-1-h3-grid-freshness` D4) expires.

**Status:** Settled.

---

## D3 — Explicit 5s timeout on the Overpass HTTP call

**Decision:** The Overpass fetch during cold start carries its own 5s client-side timeout, independent of `2-nearby-search`'s 3s PostGIS query deadline.

**Rationale:** The ms-scale Overpass latency assumption (D1) was written down as deliberately falsifiable, but nothing actually caught the falsification — a hung Overpass call had no stated bound. This closes that gap. It also means worst-case cold-start latency is the sum of both timeouts (~8s), not the 3s a reader of `2-nearby-search` alone might assume; that composed number wasn't previously stated anywhere.

**Status:** Settled.

---

## Unresolved at time of writing

None.
