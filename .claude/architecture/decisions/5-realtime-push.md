# Real-Time Push — Decision Log

Decisions leading to `../designs/5-realtime-push.md`.

**Updated:** 2026-10-08

---

## D1 — Push rather than poll

**Decision:** Push a "ready" signal to the client when ingestion completes, instead of having the client poll.

**Alternatives considered:**
- Client polls the nearby endpoint on an interval until results change.
- Client does nothing; the user pulls to refresh when they want.

**Rationale:** Polling for an event that may arrive in 400 ms or never arrive is a poor fit. A short interval wastes most requests, and a long one makes the feature feel broken. "Do nothing" is defensible for a utility app and remains the fallback (D8).

**Honest framing:** the user-facing benefit is modest. Under the pending model it matters more, because the first results of a cold area arrive through this signal (D15).

**Status:** Settled.

---

## D2 — Postgres LISTEN/NOTIFY as the cross-process signal

**Decision (original):** The ingestion worker signals the API instances with `NOTIFY` after commit. Each API instance holds a dedicated `LISTEN` connection.

**Alternatives considered (original):**

| Option | Why rejected |
|---|---|
| In-process event bus | Cannot cross the process boundary between the worker and the API. |
| Redis pub/sub | Works, but adds infrastructure that exists only for this signal. |
| Worker calls an internal API endpoint | Requires the worker to reach every API instance, which reintroduces service discovery for one message. |

**Rationale (original):** `NOTIFY` delivers only after the transaction commits, so a client is never told a cell is ready before its rows are readable.

**Status:** ⚠ Superseded by D12 (2026-10-08).

---

## D3 — WebSocket over Server-Sent Events

**Decision (original):** WebSocket as the push transport.

**Honest comparison (original):** the push is strictly one-way, carrying a small text payload. Server-Sent Events is the better-fitting tool by most technical measures: simpler protocol, plain HTTP, automatic browser reconnection, no upgrade handshake.

**Rationale (original):** learning value, stated explicitly rather than rationalised. WebSocket is the broader primitive, and the bidirectional case is where the interesting concurrency problems live. The decision was recorded as a preference, not as a technical necessity.

**Status:** ⚠ Superseded by D12 (2026-10-08).

---

## D4 — Transient connections, not persistent viewport tracking

**Decision:** Open a stream only when a nearby response lists pending cells. Close it after the single event arrives, or after the timeout.

**Alternative considered:** a persistent connection tracking the live map viewport, pushing updates for any cell that enters view.

**Why the persistent version was rejected for MVP:** it needs subscription updates as the viewport moves, multi-cell tracking, heartbeats, reconnect state carrying the subscription set, and a much larger registry. The user-visible benefit is questionable. A person looking for a bench does not typically pan across a map waiting for new benches.

**Status:** Settled. Persistent viewport tracking is explicitly deferred (design section 7).

---

## D5 — Payload is an invalidation signal, not data

**Decision:** The push says which cells are ready. The client re-calls the nearby endpoint. Bench data never travels over the push path.

**Rationale:** The REST response remains the single source of truth. If bench data went over both paths, they would need identical serialisation and identical filtering (the KNN limit, the cutoff), and they would drift apart the first time either changed. The invalidation approach gives the push no schema of its own and no consistency story.

**Trade-off accepted:** one extra round trip, which is acceptable for a signal that arrives asynchronously anyway.

**Status:** Settled.

---

## D6 — Scoped to cascade pre-warming, not to the user's own request

**Decision (original):** This mechanism exists for cells warmed by the cascade, not for the cold cell the user queried. The user's own cold request blocks and returns real data inline, so there was nothing to push for it.

**Status:** ⚠ Superseded by D15 (2026-10-08). The user's own request now returns pending cells, so push is needed for it.

---

## D7 — Accepting the connection-pooler constraint

**Decision (original):** Choosing LISTEN/NOTIFY constrains the database provider. A pooler in transaction mode breaks LISTEN, so the provider must offer a direct connection for the listener. This was the strongest filter on the provider choice.

**Status:** ⚠ Superseded by D12 (2026-10-08). There is no listener connection, so the constraint no longer applies.

---

## D8 — Client fallback: pull-to-refresh, no polling

**Decision:** If the stream does not establish, or the event does not arrive in time, fall back to pull-to-refresh. No polling.

**Rationale:** Simpler. It degrades honestly rather than spending battery on speculative requests. For a utility app, the user is already in a position to look again. Polling as a fallback would reintroduce the mechanism rejected in D1, in its least reliable form.

**Status:** Settled.

---

## D9 — Correcting D2's stated rationale: multi-instance fan-out

**Decision (original):** D2's justification was corrected. The constraint is multi-instance fan-out. A job dispatched by one API instance can complete on that instance while the client's stream is held by another, so an in-process event bus cannot reach it.

**Status:** ⚠ Superseded by D12 (2026-10-08). The mechanism it corrects has been replaced.

---

## D10 — Timeout of 15 seconds

**Decision:** The client abandons a pending stream after 15 s and falls back to pull-to-refresh.

**Rationale:** The stream exists to wait for an ingestion job that may never finish: Overpass could be down, the worker could fail, or the message could be lost before the queue was durable. The wait cannot be unbounded. 15 s is a multiple of the 5 s Overpass timeout (`1-2-cell-ingestion` D3), long enough for a job to finish and short enough that a stuck job does not leave the user waiting.

**On expiry:** the client treats this as "nothing is ready yet", not as an error. Cells may still warm afterwards, and a later pull-to-refresh picks them up.

**Trade-off accepted:** the explicit close reason from the earlier WebSocket design does not carry over. The client enforces the timeout itself.

**Status:** Settled as a default, flagged for tuning against real completion times. Same category as the freshness window (`1-1-h3-grid-freshness` D4).

---

## D11 — Accept the replay gap; no reconciliation sweep

**Decision:** An event fired while the client is not attached is not replayed. No reconciliation sweep is built to close that gap at MVP.

**Alternatives considered:**

| Option | Why rejected |
|---|---|
| Periodic reconciliation sweep over `polled_cells` for newer cells | A real fix, but a second consistency mechanism to build and maintain for a gap that already degrades into a supported path. |

**Rationale:** A missed event only delays the push. It never affects the correctness of the underlying data, because the rows are committed before the publish. A client that misses the event falls back to pull-to-refresh and sees correct data. Building reconciliation now would solve a cosmetic gap with permanent machinery.

**Status:** ⚠ Partially superseded by D12 (2026-10-08): the reconnect-with-backoff clause is removed with the long-lived listener. The replay-gap clause stands, and is explicitly deferred. This decision does not cover the subscribe-before-publish race, which is an open item in the design.

---

## D12 — Mercure (SSE) hub replaces WebSocket and LISTEN/NOTIFY

**Decision:** Push is delivered as Server-Sent Events through a Mercure hub. The publisher (the ingestion job) posts an update to the hub over HTTP. Clients subscribe to cell topics. This replaces the WebSocket transport and the LISTEN/NOTIFY cross-instance signal. It supersedes D2, D3, D7 and D9.

**Alternatives considered:**

| Option | Why rejected |
|---|---|
| WebSocket with LISTEN/NOTIFY (the earlier design) | Needs a long-lived listener connection per API instance, held outside the query pool. Needs a direct database connection, which constrains the provider. Needs a per-instance connection registry. |

**Rationale:** The hub does the fan-out. Publishers post over HTTP, so no listener connection, no per-instance registry and no database-notification constraint are needed. The push is one-way, which the earlier analysis already identified as the better fit for SSE (D3, superseded).

**Trade-off accepted:** the learning value that motivated the WebSocket choice is given up for this transport. A later WebSocket-vs-Mercure comparison is deferred (design section 7).

**Status:** Settled.

---

## D13 — Mercure hub embedded in the app process; single app instance

**Decision:** The Mercure hub runs embedded in the app's FrankenPHP process. At MVP there is one app instance. The hub is standalone only once a second app instance exists.

**Alternatives considered:**

| Option | Why rejected |
|---|---|
| Standalone hub now | Infrastructure for a scale that does not exist. |
| Managed Mercure hosting with high availability | Paid. |

**Rationale:** The open-source hub is single-node. Publishers post to it over HTTP, so fan-out is solved as long as there is one hub. Adding a standalone hub only pays off when there is more than one app instance to fan out between.

**Trade-off accepted:** the hub is a single point. If the app process is down, pushes stop, and clients fall back to pull-to-refresh.

**Revisit trigger:** a second app instance. Then move to a standalone hub.

**Status:** Settled.

---

## D14 — One update per job, topics covering every cell the job touched

**Decision:** The ingestion job publishes one Mercure update whose topics list every cell it touched. A subscribed client receives exactly one event.

**Alternatives considered:** one update per cell. A client subscribed to several cells would receive several events, and would re-call the endpoint for each.

**Rationale:** The client is meant to receive exactly one event and re-call once.

**Status:** Settled.

---

## D15 — Push is the primary first-population path, plus staleness refresh

**Decision:** Under the pending model (`2-nearby-search` D14), push is the primary way a cold area's first results arrive. It also refreshes stale cells that were answered from older data. The user's own request is covered. Supersedes D6.

**Rationale:** Under the pending model a cold request returns no inline bench data for its own cell. Push is the only way that cell's results arrive without a poll.

**Trade-off accepted:** the first results of a cold area now wait on push, and on the 15 s timeout if push does not arrive.

**Status:** Settled.

---

## Unresolved at time of writing

- Subscribe-before-publish race (design open item 1).
- Topic name format and event payload shape (design open items 2 and 3).
- Publisher authorisation for the hub (design open item 4).
- Client re-call policy (design open item 5).
