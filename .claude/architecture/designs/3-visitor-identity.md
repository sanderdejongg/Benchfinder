# Anonymous Visitor Identity — Final Design

**Status:** Design settled, two placement/mechanism questions open

---

## 1. Purpose

Benchfinder has no accounts by design. But "no accounts" doesn't have to mean "every request is fully anonymous." The app generates a persistent anonymous identifier on first launch and sends it with API requests, so the backend can recognise one installation across time without any registration step.

At MVP this is **pure plumbing**. Nothing user-facing depends on it. It exists so that favorites, contribution history, and similar per-install features can be built later without a retroactive identity migration.

---

## 2. Privacy framing

This is stated first rather than last, because it constrains the rest of the design.

A persistent anonymous ID is functionally a **device fingerprint**, login or no login. Combined with the location data this app necessarily handles, it is capable of reconstructing a location history tied to a stable identifier. That capability is not wanted and is designed against.

The ID's purpose is scoped narrowly and explicitly: **distinguish this installation for feature purposes.** Not analytics. Not monetization. Location-data monetization tied to an identifier of this kind was evaluated as a business direction and shelved — GDPR purpose-limitation makes it legally awkward, and app-store policy makes it commercially risky. The ID must not be quietly repurposed toward it later.

---

## 3. Settled design

### Generation

UUIDv4, generated **once on first app launch**, client-side. The server never issues or assigns an ID.

### Client storage

`flutter_secure_storage` — Keychain on iOS, Keystore on Android.

Deliberately **not** `SharedPreferences` / `UserDefaults`. Those participate in automatic cloud backup, which would mean a reinstall silently resurrects the old identity from an iCloud or Google backup. Secure-storage-backed values don't restore that way, so an uninstall genuinely ends that identity and a reinstall starts a new one. This is the desired semantics: uninstalling is a meaningful reset.

### Transport

Custom header on every request:

```
X-Visitor-Id: <uuid-v4>
```

Not a query parameter (would land in access logs and URL history). Not a cookie (wrong model for a native client, and drags in cookie semantics nobody wants). Not folded into `Authorization` (this is not authentication and must not be mistaken for it by any middleware, proxy, or future reader of the code).

### Backend middleware

1. Read `X-Visitor-Id`.
2. Validate it parses as a UUID.
3. If valid, attach to the request context.
4. If missing or malformed, **proceed anyway.**

The middleware **never rejects a request** on the basis of the visitor ID. Core functionality — finding nearby benches — must work with zero identity present. A client that sends nothing gets the same benches as one that sends a valid ID.

### Storage

`Visitor` table, upsert on request. Minimal columns at MVP:

```
visitor
  id          -- the UUIDv4
  first_seen
  last_seen
```

No IP address. No location. No device details. Nothing else joined to this table.

### Retention

Visitor rows are purged after **12 months of inactivity** (`last_seen` older than 12 months). This is a decided policy number. The enforcement mechanism — a scheduled job — is not yet built.

### Logging policy

**Visitor ID and precise location must never appear in the same log line.**

This is the operational control that keeps the ID from becoming a location-history key. Either datum alone is comparatively harmless; correlated in a log stream, they reconstitute exactly the tracking capability the design is trying to avoid.

Request logs that do contain the visitor ID fall under the same 12-month retention policy as the `Visitor` table itself. Retention that only covers the database while logs keep everything indefinitely is not retention.

---

## 4. MVP behaviour summary

| Aspect | MVP state |
|---|---|
| Generated and persisted client-side | Yes |
| Sent on every request | Yes |
| Validated and attached to context | Yes |
| Upserted to `Visitor` table | Yes |
| Returned to the client by any endpoint | No |
| Any feature branches on it | No |
| Any analytics job consumes it | No |
| Retention job running | Not yet built |

---

## 5. Planned sub-tasks

- **Client:** generate + persist UUIDv4 via `flutter_secure_storage`
- **Client:** attach `X-Visitor-Id` header to all API requests
- **Backend:** visitor ID validation middleware
- **Backend:** `Visitor` table + upsert-on-request logic
- **Ops:** enforce 12-month retention for the `Visitor` table and related logs

---

## 6. Deferred

Features this ID unlocks — favorites, contribution history — are separate topics to be scoped when they're actually wanted. The point of this design is that when that day comes, the identity layer already exists and already has a defensible privacy posture, rather than being bolted on under feature pressure.

---

## 7. Open questions

1. **Synchronous vs. deferred upsert in middleware.** Should the `Visitor` upsert happen inline on the request path, or be pushed onto a background channel?
   - *Synchronous:* simple, correct, but puts a write on the critical path of a read endpoint whose latency budget is already partly consumed by the ingestion pipeline's cold-start path.
   - *Deferred:* keeps the read path clean, but introduces a queue, a dropped-write failure mode, and the question of what happens on shutdown.
   - Note that the in-process worker pool from the ingestion pipeline's dispatch mechanism already exists as a possible home for this.

2. **Placement of the ops retention work.** Whether it belongs here now, or should wait on the deployment-shape design — since the enforcement mechanism (cron job, scheduled machine, GitHub Actions workflow) depends entirely on which hosting platform is chosen, and that decision is still open.
