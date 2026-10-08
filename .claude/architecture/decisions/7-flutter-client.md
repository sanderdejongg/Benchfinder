# Flutter Client — Decision Log

Decisions leading to `../designs/7-flutter-client.md`.

**Updated:** 2026-10-08

---

## D1 — `flutter_map` over `google_maps_flutter`

**Decision:** Use flutter_map for the map.

**Alternatives considered:**

| Option | Why rejected |
|---|---|
| `google_maps_flutter` | Billing and API-key friction. Avoidable here. |

**Rationale:** Avoids billing and API-key setup. The choice is reversible and confined to the presentation layer, so it can be changed later without touching the data flow.

**Status:** Settled.

---

## D2 — Riverpod with AsyncNotifier and FutureProvider.family

**Decision:** Riverpod for state. Asynchronous state is held in AsyncNotifier. Parameterised reads (for example, nearby results by coordinate) use FutureProvider.family.

**Alternatives considered:** none evaluated in the 2026-10-08 design discussion.

**Rationale:** Riverpod was the stated leaning in the earlier client planning, and the 2026-10-08 design discussion records no further argument. AsyncNotifier fits the request-then-render shape. FutureProvider.family fits reads keyed on a coordinate.

**Status:** Settled.

---

## D3 — Visitor ID stored in `flutter_secure_storage`

**Decision:** The client's visitor ID is stored with flutter_secure_storage.

**Rationale:** Backup and restore semantics. Keychain and Keystore storage does not restore through cloud backup, so uninstalling ends the identity. Full reasoning and alternatives: `3-visitor-identity` D4.

**Status:** Settled.

---

## D4 — geolocator for location

**Decision:** Use geolocator to obtain the device location.

**Alternatives considered:** none evaluated. Chosen as part of the stack.

**Status:** Settled.

---

## D5 — Hand-rolled SSE client over `package:http`

**Decision:** The push stream is a hand-rolled client over `package:http`. It sends a streamed GET, reads the first `data:` event, and closes the stream. There is no reconnect logic. The stream is transient by design (`5-realtime-push` D4).

**Alternatives considered:**

| Option | Why rejected |
|---|---|
| Existing SSE packages from pub.dev | Maintenance quality varies across available packages, and most bundle reconnect machinery a transient connection does not need. |

**Rationale:** Learning Dart Streams is a project goal, and this is a small, contained use of them. A transient stream needs no reconnect machinery. Avoids a dependency on a package whose maintenance quality varies.

**Trade-off accepted:** no reconnect. A dropped stream falls back to pull-to-refresh, which is the designed fallback (`5-realtime-push` D8).

**Status:** Settled.

---

## D6 — Explicit "empty now, may arrive shortly" state, distinct from loading

**Decision:** When the response has no benches and a non-empty `pending_cells`, the UI shows a pending-empty state of its own. When the response has benches and a non-empty `pending_cells`, the UI shows the benches with a pending notice. Neither state is a spinner. The fallback is pull-to-refresh after the 15 s timeout.

**Alternatives considered:**

| Option | Why rejected |
|---|---|
| A loading spinner while pending | A spinner says the wait ends with results. A pending area can end with none, and the user should be told that. |

**Rationale:** Under the pending model (`2-nearby-search` D14), a pending response is a valid answer that may change. The client has to show that without implying an error or an endless wait.

**Status:** Settled. Wording of the messages is an open item.

---

## D7 — `riverpod_lint` and `custom_lint` in scope

**Decision:** Add riverpod_lint, with custom_lint, to the client's analysis setup.

**Rationale:** The lint packages are useful once the dependency exists. Riverpod is now adopted (D2). This supersedes the deferral in `6-architecture-setup` D7.

**Status:** Settled.

---

## Unresolved at time of writing

- Re-call policy after a push (design Open item 1).
- Permission-denied path (design Open item 2).
- Stream handling under `package:http` (design Open item 3).
- Pending-state copy (design Open item 4).
