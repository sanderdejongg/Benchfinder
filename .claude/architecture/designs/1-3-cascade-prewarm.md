# Cascade Pre-Warming — Final Design

**Parent:** `1-ingestion-pipeline`
**Status:** Settled for MVP, not implemented; one dangling rationale reference (see Open items)
**Updated:** 2026-09-08

---

## Purpose

After any cell finishes ingesting (via `1-2-cold-start-ingestion`), pre-warm its neighbours so the next request nearby hits a warm cache instead of paying the cold-start cost again.

---

## Constraints

- Must never block the triggering request.
- Fan-out cost must be bounded — no unstated ripple across the whole grid.
- Ring size should be defensible against actual human walking distance, not an arbitrary constant.

---

## Design

- Compute `gridRing` / `gridDisk` neighbours at **k = 2 to 3** at res 8, sized on the assumption that a person looking for a bench may plausibly walk that far (roughly 900m–1400m outward).
- Dispatched as **background jobs that never block the triggering request**, via the worker pool in `1-4-dispatch-mechanism`.
- **Fan-out is capped at depth 1: a cascade-warmed cell never itself triggers cascade.** Only a cell reached via a direct cold-start user query originates a cascade. Jobs dispatched by cascade pre-warm carry a flag the worker pool checks before deciding whether to re-dispatch — without this, a single query could in principle ripple outward with no bound.
- This is the path that makes `5-realtime-push` meaningful: a client whose viewport overlaps a neighbour cell still being pre-warmed is the one that gets a push when it commits.

---

## Contracts & data model

None of its own — writes go through the same `benches` / `polled_cells` upsert path as cold-start ingestion.

---

## Dependencies

- `1-ingestion-pipeline` — parent: guiding constraints, end-to-end flow.
- `1-1-h3-grid-freshness` — `gridRing`/`gridDisk` adjacency this design relies on.
- `1-4-dispatch-mechanism` — the execution substrate; this is currently its only consumer.
- `5-realtime-push` — the consumer of the completion event this design generates.

---

## Explicitly deferred

None specific to this sub-design.

---

## Superseded

None.

---

## Open items

1. **Dangling walking-distance rationale.** The k=2..3 ring size was originally justified partly by reference to the 750m/1500m radius ladder from `2-nearby-search`'s pre-KNN scope. That ladder no longer exists (superseded by KNN — see `2-nearby-search` §10). The k value is still independently defensible on walking-distance grounds alone, but the stated justification chain currently points at a design that doesn't exist anymore. Needs a clean restatement rather than leaving the dangling reference. (Carried forward from a note in the prior combined decision log rather than left buried there.)
