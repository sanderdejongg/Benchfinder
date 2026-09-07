# BEN-3 — Anonymous Visitor Identity: Decision Log

Decisions leading to the design in `BEN-3-visitor-identity-design.md`.

---

## D1 — Have an identity at all, despite "no accounts"

**Decision:** Generate a persistent anonymous visitor ID, even though the product philosophy is explicitly no-accounts, no-login, no-social.

**The tension:** Benchfinder's stated philosophy is "excellent utility first, social features never (at MVP)." An identity layer looks like the first step down the road that ends in accounts.

**Rationale:** "No accounts" is a statement about *user-facing* product surface — no signup, no email, no password, no profile. It isn't a statement that the backend must be incapable of distinguishing two installations. Future features that are clearly in scope for a utility app (favorites, a record of your own contributions) require exactly that capability and nothing more.

Building it now, as inert plumbing, is far cheaper than retrofitting identity onto a live app later — retrofitting means either losing all existing users' state or writing a migration for data that was never keyed.

**Guard against drift:** the epic states explicitly that at MVP nothing returns it, nothing branches on it, and no analytics consumes it. The plumbing is present and unused, by design.

---

## D2 — Privacy posture defined before mechanism

**Decision:** Treat the ID as a device fingerprint and scope its purpose narrowly, in writing, before deciding any implementation detail.

**Rationale:** A persistent anonymous ID plus location data is, functionally, a tracking system — the absence of a name or email doesn't change that. Naming this honestly up front is what makes the downstream decisions (D4, D6, D7) legible instead of arbitrary.

**Related prior decision carried in:** location-data monetization was evaluated as a business model and shelved, on two grounds: GDPR purpose-limitation (data collected to show you benches cannot be repurposed for sale), and app-store policy risk. The epic records this so that the ID isn't quietly repurposed later by someone who doesn't know the question was already asked and answered.

---

## D3 — UUIDv4, generated client-side

**Decision:** Client generates a UUIDv4 on first launch. Server never issues one.

**Alternatives considered:**
- Server-issued ID returned on first contact.
- A derived device identifier (IDFV, Android ID).

**Rationale:** Client generation needs no bootstrap round trip and no server state before first use. UUIDv4 is random rather than derived, so it carries no information about the device — unlike platform identifiers, which can be correlated across apps and are subject to platform policy changes. Random generation also means the server has no basis to link two IDs even if it wanted to.

---

## D4 — `flutter_secure_storage`, explicitly not SharedPreferences

**Decision:** Persist the UUID in `flutter_secure_storage` (Keychain / Keystore).

**Rationale — and this is the interesting one:** the choice is *not* primarily about protecting the value from other apps. It's about **backup and restore semantics.**

`SharedPreferences` / `UserDefaults` participate in automatic cloud backup. An identifier stored there survives an uninstall: the user deletes the app, reinstalls it months later, and iCloud or Google Backup silently restores the old identity. That makes uninstalling a no-op as a privacy action, which is the wrong default for an identifier the user never consented to in the first place.

Keychain/Keystore-backed storage doesn't restore that way. Uninstall genuinely ends the identity; reinstall starts fresh. The user gets a meaningful reset mechanism without needing a "reset my ID" setting.

**Trade-off accepted:** favorites (a future feature) would not survive a reinstall. That's the correct side of the trade for an app with no account to sync them to anyway.

---

## D5 — Custom `X-Visitor-Id` header

**Decision:** Transmit via a dedicated request header.

**Alternatives considered:**

| Option | Why rejected |
|---|---|
| Query parameter | Lands in access logs, proxy logs, and referrer chains by default — actively works against D7's logging policy. |
| Cookie | Wrong model for a native client; imports cookie semantics, expiry rules, and same-site concerns for no benefit. |
| Folded into `Authorization` | **This is not authentication.** Putting it there invites middleware, proxies, and future readers of the code to treat it as a credential — which is precisely the confusion D6 is trying to prevent. |

**Rationale:** A distinct header keeps identity semantically separate from auth semantics, and keeps the value out of URLs.

---

## D6 — Middleware never rejects on a missing or malformed ID

**Decision:** Validate the header format; attach to context if valid; **proceed regardless**. Never 401, never 400, on the visitor ID.

**Rationale:** This is the decision that keeps the ID from silently becoming authentication. If a missing ID could fail a request, then the ID is a credential in everything but name, and the "no accounts" promise is hollow.

Concretely: finding a nearby bench must work for a client that sends no header at all — a curl request, a future web client where secure storage behaves differently, a user whose Keystore entry got wiped. The core utility never depends on being identifiable.

---

## D7 — Visitor ID and precise location must never be co-logged

**Decision:** A hard rule — the two must not appear in the same log line.

**Rationale:** This is the operational control that makes the privacy framing real rather than aspirational. Neither datum alone is especially sensitive: a random UUID with no location is meaningless, and a coordinate with no identifier is a dot on a map. Correlated in a log stream over time, they reconstitute a complete location history keyed to a persistent identifier — the exact capability the design exists to avoid.

The rule is stated as an absolute rather than a guideline because it's the kind of thing that erodes through well-meaning debug logging one line at a time.

---

## D8 — Minimal `Visitor` schema: `(id, first_seen, last_seen)`

**Decision:** Three columns. No IP, no location, no device details, no user agent.

**Rationale:** Data minimisation as a schema constraint rather than a policy. Anything not stored cannot leak, cannot be subpoenaed, cannot be accidentally joined, and cannot be repurposed. `first_seen` and `last_seen` are the minimum needed to make the retention policy in D9 enforceable — and notably, `last_seen` exists *for* retention, not for analytics.

---

## D9 — 12-month inactivity-based retention

**Decision:** Purge `Visitor` rows where `last_seen` is older than 12 months. Request logs containing the visitor ID fall under the same window.

**Rationale:** Inactivity-based rather than creation-based, so a continuously active installation keeps its favorites indefinitely while abandoned identities age out on their own. 12 months is long enough not to break seasonal usage patterns, short enough to be a genuine limit.

**Extending it to logs specifically:** retention that covers the database while logs retain the same identifiers forever isn't retention. Stating both together closes that gap.

**Status:** the number is decided; the enforcement job is not built.

---

## D10 — Zero user-facing surface at MVP

**Decision:** No endpoint returns the ID. No feature branches on it. No analytics consumes it.

**Rationale:** Keeps the MVP scope honest and gives a clear test for scope creep — if any of those three become true, that's a new epic with its own privacy review, not an incremental change to plumbing.

---

## Unresolved at time of writing

**Synchronous vs. deferred upsert in middleware.** Open. Synchronous is simpler and never drops a write, but puts a database write on the critical path of a read endpoint that may already be paying for a cold-start Overpass fetch (BEN-9). Deferred keeps the read path clean but adds a queue, a drop-on-shutdown failure mode, and coupling to the BEN-11 worker pool. Not blocking, but it should be settled before the middleware is written rather than after.

**Where the ops retention sub-issue lives.** Whether to create it under BEN-3 now or defer pending BEN-4, since the enforcement mechanism is entirely determined by the hosting platform choice — which is itself still open.
