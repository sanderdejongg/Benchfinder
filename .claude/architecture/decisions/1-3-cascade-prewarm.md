# Cascade Pre-Warming — Decision Log

Decisions leading to `../designs/1-3-cascade-prewarm.md`.

**Provenance note:** split out of the pipeline-wide `1-ingestion-pipeline` decision log. The entries below carry fresh, locally-scoped numbering (D1–D2); their prior identities in that log were D6 and D13 respectively — see that log for pointers.

---

## D1 — Cascade pre-warming at k=2..3, non-blocking

**Decision:** After a cell ingests, dispatch background pre-warm jobs for its k=2 to k=3 H3 ring neighbours. Never block the triggering request on these.

**Rationale:** Turns the cold-start penalty from a per-cell cost into a roughly one-time cost per *region visited*. A user who walks or scrolls outward from where they first searched should be moving into already-warm cells. Roughly 900m–1400m out at res 8, sized on walking distance.

**Known issue, not yet resolved:** The original justification referenced the 750m/1500m radius ladder from the original `2-nearby-search` scope, which was subsequently superseded by KNN. The k value is still defensible on walking-distance grounds, but the stated rationale now points at a design that no longer exists. Promoted to a listed open item in the design doc rather than left as a footnote here.

**Status:** Settled (mechanism); rationale citation needs restating — see design doc Open items.

---

## D2 — Cascade fan-out capped at depth 1

**Decision:** A cell warmed via cascade pre-warm never itself triggers a further cascade. Only a cold-start cell (reached by a direct user query) originates one.

**Rationale:** Without a cap, cascade pre-warming has no stated bound — a single query could in principle warm a ring whose warming warms further rings, with no stopping condition short of running out of unpolled cells. A depth-1 cap keeps the cost of any single user query bounded to at most one ring of neighbours, while still delivering the "walk outward into warm cells" benefit the design is going for.

**Trade-off accepted:** A user who walks several cells outward from their original query may still hit an occasional cold cell beyond the pre-warmed ring, rather than a fully pre-warmed corridor. Preferred over the alternative, which trades a rare cold cell for an unbounded worst case.

**Status:** Settled.

---

## Unresolved at time of writing

See design doc Open items — the walking-distance rationale for D1 cites a superseded design and needs restating.
