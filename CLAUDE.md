# CLAUDE.md — Benchfinder

Benchfinder is a no-login mobile app that answers "where can I sit?": it shows public benches near the user, from OpenStreetMap `amenity=bench` data held in our own PostGIS database. It is a solo learning project. Tech choices are weighed for learning value, and over-engineering is encouraged within reason. Focus on planning: propose designs and options, don't write code unless asked.

## Stack
- Backend: Symfony on FrankenPHP, Mercure (embedded hub) for push, Doctrine with jsor/doctrine-postgis, PostgreSQL + PostGIS + h3-pg, Symfony Messenger on RabbitMQ, Symfony Scheduler, Symfony Console.
- Client: Flutter (iOS + Android), Riverpod, flutter_map, geolocator, flutter_secure_storage.
- Data: OSM via Overpass, ingested into our own PostGIS. Overpass is never on the read path.
- Hosting: one DigitalOcean Droplet running docker compose (FrankenPHP, messenger worker, RabbitMQ), plus DigitalOcean Managed Postgres in the same region and VPC.

## Monorepo
- `/backend` Symfony (later phase)
- `/app` Flutter (later phase)
- `/.claude/architecture/designs/` current state of each topic
- `/.claude/architecture/decisions/` why, and what was rejected (append-only)
- `/.claude/skills/decision-log/` the skill that writes these pairs
- `/.claude/rules/` project rules (not architecture)

## Hard constraints
- Visitor ID and precise location never appear in the same log line. At MVP the visitor ID is never logged (requires the access-log header filter — see `3-visitor-identity`).
- No results is `200` with `"benches": []`, never `404`.
- OSM/ODbL attribution is shown in-app (settings/about).
- No user accounts at MVP. The anonymous visitor UUID is a device fingerprint: plumbing only, purpose-limited under GDPR, never repurposed for location-data monetisation.

## Where design truth lives
- `.claude/architecture/designs/<slug>.md`: what is true now. Replaced on change.
- `.claude/architecture/decisions/<slug>.md`: why. Superseded entries stay, marked `⚠ Superseded by D<n>`.
- Check these before assuming something is undecided. Undecided items are listed under each design's Open items.
- Slugs are numeric: `2-nearby-search`, children `1-1-h3-grid-freshness`. Tracker IDs appear only in a design's `**Issue:**` line.

## Build order
Nearby search (`2-nearby-search`) comes before the ingestion pipeline (`1-ingestion-pipeline`). Within ingestion, `1-1-h3-grid-freshness` is built first.

## Key decisions (details in the decision logs)
- Always answer from PostGIS. Cold or stale cells are ingested asynchronously and announced by Mercure push; the client re-calls (`2-nearby-search` D14, `1-2-cell-ingestion` D7).
- KNN with a hard 5000 m cutoff, not a radius ladder (`2-nearby-search` D1–D2).
- H3 res 8 via h3-pg for freshness (`1-1-h3-grid-freshness` D1, D5).
- One Overpass call per cold or stale cell, covering the k=2 disk of about 2 km (`1-2-cell-ingestion` D8–D9).
- Symfony Messenger on RabbitMQ for ingestion jobs (`1-4-dispatch-mechanism` D2).
- Mercure (SSE) for push: one event per job, invalidation signal only (`5-realtime-push` D5, D12, D14).
- Visitor ID is not logged (access-log filter required, see `3-visitor-identity`). The 12-month purge runs on Symfony Scheduler (`3-visitor-identity` D13–D14).
- flutter_map over google_maps_flutter (`7-flutter-client` D1).

## Explicitly deferred (don't re-litigate)
- Deterministic tiebreak in nearby results (`2-nearby-search` D5)
- H3 res 8 to res 9 graduation (`1-1-h3-grid-freshness` D2)
- Persistent viewport-tracking push connections
- Flutter web target (CORS; browser storage differs from keychain)
- Scheduled ingestion, until its named trigger (`4-deployment-shape` D12, D16)
- Standalone or HA Mercure hub; trigger: a second app instance (`5-realtime-push` D13)
- WebSocket vs Mercure comparison
- OpenBenches enrichment
- `GET /benches/{id}`
- Pagination and `count`
