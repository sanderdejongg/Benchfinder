# Flutter Client — Final Design

**Issue:** none yet  **Status:** settled, not implemented  **Updated:** 2026-10-08

---

## Purpose

The Flutter app for iOS and Android. It finds the user's location, asks the backend for nearby benches, shows them on a map and in a list, and handles the pending state, where the backend says ingestion is still running for the area and results may arrive by push.

No accounts, no login. The app keeps one anonymous visitor ID (`3-visitor-identity`).

---

## Constraints

- **Pending is a normal state, not an error.** A cold area can answer with no benches and a non-empty `pending_cells` list (`2-nearby-search` D14). The UI must show "empty now, may arrive shortly" as its own state (D6).
- **Push is short-lived.** The client holds one stream, for at most 15 s, and closes it after one event (`5-realtime-push`).
- **Attribution.** OSM/ODbL attribution is shown in-app on a settings or about screen (`1-ingestion-pipeline` D5). Map tile attribution follows the tile provider's requirements.
- **Mobile only at MVP.** Flutter web is deferred (see Explicitly deferred).

---

## Design

### Libraries

| Concern | Choice | Decision |
|---|---|---|
| Map | flutter_map | D1 |
| State | Riverpod: AsyncNotifier, FutureProvider.family | D2 |
| Visitor ID storage | flutter_secure_storage | D3 |
| Location | geolocator | D4 |
| Push stream | Hand-rolled SSE client over package:http | D5 |
| Lint | very_good_analysis; riverpod_lint with custom_lint | `6-architecture-setup` D5; D7 below |
| Format | dart format | `6-architecture-setup` D6 |
| Tests | flutter_test; mocktail for repository mocks | `6-architecture-setup` D11 |

### Request flow

1. Obtain the user's location with geolocator.
2. Call `GET /benches/nearby?lat=&lon=` with the `X-Visitor-Id` header (`2-nearby-search`, `3-visitor-identity`).
3. Render the `benches` list and map markers at once.
4. If `pending_cells` is empty: done.
5. If `pending_cells` is non-empty: show the pending state and open one SSE stream to the hub URL from the `Link` header, subscribed to the pending cell topics. Start a 15 s timer.
6. On one event: close the stream and re-call the endpoint once, then render the result.
7. On timeout, stream failure, or no `Link` header: close the stream and fall back to pull-to-refresh.

### UI states

| State | Shown when | Notes |
|---|---|---|
| Loading | First request in flight | Spinner. Not the pending state |
| Results | `benches` non-empty, `pending_cells` empty | Map and list |
| Pending | `pending_cells` non-empty | Shows whatever benches came back, with a notice that more may arrive shortly. Not a spinner |
| Empty, final | `benches` empty, `pending_cells` empty | "No benches near you" |
| Empty, pending | `benches` empty, `pending_cells` non-empty | A message that the area is being checked and results may arrive. Distinct from a loading spinner (D6). Wording open (Open item 4) |
| Error | Network failure or non-`200` response | Shows the error code from the error body where present |
| Timed out | 15 s passed with no event | Pull-to-refresh offered. Nothing is broken (`5-realtime-push` D10) |

### Permissions

Location permission is needed before the first request. A denied permission has no designed path yet (Open items).

### Identity

The visitor ID is generated once on first launch, stored in flutter_secure_storage, and sent on every request (`3-visitor-identity` D3, D4, D5). It is never shown to the user.

---

## Contracts & data model

Consumes:

- `GET /benches/nearby` response: `benches` array and `pending_cells` array (`2-nearby-search`).
- `Link: <hub-url>; rel="mercure"` response header.
- Mercure SSE stream: one event, a set of ready cell indexes, no bench data (`5-realtime-push`).
- Error body: `{"error": {"code", "message"}}` (`2-nearby-search` D11).

Client-side state: no local bench database. The list response is cached in memory for the detail sheet (`2-nearby-search` D10).

---

## Dependencies

- `2-nearby-search`: response shape, pending contract, error body.
- `3-visitor-identity`: ID generation, storage, header.
- `5-realtime-push`: stream, topics, timeout, fallback.
- `6-architecture-setup`: lint, format and test tooling.

---

## Explicitly deferred

- **Flutter web target.** Architecturally possible, deliberately postponed. CORS and browser storage differ from the keychain model in D3.
- **Persistent viewport-tracking push.** A stretch goal (`5-realtime-push`).

---

## Superseded

- Riverpod lint packages deferred until Riverpod is adopted: superseded by D7 here, since Riverpod is adopted (`6-architecture-setup` D7).

---

## Open items

1. **Re-call policy.** After one push, the client re-calls once. Whether it repeats while `pending_cells` is still non-empty is not decided. Interacts with `2-nearby-search` open item 3: under the wider `pending_cells` scope, a repeat re-call can dispatch further jobs.
2. **Permission-denied path.** No designed UI for a denied or restricted location permission.
3. **Stream handling under package:http.** Reading a streamed response, and closing it cleanly, has not been verified against the chosen package.
4. **Pending-state copy.** Exact wording of the two pending messages is not written.
