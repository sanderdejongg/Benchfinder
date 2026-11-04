# `GET /benches/nearby` — Decision Log

Decisions leading to `../designs/2-nearby-search.md`.

**Updated:** 2026-11-04

---

## D1 — KNN (`<->`) over an adaptive radius ladder

**Alternatives considered:**

| Option | Why rejected |
|---|---|
| Fixed radius | Fails at both ends of the density spread. A 500 m radius returns about 80 benches in a city centre and none in a rural area. |
| Adaptive radius ladder (300 → 750 → 1500 → 2000 m, expanding until enough results) | Works, but each rung is a separate index traversal, and the ladder needs logic to decide when "enough" has been found. |

**Rationale:** The ladder treats density adaptation as control flow in application code. KNN makes it a property of the query: the GIST index walks outward from the query point and stops after N rows. The ladder and the "single max-radius query" option were both workarounds for the same constraint, and KNN removed the constraint. This superseded the minimum-results threshold and the open ladder-vs-max-radius question at the same time.

**Status:** Settled.

---

## D2 — Keep a hard `ST_DWithin` cutoff alongside KNN

**Decision:** Bound the KNN query with `WHERE ST_DWithin(geog, point, 5000)`.

**Rationale:** Two reasons.

- *Performance:* unbounded KNN in a sparse region keeps expanding the index walk looking for an Nth-nearest row. In the worst case, a query where there are no benches for tens of kilometres degrades toward a full scan. The cutoff caps the work.
- *Product correctness:* the nearest 20 benches in the middle of nowhere are 20 benches spread across half a province. They are the nearest, and they are not an answer to "where can I sit?". The cutoff encodes the judgement that beyond a certain distance the honest answer is "nothing nearby".

**Status:** Settled.

---

## D3 — 5000 m cutoff, `LIMIT` 20

**Decision:** Cutoff 5000 m. `LIMIT` is a server-side constant, default 20 (range considered: 15–20).

**Rationale:** Both values are defaults chosen for plausibility and flagged for tuning against real usage. They are not derived. 5000 m is well beyond walking distance, so it works as a backstop rather than a routine constraint. Twenty results fill a map view and a scrollable list.

**Known tension:** the cutoff reaches further than the roughly 2 km warm footprint that ingestion fills (`1-2-cell-ingestion` D9). Benches in cold cells beyond the footprint stay invisible until someone queries there. Accepted, because rural density (about 5/km²) gives about 70 benches within the 19-cell footprint. The ~2 km figure is measured from the request cell's centre; from an arbitrary point inside that cell the guaranteed covered radius is about 1.4 km (`1-2-cell-ingestion` D9).

**`LIMIT` is not client-exposed at MVP.** It can be tuned without an API contract change or a client release, and there is no clamping or abuse question to answer for a client-supplied `limit`.

**Status:** Settled. Tuning is deferred to D13.

---

## D4 — Empty result set is a 200, not a 404

**Decision:** Zero benches within the cutoff returns `200` with `"benches": []`.

**Rationale:** The request was well-formed and the query ran. "No benches near this location" is a successful answer. A `404` would mean the endpoint was not found, and would push the client into treating a normal outcome as an error.

**Status:** Settled.

---

## D5 — Deterministic tiebreak deferred

**Decision:** Ship without `, id` as a secondary sort key. Track as post-MVP.

**Rationale for taking it seriously:** when rows tie on exact distance at the `LIMIT` boundary, which tied rows fall inside the limit is unspecified. The returned set, not just its order, can differ between identical queries.

**Rationale for deferring:** exact float-distance ties between distinct benches are rare, the user-visible effect is negligible, and the fix is a two-word change against an already-indexed column.

**Status:** Settled, explicitly deferred.

---

## D6 — Validate before querying

**Decision:** Range-check `lat` and `lon` before the query is issued.

**Rationale:** An out-of-range coordinate is a client bug, not a database question. Failing fast avoids spending a connection and the 3 s budget on a query that cannot give a meaningful answer, and it keeps the error with the right layer.

**Status:** Settled.

---

## D7 — Repository seam: service depends on a repository interface, DBAL implementation behind it

**Decision:** Three layers: controller → search service → repository interface, with a DBAL implementation. The service is unit-tested against an in-memory fake.

**Alternatives considered:**

| Option | Why rejected |
|---|---|
| No interface; test the SQL directly against PostGIS | This was the leaner option and was recommended in deliberation. Not chosen, for the learning reason below. |

**Rationale:** Chosen deliberately for learning value, not because a single endpoint needs it. The seam makes the service unit-testable without a database. The SQL is still covered by integration tests against real PostGIS, so the seam does not replace those tests.

**Trade-off accepted:** this sits in tension with the project rule against speculative abstractions. The interface has one implementation today. That tension is recorded here rather than resolved.

**Status:** Settled.

---

## D8 — Database query deadline of 3 s, enforced by Postgres `statement_timeout`

**Decision:** Every search query runs under a 3 s deadline, enforced by Postgres `statement_timeout`. The deadline covers the database query only.

**Rationale:** Bounds worst-case latency for each query, and releases the connection when a query stalls rather than holding it. The request has no overall deadline: the visitor upsert and a RabbitMQ publish also run on the request path (`1-4-dispatch-mechanism`), and no Overpass call does.

**Trade-off accepted:** a query that exceeds the deadline fails rather than returning partial results. The response for that case is an open item.

**Status:** Settled.

---

## D9 — Raw OSM tags excluded from the response

**Decision:** Expose only the derived booleans (`has_backrest`, `is_covered`, `is_accessible`), coordinates and distance. The raw tag set stays internal.

**Rationale:** OSM tags are inconsistent, free-form and sometimes junk. Exposing them would leak ingestion messiness into the public contract and invite the client to interpret tag semantics. Deriving a small stable set server-side keeps the interpretation in one place.

**Status:** Settled.

---

## D10 — Bench detail sheet reads from the list cache

**Decision:** No `GET /benches/{id}` at MVP. The detail view uses the object already in the client's list-response cache.

**Rationale:** The list response already carries every field the detail sheet shows. A second round trip would fetch data the client already has. If the detail view later needs fields the list does not carry, the endpoint is added then.

**Status:** Settled.

---

## D11 — Structured error body from day one

**Decision:** Validation failures and future errors return `{"error": {"code": "<machine_code>", "message": "<human string>"}}`.

**Alternatives considered:**

| Option | Why rejected |
|---|---|
| `{"error": "string"}` | Cannot be branched on without brittle string matching. The first client that needs to tell error types apart forces a breaking change. |
| RFC 7807 Problem Details | More ceremony than a small no-login API needs. The format pays off when several services share one error contract, which does not exist here. |

**Rationale:** Starting structured costs nothing now. Adding a `code` to a bare-string shape later breaks any client that matched on the string. This convention is inherited by every future endpoint. Codes are short snake_case strings, e.g. `invalid_coordinates`.

**Status:** Settled.

---

## D12 — No `count` field in the response envelope

**Decision:** The body has `benches` and, under D15, `pending_cells`. No `count`, and no other envelope metadata.

**Rationale:** `count` is redundant with `benches.length`. Its only justification is pagination or truncation, neither of which exists. Per the project stance against speculative fields, it is added when pagination is built. `pending_cells` is not in this category: D14 requires it (D15).

**Status:** Settled, deferred.

---

## D13 — `LIMIT` and cutoff values are tuning tasks, not open design questions

**Decision:** Keep `LIMIT` 20 and cutoff 5000 m as MVP defaults. Final values are a post-launch tuning task.

**Rationale:** Neither value can be derived at design time. Both need real query patterns and density data. This is the same category as the freshness window (`1-1-h3-grid-freshness` D4). Treating "what is the right number" as unresolved design work would stall the design on a question only production traffic can answer.

**Status:** Explicitly deferred, with a named trigger: real usage data.

---

## D14 — Always answer from PostGIS; cold or stale cells are ingested asynchronously and announced by push

**Supersedes:** `1-2-cell-ingestion` D1 (and the empty-array response clause of its D2).

**Decision:** The endpoint always responds from PostGIS. A cold or stale request cell does not block the response. The cell is dispatched for ingestion and listed in `pending_cells`. The client waits for a push and then re-calls the endpoint. This replaces the earlier synchronous cold-start design (`1-2-cell-ingestion` D1, superseded). For the date of this decision, see `1-2-cell-ingestion` D7.

**Alternatives considered:**

| Option | Why rejected |
|---|---|
| Synchronous blocking cold start | The request then includes an Overpass call. With the 3 s query deadline and the 5 s Overpass timeout, worst-case latency is about 8 s, which the 3 s contract does not reflect. |
| A 3 s timeout on the nearby request, with the cold path inside it | An Overpass call cannot be bounded under 3 s. The timeout would be the real contract and the cold path would break it. |
| Push on cache miss | Left the miss response without a defined pending signal, and implied a contract that did not match the other two framings. |

**Rationale:** The three framings above implied contradictory contracts. Option B is the one that makes them coherent: the nearby response is always fast, and the push is how cells that were pending become ready. The client needs an explicit "empty now, may arrive shortly" state (`7-flutter-client` D6) for this to work.

**Trade-off accepted:** the first user in a cold area gets an empty or partial answer plus a pending signal, not inline results. This reverses the earlier argument that a first-time empty list is a bad first impression.

**Status:** Settled.

---

## D15 — Response carries `pending_cells` and a Mercure discovery `Link` header

**Decision:** The `200` body is `{ "benches": [...], "pending_cells": ["<h3 res-8 index>", ...] }`. Stale cells are served from PostGIS and listed in `pending_cells`. An empty array means everything is fresh. When non-empty, the response carries `Link: <hub-url>; rel="mercure"` (scope provisional, see design open item 2).

**Alternatives considered:**

| Option | Why rejected |
|---|---|
| One per-request topic | The server would have to track a mapping from cell to topic. |
| A boolean `pending` flag | Forces H3 cell logic into the client, which would then have to work out which cells are pending. |

**Rationale:** `pending_cells` is required by D14. It is not speculative metadata. The `count` decision (D12) still stands.

**Trade-off accepted:** the response carries H3 indexes that the client treats as opaque.

**Status:** Settled for the body; Link header scope Provisional (design open item 2).

---

## D16 — Geography through jsor/doctrine-postgis; KNN as native SQL through DBAL

**Decision:** Geography types and GIST indexes are handled with jsor/doctrine-postgis, including migrations. The KNN query (`<->` with `ST_DWithin` and `ST_Distance`) is native SQL executed through DBAL.

**Alternatives considered:**

| Option | Why rejected |
|---|---|
| Raw DBAL for everything | Schema diffing fights the unknown column type. |
| Custom DQL functions for `<->` | Most friction for a single query. |

**Rationale:** DQL cannot express `<->` without custom functions. Doctrine handles the schema and the index definitions. The one query that needs the KNN operator is written as SQL.

**Trade-off accepted:** two query styles in the codebase, DQL for ordinary data and native SQL for the KNN query.

**Status:** Settled.

---

## D17 — Build nearby search before ingestion

**Decision:** The nearby search endpoint is built first, against seeded fixtures, with no freshness hook. Ingestion is built after.

**Rationale:** The 2026-10-08 design discussion states this order and gives no further argument for it. Its stated consequence: the endpoint ships against seeded data, and the freshness hook arrives with ingestion.

**Status:** Settled.

---

## D18 — Bench `id` is a UUIDv7, generated in PHP

**Decision:** The `benches` primary key is a UUIDv7 from Symfony Uid, stored in a native Postgres `uuid` column. It is serialised as a string in the response, as the response contract already showed (`"id": "..."`). PHP generates the value, not the database: Postgres 17 has no `uuidv7()` function. That function arrives in Postgres 18, which is blocked (`4-deployment-shape` D17).

**Alternatives considered:**

| Option | Why rejected |
|---|---|
| Bigint identity column | Not argued beyond the string `id` the response contract already showed. |

**Rationale:** Recommended in the 2026-11-04 discussion and accepted. Time-ordered UUIDs keep B-tree insert locality, which random v4 does not. The string `id` matches the response contract.

**Trade-off accepted:** every insert path must supply the `id` from PHP, including ingestion's native-SQL upsert when it is built.

**Status:** Settled.

---

## D19 — Dev data is seeded with doctrine/doctrine-fixtures-bundle; integration tests seed their own rows

**Decision:**

- The bundle is a dev dependency, enabled in `dev` and `test` only, never in production.
- Fixtures load a small real OSM sample: a few dozen benches in one area. The fixture file carries the OSM/ODbL attribution, since the data is OSM-derived.
- Integration tests do not depend on the fixtures. Each test inserts the rows it needs, rolled back by dama/doctrine-test-bundle (`6-architecture-setup` D16).

**Alternatives considered:**

| Option | Why rejected |
|---|---|
| A seed console command that parses Overpass JSON | It builds ingestion's parser ahead of `1-2-cell-ingestion`, against the project rule against speculative abstractions. |

**Rationale:** Fixtures never run in production. `1-ingestion-pipeline` D1 ("no bulk seed") is about production data, so the two do not conflict.

**Status:** Settled.

---

## D20 — The first migration creates only the `benches` columns the first slice uses

**Decision:** The first migration creates these `benches` columns: `id`, `source`, `source_id`, `geog` (geography Point, SRID 4326, GIST index), `has_backrest`, `is_covered`, `is_accessible`, `created_at` and `updated_at`. Uniqueness is on `(source, source_id)` (`1-ingestion-pipeline` D3). The `h3_index` and `tags` columns are added by the ingestion migration. The nullability of the three booleans is not part of this decision (design open item 4).

**Rationale:** Search reads neither `h3_index` nor `tags`. `h3_index` also brings a column-type choice (h3-pg's `h3index` type or a bigint) that belongs to ingestion. The project rule is to build nothing before it is needed.

**Trade-off accepted:** a later migration alters `benches`.

**Status:** Settled.

---

## D21 — A KNN statement that hits `statement_timeout` returns `503` with code `search_unavailable`

**Decision:** A KNN statement cancelled by the 3 s `statement_timeout` (D8) returns `503` with error code `search_unavailable`. The timeout is detected as Postgres SQLSTATE `57014` (query_canceled). The body follows D11.

**Rationale:** Recommended in the discussion as the part of open item 1 that this slice reaches, and accepted. A `503` marks a server-side condition that can clear on retry, not a client error. Alternatives were not argued.

**Scope:** The decision covers the timeout only. Other KNN failures (for example connection loss), dispatch failure and visitor-upsert failure remain open (design open item 1).

**Status:** Settled.

---

## D22 — The visitor upsert is not part of the first slice

**Decision:** The first slice builds request flow steps 1, 3 and 5 only.

- Step 4, the freshness check, is out (D17). `pending_cells` is therefore always `[]` in the first slice, and the `Link` header is never sent.
- Step 2, the visitor upsert, is out. The `X-Visitor-Id` header is ignored until the upsert lands.

**Rationale:** D17 excluded only the freshness hook. The upsert brings the `visitor` table from `3-visitor-identity` and a failure path that is still undecided (`3-visitor-identity` open item 1). The upsert ships as its own follow-up.

**Trade-off accepted:** the first slice's requests record no visitor rows.

**Status:** Settled.

---

## D23 — Amenity booleans are nullable; null means the tag is absent

**Resolves:** design open item 4 (nullability only).

**Decision:** `has_backrest`, `is_covered` and `is_accessible` are nullable, in the `benches` columns and in the response. Each is `true`, `false` or `null`. `null` means OSM carries no usable tag for it, so the value is unknown.

**Alternatives considered:**

| Option | Why rejected |
|---|---|
| Non-null, defaulting to `false` | A missing tag is reported as "no", which is often wrong. For accessibility it is actively misleading: a bench with no `wheelchair` tag is not known to be inaccessible. |

**Rationale:** Recommended in the 2026-11-04 discussion and accepted. OSM often omits these tags. The response should say what the data says, and the client shows "unknown" for `null`.

**Trade-off accepted:** the client handles three states per amenity instead of two (`7-flutter-client`).

**Status:** Settled.

---

## Unresolved at time of writing

- Response on a KNN failure other than the timeout (for example connection loss), on dispatch failure (RabbitMQ unreachable) or on visitor-upsert failure: status and error codes not chosen (design open item 1). The timeout is settled by D21.
- `Link` header scope on `200` (design Open item 2).
- Scope of the freshness check, and therefore of `pending_cells` (design Open item 3).
- Which OSM tag values map to `true`, `false` and `null` for the three amenity booleans (design open item 4). Nullability is settled by D23.
