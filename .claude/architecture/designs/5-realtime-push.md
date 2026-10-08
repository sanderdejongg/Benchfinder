# Real-Time Push on Ingestion Completion — Final Design

**Issue:** [BEN-5](https://benchfinder.youtrack.cloud/issue/BEN-5)  **Status:** settled, not implemented  **Updated:** 2026-10-08

---

## 1. Purpose

A nearby request answers at once from PostGIS, and may list cells as pending (`2-nearby-search` D14). The ingestion job for a pending cell runs in the background. This design covers how the client learns that a pending cell is ready, so it can re-call the endpoint, without polling.

Push is the primary way a cold area's first results arrive. It also refreshes stale cells that were answered from older data.

---

## 2. Constraints

- **Client stream timeout: 15 s.** The client holds one stream and abandons it after 15 s, then falls back to pull-to-refresh (D10).
- **Overpass timeout: 5 s.** The 15 s client timeout is sized as a multiple of the 5 s Overpass timeout inside the ingestion job (`1-2-cell-ingestion` D3).
- **Single hub.** One Mercure hub, embedded in the single app instance (D13).
- **Invalidation only.** Topics are public. The payload carries cell indexes and nothing else. The visitor ID never appears in a topic or payload (`3-visitor-identity`).

---

## 3. Architecture

```
GET /benches/nearby  (app)
   │  200 { benches, pending_cells }  +  Link: <hub>; rel="mercure"
   ▼
Client opens one SSE stream to the Mercure hub, subscribed to the pending cell topics
   │
   ▼
Ingestion worker (separate process)
   │  upsert benches, mark cells polled
   │  publish ONE update to the hub, topics = every cell the job touched
   ▼
Mercure hub (embedded in the app's FrankenPHP process, D13)
   │  pushes one event to the subscribed stream
   ▼
Client re-calls GET /benches/nearby, once, and renders the result
```

The update is published after the upserts and polled marks are committed. The client therefore re-calls against data that is already present (see Open items for the subscribe race).

---

## 4. Settled design decisions

### Transport: Mercure over SSE

Server-Sent Events, through the Mercure hub. The push is strictly one-way, server to client, with a small payload. The publisher posts to the hub over HTTP. There is no per-instance listener connection and no database notification (D12).

### Connection lifecycle: transient

The client opens a stream only when a nearby response lists pending cells. It closes the stream after the one event arrives, or after the 15 s timeout. This is not a persistent viewport connection (D4).

### Payload: invalidation signal only

The event says which cells are ready. It does not carry bench data. The REST response stays the single source of truth for bench data (D5).

### Topics

One topic per res-8 cell. The topic name format is an open item. Topics are public: a subscriber needs only the cell index. The payload reveals nothing beyond "these cells are ready".

### One update per job

The job publishes one update whose topics cover every cell it touched, so a subscribed client receives exactly one event (D14).

### Hub topology

One Mercure hub, embedded in the app's FrankenPHP process. Single app instance at MVP (D13).

### Client timeout: 15 seconds

The client abandons the stream after 15 s and falls back to pull-to-refresh. The timeout is sized as a multiple of the 5 s Overpass timeout (`1-2-cell-ingestion` D3). The client enforces the timeout itself. The value is a default flagged for tuning against real completion times.

### Missed events

An event that is not delivered is not replayed. A missed event only delays the push. It never affects the correctness of the data, because the rows are committed before the publish (D11). The client then waits for the 15 s timeout and falls back to pull-to-refresh.

### Client fallback: pull-to-refresh only

If the stream does not connect, or the event does not arrive in time, the client falls back to pull-to-refresh. There is no polling fallback (D8).

---

## 5. Contracts & data model

- Hub discovery: `Link: <hub-url>; rel="mercure"` on the nearby response (`2-nearby-search` D15).
- Publish: one POST to the hub per job, with one topic parameter per cell touched.
- Subscribe: one stream per client, with one topic parameter per pending cell.
- Event payload: the set of cell indexes that became ready. No bench data. Exact JSON shape is an open item.
- Visitor ID: never in a topic, never in a payload (`3-visitor-identity`).

---

## 6. Dependencies

- `1-2-cell-ingestion`: the job that produces the event, and the cells it covers.
- `1-4-dispatch-mechanism`: the queue the job travels through.
- `2-nearby-search`: the response that lists pending cells and carries the `Link` header.
- `3-visitor-identity`: the rule that the visitor ID never appears in a topic or payload.
- `4-deployment-shape`: the hub is inside the app container. Single instance.
- `7-flutter-client`: the client stream and its states.

---

## 7. Explicitly deferred

- **Persistent viewport-tracking connections.** Subscription updates as the map moves, multi-cell tracking, heartbeats, reconnect state. Substantial machinery for a benefit that is questionable for a bench finder. Possible future stretch goal.
- **WebSocket vs Mercure comparison.** Deferred. A later comparison with WebSocket is of interest.
- **Standalone or highly available hub.** Trigger: a second app instance. See D13.
- **Reconciliation sweep for missed events.** Not built (D11).
- **15 s timeout tuning.** Named, not open. Tuned against real completion times (D10).

---

## 8. Superseded

- Postgres LISTEN/NOTIFY as the cross-process signal, with a per-instance listener and a WebSocket registry: replaced by Mercure (D12).
- WebSocket transport: replaced by SSE through Mercure (D12).
- Push scoped to the cascade ring only, not the user's own request: replaced. Push is now the primary first-population path (D15).
- Connection-pooler constraint on the database provider: no longer applies, because there is no listener connection (D12).

---

## 9. Open items

1. **Subscribe-before-publish race.** A job can finish before the client has attached its stream. The event is then not delivered, and the client waits for the 15 s timeout before the pull-to-refresh fallback, even though the data is ready. Options not decided: replay of past events from the hub (Mercure's `Last-Event-ID` mechanism, if the hub keeps history), a single PostGIS re-check once the stream is attached, or accepting the delay.
2. **Topic name format.** Not chosen. A cell index is the obvious basis.
3. **Event payload JSON shape.** Not chosen beyond "cell indexes, no bench data".
4. **Publisher authorisation.** The hub's publish endpoint needs authorisation from the publisher. Key handling is not designed.
5. **Client re-call policy.** Whether the client re-calls once per event, or repeats while `pending_cells` is non-empty. See `7-flutter-client` open items. Interacts with `2-nearby-search` open item 3: under the wider `pending_cells` scope, a repeat re-call can dispatch further jobs.
