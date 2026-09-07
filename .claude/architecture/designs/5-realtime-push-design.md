# BEN-5 — Real-Time Push on Ingestion Completion: Final Design

**Epic:** [BEN-5](https://benchfinder.youtrack.cloud/issue/BEN-5)
**Status:** Architecture settled, four sub-issues open

---

## 1. Purpose

Under BEN-1's demand-driven model, a `/benches/nearby` request can trigger background ingestion for cells the user didn't directly query — specifically the k=2..3 cascade pre-warm ring (BEN-10). Without a push channel, the client has no way to learn that this data has arrived short of polling.

This epic covers pushing a "ready" signal to the client the moment the ingestion transaction commits.

**Scope clarification:** this is *not* about the user's own cold-start request. That path is synchronous (BEN-9) and returns real data inline. This is about the cells warmed *around* it.

---

## 2. Architecture

```
Ingestion worker (separate process)
        │
        │  upsert benches
        │  COMMIT
        ▼
  NOTIFY bench_cell_updated, '<h3_cell_id>'
        │
        │  Postgres fans out to all listeners
        ▼
┌───────────────┬───────────────┐
│ API instance  │ API instance  │  ... each holding a dedicated
│      #1       │      #2       │      long-lived LISTEN connection
└───────┬───────┴───────┬───────┘
        │               │
   check local     check local
   WS registry     WS registry
        │               │
        ▼               ▼
  push "cell X ready" to matching connections
        │
        ▼
  Client re-calls GET /benches/nearby
```

### Step by step

1. **Ingestion worker** upserts benches. On transaction commit, issues `NOTIFY bench_cell_updated, '<h3_cell_id>'`.

   Postgres fires `NOTIFY` payloads only *after* commit. This gives the "as soon as the data is actually queryable" guarantee for free — there is no window in which a client is told data is ready and then reads a snapshot that doesn't contain it.

2. **Each API instance** holds a dedicated long-lived `LISTEN` connection, kept separate from the normal query pool because it blocks waiting on notifications and must not be recycled by the pool.

   Postgres fans a `NOTIFY` out to every listening connection, so this scales horizontally across API instances without a separate pub/sub layer.

3. **Each API instance** maintains a local WebSocket registry mapping connection → H3 cell ID(s) that connection is waiting on.

4. **On NOTIFY**, the instance checks its registry and pushes a lightweight "cell X ready" message to any matching connections.

5. **The client** responds by re-calling `GET /benches/nearby`. Bench data is never duplicated over the WebSocket.

---

## 3. Settled design decisions

### Transport: WebSocket

Chosen over Server-Sent Events. Functionally SSE would suffice — the channel is strictly one-way server→client — but WebSocket was chosen deliberately for learning value.

### Cross-process signal: Postgres `LISTEN/NOTIFY`

The ingestion worker is a separate process from the API (a consequence of BEN-4's decision to run ingestion as a batch job). An in-process event bus therefore cannot carry the signal. `LISTEN/NOTIFY` uses the database already in the architecture rather than adding Redis or a broker.

### Connection lifecycle: transient

WebSocket connections are opened only when a `nearby` call returns a "pending" cache-miss state, and closed after the client receives its one push.

This is **not** a persistent connection tracking a live-panning viewport. That variant is a possible future stretch goal, but it adds meaningfully more complexity — subscription updates as the viewport moves, multi-cell tracking, heartbeats, reconnection state — for a benefit that's questionable at MVP.

### Payload: invalidation signal only

The push carries "cell X is ready," not bench data. The REST response stays the single source of truth for bench data, and the WebSocket path never needs its own serialisation format, versioning, or consistency story.

---

## 4. Dependencies

- **BEN-1 / BEN-10** — the cascade pre-warm path is what generates the events worth pushing.
- **BEN-2** — the `nearby` handler is where a "pending" state would be signalled and where the client learns to open a socket.
- **BEN-4** — the choice of Postgres provider constrains this design significantly (see below).

---

## 5. Deployment constraint worth flagging early

`LISTEN/NOTIFY` **does not survive a connection pooler running in transaction mode.** Several managed Postgres providers front connections with pgBouncer in exactly that mode by default — Supabase notably so.

This means the provider choice in BEN-4 is not free with respect to this epic. Either the provider must offer a direct (non-pooled) connection for the listener, or this architecture doesn't work as designed. This should be verified before the provider is chosen, not after.

---

## 6. Open questions / sub-issues to break out

1. **WebSocket connection lifecycle and timeout handling.** What happens if ingestion never completes — Overpass fails, the worker crashes, the job is dropped from the in-process queue on restart? The socket cannot wait forever. Needs a timeout, and a decision on what the client is told when it fires.

2. **`LISTEN` connection resilience.** What happens if the dedicated listener connection drops? Reconnect with backoff is the obvious answer, but there's a correctness gap: notifications fired during the disconnected window are lost entirely, since `LISTEN/NOTIFY` has no replay. Whether that gap needs closing, and how, is open.

3. **Registry data structure.** The per-instance connection → cell mapping. Needs to support lookup by cell on NOTIFY, and cleanup on disconnect, under concurrent access.

4. **Client-side fallback if the WebSocket fails to establish.** Poll, or fall back to pull-to-refresh and let the user drive it? The latter is more honest for a utility app and much simpler.
