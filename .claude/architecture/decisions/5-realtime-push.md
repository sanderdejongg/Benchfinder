# Real-Time Push — Decision Log

Decisions leading to `../designs/5-realtime-push.md`.

---

## D1 — Push rather than poll

**Decision:** Push a "ready" signal to the client when ingestion commits, instead of having the client poll.

**Alternatives considered:**
- Client polls `/benches/nearby` on an interval until results change.
- Client does nothing; the user pulls to refresh if they feel like it.

**Rationale:** Polling for an event that may arrive in 400ms or may never arrive at all is a poor fit — either the interval is short and most requests are wasted, or it's long and the feature feels broken. The "do nothing" option is genuinely defensible for a utility app and remains the likely *fallback* (see D8), but push is the more interesting problem and produces a better experience when it works.

**Honest framing:** the user-facing benefit here is modest. The scope was deliberately narrowed (D4) rather than inflated to justify the machinery.

---

## D2 — Postgres `LISTEN/NOTIFY` as the cross-process signal

**Decision:** The ingestion worker signals the API instances via `NOTIFY`; each API instance holds a `LISTEN` connection.

**Alternatives considered:**

| Option | Why rejected |
|---|---|
| In-process event bus (Go channel) | **Impossible given the architecture.** The deployment-shape design runs ingestion as a separate batch process from the API. A channel cannot cross a process boundary. |
| Redis pub/sub | Works, but adds a whole piece of infrastructure that exists solely for this one signal. |
| Ingestion worker calls an internal API endpoint | Requires the worker to know about and reach every API instance, which reintroduces service discovery for one message. |

**Rationale:** Postgres is already in the architecture and already the thing the ingestion worker is talking to at the moment the event occurs. `LISTEN/NOTIFY` gets pub/sub semantics with zero new infrastructure, and Postgres fans notifications out to every listening connection — which means horizontal scaling of API instances works without any additional coordination.

**The property that made this click:** `NOTIFY` only delivers *after* the transaction commits. That is exactly the guarantee needed — a client must never be told "cell X is ready" and then read a snapshot that doesn't contain the new rows. Getting that for free, rather than having to reason about ordering between a commit and a separate signal, is the strongest argument for this option.

**Cost accepted:** a dedicated long-lived connection per API instance, held outside the query pool because it blocks. And the pooler constraint in D7.

**Status:** Mechanism settled; original rationale corrected — see D9.

---

## D3 — WebSocket over SSE

**Decision:** WebSocket.

**The honest comparison:** the channel is strictly one-way, server→client, carrying a tiny text payload. Server-Sent Events is the better-fitting tool by almost every technical measure — simpler protocol, plain HTTP, automatic browser reconnection, no upgrade handshake, no framing to reason about.

**Rationale for choosing WebSocket anyway:** learning value, stated explicitly rather than rationalised. WebSocket is the more broadly applicable primitive, the bidirectional case is where the interesting concurrency problems live in Go, and if the persistent-viewport stretch goal is ever picked up it becomes genuinely bidirectional anyway.

This is a clean example of the project's "learning value counts alongside technical suitability" principle — the decision is recorded as a preference, not dressed up as a technical necessity.

---

## D4 — Transient connections, not persistent viewport tracking

**Decision:** Open a WebSocket only when a `nearby` call returns a pending cache-miss state; close it after the single push arrives.

**Alternative considered:** a persistent connection tracking the user's live-panning map viewport, pushing updates for any cell that enters view and finishes ingesting.

**Why the persistent version was rejected for MVP:** it requires subscription updates as the viewport moves, multi-cell tracking per connection, heartbeats and liveness detection, reconnection state carrying the current subscription set, and a much larger registry. That's a substantial amount of machinery, and the user-visible benefit is questionable — a person looking for a bench is not typically panning across a map waiting for new benches to appear.

Recorded as a possible future stretch goal, explicitly not silently dropped.

---

## D5 — Payload is an invalidation signal, not data

**Decision:** The push says "cell X is ready." The client re-calls `GET /benches/nearby`. Bench data never travels over the WebSocket.

**Rationale:** Keeps the REST response as the single source of truth. If bench data went over both channels, they would need identical serialisation, identical filtering (the KNN limit, the distance cutoff), and would drift apart the first time either changed. The invalidation approach means the WebSocket path has no schema of its own to version and no consistency story to maintain.

Costs one extra round trip, which is trivially acceptable for a signal that arrives asynchronously anyway.

---

## D6 — Scoped to cascade pre-warm, not to the user's own request

**Decision:** This mechanism exists for cells warmed by the k=2..3 cascade, not for the cold-start cell the user actually queried.

**Rationale:** The ingestion pipeline decided the user's own cold-start request blocks and returns real data inline. There is nothing to push for that request — it already got its answer. The value of push is entirely in the *surrounding* cells, which the user may pan into next.

Worth stating explicitly because it's easy to misread this design as "the cold-start fix," which would put it in direct conflict with the cold-start design's synchronous behaviour.

**Latent connection:** if the assumption about ms-scale Overpass latency turns out to be wrong, the cold-start path would flip to async — and this mechanism is already the thing that would make that flip possible. That's a real, if unstated, part of why building it is worthwhile.

---

## D7 — Accepting the connection-pooler constraint

**Decision (implicit, now made explicit):** choosing `LISTEN/NOTIFY` constrains the deployment-shape design's provider choice.

**The problem:** `LISTEN/NOTIFY` does not work through a connection pooler in transaction mode. Several managed providers default to exactly that — Supabase notably fronts connections with pgBouncer.

**Consequence:** the provider must offer a direct, non-pooled connection for the listener, or this architecture doesn't work as designed. This should be verified *before* the deployment-shape provider decision is made. It is arguably the single strongest filter on that still-open choice, stronger than pricing or free-tier limits.

---

## D8 — Client fallback leaning toward pull-to-refresh

**Status: open, but with a stated lean.**

If the WebSocket fails to establish, the options are to fall back to polling or to fall back to nothing and let the user pull to refresh.

**Leaning toward pull-to-refresh:** it's simpler, it degrades honestly rather than burning battery on speculative requests, and for a utility app the user is already in a position to just look again. Polling as a fallback would reintroduce exactly the mechanism D1 rejected, only in the less reliable case.

---

## D9 — Correcting D2's stated rationale: multi-instance fan-out, not process separation

**Decision:** No change to the mechanism — `LISTEN/NOTIFY` stands. The *reason* given in D2 was wrong and is corrected here.

**What was wrong:** D2 justified `LISTEN/NOTIFY` by saying "the ingestion worker is a separate process from the API," attributing this to the deployment-shape design's batch job. But the batch job (Geofabrik-based bulk refresh) isn't the source of the events this design pushes on — those come from the ingestion-pipeline design's **cascade pre-warm** jobs, which run in-process inside the API via a goroutine worker pool (that design's §7), not as a separate process at all.

**The actual constraint:** multi-instance fan-out. A cascade job dispatched by API instance A completes on instance A, but the client's WebSocket connection may be held by instance B — nothing pins a client to the instance that happened to do the work. An in-process Go channel can't cross that boundary; `LISTEN/NOTIFY` can, because Postgres fans the notification out to every listening instance regardless of which one produced it.

**Why this still matters even though the conclusion doesn't change:** the wrong reason invites a wrong fix later — someone reading D2 literally could "resolve" the ingestion-pipeline/deployment-shape process question and then wrongly conclude `LISTEN/NOTIFY` is no longer needed. The real dependency is on having multiple API instances at all, not on where ingestion runs.

**Status:** Settled. See ingestion-pipeline design §7 and deployment-shape decision D9 for the related (separate) Overpass/Geofabrik split — the two corrections are not the same issue, despite both stemming from conflating the deployment-shape batch job with the ingestion-pipeline's in-process worker pool.

---

## Unresolved at time of writing

1. **Timeout handling.** What happens when ingestion never completes — Overpass down, worker crashed, job lost from the in-process queue on restart (a known gap from the dispatch mechanism's no-persistence decision). The socket needs a deadline and the client needs to be told something when it expires.

2. **`LISTEN` reconnect resilience.** Reconnect with backoff is obvious. The harder question is the correctness gap: `LISTEN/NOTIFY` has no replay, so notifications fired during a disconnect are lost permanently. Whether that needs closing — and if so, whether via a polled `polled_cells` reconciliation sweep on reconnect — is open.

3. **Registry data structure.** Per-instance connection → cell mapping, needing cell-keyed lookup on NOTIFY, cleanup on disconnect, and safety under concurrent access.

4. **Client-side fallback behaviour** (see D8 — leaning, not settled).
