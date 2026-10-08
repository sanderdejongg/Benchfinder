# Anonymous Visitor Identity — Decision Log

Decisions leading to `../designs/3-visitor-identity.md`.

**Updated:** 2026-10-08

---

## D1 — Have an identity at all, despite "no accounts"

**Decision:** Generate a persistent anonymous visitor ID, even though the product has no accounts, no login and no social features.

**The tension:** the product's stated philosophy is utility first, social never (at MVP). An identity layer looks like the first step toward accounts.

**Rationale:** "No accounts" describes the user-facing surface: no signup, no email, no password, no profile. It does not mean the backend must be unable to tell two installations apart. Features that are clearly in scope for a utility app, such as favorites and a record of your own contributions, need exactly that capability and nothing more.

Building it now as inert plumbing is far cheaper than retrofitting identity onto a live app. Retrofitting means losing existing users' state or writing a migration for data that was never keyed.

**Guard against drift:** at MVP nothing returns the ID, nothing branches on it, and no analytics consumes it. The plumbing is present and unused by design (D10).

**Status:** Settled.

---

## D2 — Privacy posture defined before mechanism

**Decision:** Treat the ID as a device fingerprint and scope its purpose narrowly, in writing, before deciding any implementation detail.

**Rationale:** A persistent anonymous ID plus location data is functionally a tracking system. The absence of a name or email does not change that. Naming it up front makes the downstream decisions (D4, D6, D7) legible rather than arbitrary.

**Related prior decision carried in:** location-data monetisation was evaluated as a business model and shelved, on GDPR purpose-limitation grounds (data collected to show benches cannot be repurposed for sale) and app-store policy risk. Recorded so the ID is not quietly repurposed by someone who does not know the question was already answered.

**Status:** Settled.

---

## D3 — UUIDv4, generated client-side

**Decision:** The client generates a UUIDv4 on first launch. The server never issues one.

**Alternatives considered:**

| Option | Why rejected |
|---|---|
| Server-issued ID returned on first contact | Needs a bootstrap round trip and server state before first use. |
| Derived device identifier (IDFV, Android ID) | Platform identifiers can be correlated across apps and are subject to platform policy changes. |

**Rationale:** Client generation needs no bootstrap and no server state. A random UUIDv4 carries no information about the device, and the server has no basis to link two IDs even if it wanted to.

**Status:** Settled.

---

## D4 — `flutter_secure_storage`, explicitly not `SharedPreferences`

**Decision:** Persist the UUID in `flutter_secure_storage` (Keychain on iOS, Keystore on Android).

**Rationale:** The choice is about backup and restore semantics, not about protecting the value from other apps. `SharedPreferences` and `UserDefaults` take part in automatic cloud backup, so an identifier stored there survives an uninstall. A user who deletes the app and reinstalls months later would silently get the old identity back. Keychain- and Keystore-backed storage does not restore that way, so uninstalling genuinely ends the identity.

**Trade-off accepted:** favorites, a future feature, would not survive a reinstall. That is the correct side of the trade for an app with no account to sync them to.

**Status:** Settled.

---

## D5 — Custom `X-Visitor-Id` header

**Decision:** Transmit the ID in a dedicated request header.

**Alternatives considered:**

| Option | Why rejected |
|---|---|
| Query parameter | Lands in access logs, proxy logs and referrer chains by default. |
| Cookie | Wrong model for a native client. Brings cookie expiry and same-site rules for no benefit. |
| Folded into `Authorization` | This is not authentication. Putting it there invites middleware, proxies and future readers to treat it as a credential. |

**Rationale:** A distinct header keeps identity semantically separate from authentication and keeps the value out of URLs.

**Status:** Settled.

---

## D6 — Never reject a request on a missing or malformed ID

**Decision:** Validate the header format. Attach the ID to the request if valid. Proceed regardless. Never `401` or `400` on the visitor ID.

**Rationale:** If a missing ID could fail a request, the ID would be a credential in everything but name, and the "no accounts" promise would be hollow. Finding a nearby bench must work for a client that sends no header: a curl request, a web client where secure storage behaves differently, or a user whose Keystore entry was wiped.

**Status:** Settled.

---

## D7 — Visitor ID and precise location are never co-logged

**Decision:** The two must not appear in the same log line.

**Rationale:** This is the operational control that makes the privacy framing real. Neither datum alone is especially sensitive. Correlated in a log stream over time, they reconstruct a location history keyed to a persistent identifier, which is the capability the design exists to avoid. The rule is stated as an absolute because it erodes through debug logging, one line at a time. With D13 the rule is also satisfied, provided the access log is configured to omit the `X-Visitor-Id` request header (`4-deployment-shape` open item 5) or access logging is off.

**Status:** Settled.

---

## D8 — Minimal `visitor` schema: `(id, first_seen, last_seen)`

**Decision:** Three columns. No IP, no location, no device details, no user agent.

**Rationale:** Data minimisation as a schema constraint rather than a policy. Anything not stored cannot leak, be subpoenaed, be joined by accident, or be repurposed. `first_seen` and `last_seen` are the minimum for the retention policy in D9. `last_seen` exists for retention, not analytics.

**Status:** Settled.

---

## D9 — 12-month inactivity-based retention

**Decision:** Purge `visitor` rows where `last_seen` is older than 12 months.

**Rationale:** Inactivity-based rather than creation-based. A continuously active installation keeps its state indefinitely, and abandoned identities age out on their own. Twelve months covers seasonal usage and is still a real limit.

**Log clause:** the original decision also put request logs containing the visitor ID under the same window. That clause is moot, because the visitor ID is no longer logged (D13).

**Status:** Settled. The number is decided. The purge job is specified under D14.

---

## D10 — Zero user-facing surface at MVP

**Decision:** No endpoint returns the ID. No feature branches on it. No analytics consumes it.

**Rationale:** Keeps MVP scope honest and gives a clear test for scope creep. If any of those three become true, that is a new design with its own privacy review, not an incremental change to plumbing.

**Status:** Settled.

---

## D11 — Visitor upsert is synchronous, inline on the request path

**Decision:** The `visitor` upsert happens in the same request that validates the visitor ID. It is not deferred to a background queue.

**Alternatives considered:**

| Option | Why rejected |
|---|---|
| Deferred upsert through the background queue | Adds a queue hop, a drop-on-shutdown risk, and coupling to the ingestion job path. That is real cost for a write this cheap. |

**Rationale:** A single upsert on an indexed primary key is negligible next to the request's other cost, which is the database query, each statement capped at 3 s (`2-nearby-search` D8). The write never silently drops. The original rationale also cited a cold-start Overpass fetch on the request path; that call has moved to the background (`1-2-cell-ingestion` D7), so the comparison now uses the database budget instead.

**Status:** Settled.

---

## D12 — Retention job as a scheduled GitHub Actions workflow

**Decision (original):** the 12-month purge runs as a GitHub Actions `on: schedule` workflow, monthly, connecting directly to the managed Postgres with a connection-string secret.

**Alternatives considered:** not recorded separately at the time.

**Rationale (original):** decoupled from the API hosting platform, because the job only needs network access to the database. GitHub Actions was also the scheduling mechanism the ingestion pipeline was expected to use.

**Status:** ⚠ Superseded by D14 (2026-10-08).

---

## D13 — The visitor ID is never logged

**Decision:** The visitor ID does not appear in any log: not in application logs, not in Monolog output, not in access logs.

**Alternatives considered:**

| Option | Why rejected |
|---|---|
| Log the ID and retain logs for 12 months, matching the database | Needs a log retention and purge mechanism that was not designed anywhere in the architecture. The open item was never closed. |

**Rationale:** With the ID absent from logs, log retention is moot, and the rule "visitor ID and precise location never share a log line" (D7) is satisfied provided the access log is configured to omit the `X-Visitor-Id` request header (a Caddy log filter deleting `request>headers>X-Visitor-Id`), or access logging is off. The filter is a configuration requirement, not a structural guarantee: Caddy's JSON access log records request headers and the full URI, and the URI carries lat/lon.

**Trade-off accepted:** request logs cannot be correlated to one visitor when debugging.

**Status:** Settled. Resolves the open item "log retention enforcement".

---

## D14 — 12-month purge on Symfony Scheduler, run by the existing worker

**Decision:** The `visitor` purge is a Symfony Scheduler recurring message, consumed by the same messenger worker that runs ingestion jobs. This supersedes D12.

**Alternatives considered:**

| Option | Why rejected |
|---|---|
| GitHub Actions scheduled workflow (D12) | Requires a connection-string secret for the production database in CI. The Scheduler approach needs no such secret. |

**Rationale:** Scheduler is a newer Symfony component, and learning it is a project goal. The purge runs where the database is already reachable, so no CI secret with database access is needed.

The purge is scheduled from day one while ingestion stays manual because the two risk profiles differ. A monthly DELETE of rows inactive for 12 months is trivial and low-blast-radius. A re-poll sweep against Overpass has rate-limit exposure and failure modes that are poorly understood. Idempotency makes automating ingestion safe, but not yet warranted.

**Trade-off accepted:** the purge runs only while the messenger worker is up.

**Status:** Settled.

---

## Unresolved at time of writing

- Visitor-upsert failure versus the never-reject rule (design open item 1).
- Log retention enforcement is resolved by D13, subject to the access-log header filter (`4-deployment-shape` open item 5).
