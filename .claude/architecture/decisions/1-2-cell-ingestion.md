# Cell Ingestion (Cold and Stale) — Decision Log

Decisions leading to `../designs/1-2-cell-ingestion.md`.

**Updated:** 2026-10-08

This log also holds the superseded entries for the earlier cascade pre-warm, which is folded into this design. Those entries (D4, D5) and the retry entry (D6) are kept intact, as the record of options that were tried and dropped.

---

## D1 — Cold start is synchronous and blocking

**Decision:** The first request in an unpolled cell blocks while Overpass is queried, and returns real results inline. The triggering user sees no pending state.

**Alternatives considered:**
- Return an empty or partial result at once with a "fetching, check back" state, and backfill asynchronously.
- Return cached-adjacent data with a staleness marker.

**Rationale:** "Where can I sit?" has a short patience window. Returning an empty list to the first user in an area is a bad first impression, and a check-back state is worse than a short wait in an app this simple.

**Assumption, stated as falsifiable:** Overpass latency for a bounded bbox query is millisecond-scale. If real latency turns out to be seconds, the design flips to async with a push, which was one of the reasons the push design existed.

**Status:** ⚠ Superseded by D7 (2026-10-08).

---

## D2 — Cold-start failure returns an empty result and does not mark the cell polled

**Decision:** If the synchronous Overpass fetch fails or times out during cold start, the request returns `200` with an empty `benches` array. The cell is not marked polled.

**Alternatives considered:**

| Option | Why rejected |
|---|---|
| Return a 5xx error | Breaks the "no results is 200" contract (`2-nearby-search` D4) for a case that looks identical to the client: nothing came back. |
| Mark the cell polled with a short TTL and retry later | Adds a "polled but low-confidence" state for a problem that the timeout already bounds. Not worth the schema complexity at MVP. |

**Rationale:** Not marking the cell polled is the part that matters. Caching a failure as a fresh, empty cell would hide real benches from every later query in that area until the freshness window expires.

**Status:** ⚠ Partially superseded by D7 (2026-10-08). The not-marked-polled clause stands and is carried into D8. The empty-array response clause is replaced by D7.

---

## D3 — Explicit 5 s timeout on the Overpass HTTP call

**Decision:** The Overpass HTTP call carries its own 5 s client-side timeout, independent of the 3 s database deadline in `2-nearby-search` D8.

**Rationale:** The millisecond-scale latency assumption in D1 was stated as falsifiable, but nothing bounded a hung call. This gives the call a bound. It now applies inside the ingestion job, not on the request path (D7). The composed request-path latency of about 8 s that earlier applied no longer applies. The 5 s was sized for a single small fetch; the merged job's bbox is about 19x larger, so 5 s is a default flagged for tuning against real Overpass latency.

**Status:** Settled.

---

## D4 — Cascade pre-warming of the surrounding ring at k=2..3, non-blocking, sized at about 900–1400 m

**Decision (original):** After a cell ingests, dispatch background pre-warm jobs for its k=2 to k=3 ring neighbours. Never block the triggering request on these. Ring size was sized on walking distance at roughly 900 m to 1400 m.

**Rationale (original):** Turns the cold-start penalty from a per-cell cost into a roughly one-time cost per region visited. The ring-size figure was wrong: at res 8 the centre-to-centre spacing is about 0.8 km, so k=2 reaches about 2.0 km and k=3 about 2.8 km. The figure of 900 m to 1400 m does not match either.

**Status:** ⚠ Superseded by D8 and D9 (2026-10-08).

---

## D5 — Cascade fan-out capped at depth 1

**Decision (original):** A cell warmed by cascade never triggers a further cascade. Only a cell reached by a direct user query originates one.

**Rationale (original):** Without a cap, cascade warming has no stated bound.

**Trade-off accepted (original):** a user who walks several cells outward may still hit an occasional cold cell beyond the warmed ring.

**Status:** ⚠ Superseded by D8 (2026-10-08). With one job covering the k=2 disk, nothing re-dispatches, so there is no fan-out to cap. Holds only while `pending_cells` is limited to cells inside the job's disk (open item on `pending_cells` scope).

---

## D6 — Retry with a smaller radius when Overpass returns no benches

**Decision (original):** If the bounded Overpass query returns nothing, retry with a smaller radius before concluding the area is empty.

**Rationale (original):** Stated purpose: conclude that an area is genuinely empty only after a retry. No other rationale was recorded.

**Status:** ⚠ Superseded by D10 (2026-10-08). A smaller bbox is a subset of the larger one. If the larger query returned nothing, the smaller one cannot return more.

---

## D7 — Option B: answer from PostGIS at once; cold or stale cells are ingested asynchronously and announced by push

**Decision:** The request is answered from PostGIS regardless of freshness. A cold or stale cell is dispatched for background ingestion and listed in `pending_cells`. When the job completes, a push tells the client, and the client re-calls the endpoint. This supersedes D1, and the empty-array response clause of D2.

**Dating:** the 2026-10-08 design discussion dates this decision to 2026-09-08. It was not logged on that date. It is logged on 2026-10-08.

**Alternatives considered:**

| Option | Why rejected |
|---|---|
| Synchronous blocking cold start (D1) | The request includes an Overpass call. With the 3 s query deadline and the 5 s Overpass timeout, worst-case latency is about 8 s. The contract did not reconcile the two. |
| A 3 s timeout on the nearby request, with ingestion inside it | Overpass cannot be bounded under 3 s. |
| Push on cache miss | Its response contract conflicted with the other two framings. The 2026-10-08 design discussion does not spell out the conflict beyond that. |

**Rationale:** The three framings above implied contradictory contracts. Option B is the framing that makes them coherent. The nearby response is always fast, and the push is how pending cells become ready. Flutter needs an explicit "empty now, may arrive shortly" state for this to work (`7-flutter-client` D6).

**Trade-off accepted:** the first user in a cold area gets an empty or partial answer plus a pending signal, not inline results. This reverses the earlier argument that a first-time empty list is a bad first impression (D1).

**Status:** Settled.

---

## D8 — One ingestion job per cold or stale cell, covering the k=2 disk

**Decision:** A cold or stale cell dispatches one message. The job issues one Overpass bbox request covering the k=2 disk (19 cells). On success it assigns benches to cells, upserts them, marks all 19 cells polled, and publishes one Mercure update covering those 19 cells. On failure or timeout it marks nothing polled and publishes nothing. Supersedes D4 and D5.

**Alternatives considered:**

| Option | Why rejected |
|---|---|
| Two jobs: the user's cell first, then the ring | The user's cell arrives sooner, but costs two Overpass calls and keeps a depth cap. |
| A separate job per ring cell (up to 18 to 36 calls) | Multiplies the rate-limit exposure of a fill one bounded call already covers. |

**Rationale:** One Overpass call instead of up to 18 to 36 matters under Overpass rate limits. The single job also removes the depth cap, since nothing re-dispatches. Holds only while `pending_cells` is limited to cells inside the job's disk (open item on `pending_cells` scope).

**Failure behaviour:** the cells stay unpolled, so the next request for the cell dispatches again. The client receives no push and falls back to pull-to-refresh after 15 s (`5-realtime-push`). A failure is not cached as a fresh empty cell (D2).

**Trade-off accepted:** the user's own cell is published only when the whole disk is complete.

**Status:** Settled.

---

## D9 — Ring size: k=2, on corrected numbers

**Decision:** The ingestion ring is the k=2 disk: 19 res-8 cells, reach about 2.0 km.

**Corrected figures:** res-8 cells average about 0.74 km² with centre-to-centre spacing about 0.8 km. Reach by k: k=1 about 1.2 km (7 cells); k=2 about 2.0 km (19 cells); k=3 about 2.8 km (37 cells).

**Alternatives considered:** k=1 (7 cells, about 1.2 km) and k=3 (37 cells, about 2.8 km). Neither was argued separately in the 2026-10-08 design discussion. The walking-distance criterion selects k=2.

**Rationale:** k=2 is chosen on walking distance with correct numbers. It also closes the dangling justification from the earlier radius-ladder design, which no longer exists (`2-nearby-search` D1). The ~2 km figure is measured from the request cell's centre. From an arbitrary point inside that cell, the guaranteed covered radius is ~1.4 km (disk inradius ~1.85 km minus cell circumradius ~0.46 km). The 19-cell disk is ~14 km², about 70 benches at 5/km².

**Status:** Settled. Supersedes the ring-size clause of D4.

---

## D10 — No retry with a smaller radius

**Decision:** A bounded Overpass query that returns no benches is taken as the area being empty. No smaller-radius retry. Supersedes D6.

**Rationale:** A smaller bbox cannot return more than a larger one that returned nothing. The retry was a defect in the design, not an option.

**Status:** Settled.

---

## Unresolved at time of writing

- Overpass retry policy: whether the queue also retries a failed job, and how many times (design Open item 1).
- Duplicate in-flight jobs for the same cell (design Open item 2).
- Reconciliation (deletes and moves) is not part of the job steps; cells only partly inside the bbox must be excluded from any diff (design Open item 3).
- Scope of the freshness check and `pending_cells` (`2-nearby-search` open item 3).
- Subscribe-before-publish race (`5-realtime-push` open items).
