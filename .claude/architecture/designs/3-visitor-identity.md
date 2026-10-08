# Anonymous Visitor Identity — Final Design

**Issue:** [BEN-3](https://benchfinder.youtrack.cloud/issue/BEN-3)  **Status:** settled, not implemented  **Updated:** 2026-10-08

---

## Purpose

Benchfinder has no accounts by design. The app still generates a persistent anonymous identifier on first launch and sends it with API requests, so the backend can recognise one installation over time without any registration step.

At MVP this is plumbing. Nothing user-facing depends on it. It exists so that favorites, contribution history and similar per-install features can be built later without a retroactive identity migration.

---

## Privacy framing

Stated first because it constrains the rest of the design.

A persistent anonymous ID is functionally a device fingerprint, with or without login. Combined with the location data this app handles, it could reconstruct a location history tied to a stable identifier. That capability is not wanted and is designed against.

The ID's purpose is scoped narrowly: distinguish this installation for feature purposes. Not analytics, not monetisation. Location-data monetisation tied to an identifier of this kind was evaluated and shelved, on GDPR purpose-limitation and app-store policy grounds. The ID must not be repurposed toward it later.

---

## Constraints

- The ID must never be the reason a request fails (D6).
- Visitor ID and precise location must never be co-logged (D7). At MVP, the visitor ID is never logged at all (D13).
- Retention: visitor rows are purged after 12 months of inactivity (D9).
- Access logs must omit the `X-Visitor-Id` header (a Caddy log filter deleting `request>headers>X-Visitor-Id`), or access logging must be off. Without this, the no-logging rule (D13) does not hold, since Caddy logs request headers.
- The upsert is synchronous on the request path (D11). Whether a failed upsert fails the request sits against the never-reject rule (D6). That question is open item 1.

---

## Design

### Generation

UUIDv4, generated once on first app launch, client-side. The server never issues or assigns an ID.

### Client storage

`flutter_secure_storage`: Keychain on iOS, Keystore on Android. Not `SharedPreferences` or `UserDefaults`, which take part in automatic cloud backup and would let a reinstall silently restore the old identity (D4). Uninstalling is a meaningful reset.

### Transport

Custom request header on every API request:

```
X-Visitor-Id: <uuid-v4>
```

Not a query parameter (it would land in access logs and URL history). Not a cookie (wrong model for a native client). Not folded into `Authorization` (this is not authentication, and must not be read as one).

### Backend handling

1. Read `X-Visitor-Id`.
2. Validate it parses as a UUID.
3. If valid, attach it to the request context.
4. If missing or malformed, proceed anyway.

The handling never rejects a request on the basis of the visitor ID. A client that sends nothing gets the same benches as one that sends a valid ID.

### Storage

The `visitor` table is upserted on every request. This is synchronous, inline on the request path (D11).

### Retention

Visitor rows are purged when `last_seen` is older than 12 months. The purge is a Symfony Scheduler recurring message, consumed by the existing messenger worker (D14). It needs no CI secret and no network path from outside the application.

### Logging policy

The visitor ID is never logged: not in Monolog output, not in access logs (D13). This makes the rule "visitor ID and precise location never share a log line" hold provided the access log is configured to omit the `X-Visitor-Id` request header, for example a Caddy log filter deleting `request>headers>X-Visitor-Id`, or access logging is switched off. Caddy's JSON access log records request headers and the full URI, and the URI carries lat/lon, so the filter is a configuration requirement tracked in `4-deployment-shape` open item 5. Precise location may still appear in request logs, because the nearby request carries coordinates; the visitor ID does not appear in them.

### Mercure

Mercure subscriptions carry a res-8 H3 cell (about 0.74 km²), never a precise location and never the visitor ID (`5-realtime-push`).

---

## Contracts & data model

```
visitor
  id          -- the UUIDv4
  first_seen
  last_seen
```

No IP address, no location, no device details, no user agent. Nothing else joined to this table.

Header: `X-Visitor-Id: <uuid-v4>`, optional on every request.

Purge: rows where `last_seen < now() - interval '12 months'`.

---

## Dependencies

- `2-nearby-search`: the endpoint that carries the optional header.
- `4-deployment-shape`: the Symfony Scheduler and messenger worker the purge runs on.
- `5-realtime-push`: the subscription rule above.
- `7-flutter-client`: client generation, storage and header.

---

## Explicitly deferred

- **Features this ID unlocks**, such as favorites and contribution history. Scoped separately when wanted. The point of this design is that the identity layer and its privacy posture already exist when that day comes.
- **Analytics and any use of the ID beyond feature purposes.** A new design with its own privacy review is required.

---

## Superseded

- Monthly GitHub Actions retention workflow: replaced by Symfony Scheduler (D14).
- Open item "log retention enforcement": resolved by "the visitor ID is never logged" (D13).

---

## Open items

1. **Visitor-upsert failure.** If the upsert fails, the request either fails (against the never-reject rule, D6) or proceeds without recording the visitor. Not decided. Shares the failure-path question in `2-nearby-search` open item 1.

The log retention enforcement item is resolved by D13 (2026-10-08), subject to the access-log requirement above.
