# CLAUDE.md — Benchfinder

Benchfinder is a no-login mobile app for finding nearby public benches, built with a Go backend and a Flutter app. Focus on planning: propose designs and options, don't write code unless asked.

## Key Decisions & Why
- **KNN + hard cutoff (5000m) over adaptive radius ladder** — bench density varies ~20x across area types (dense centers vs. rural), and this was simpler while still handling that range.
- **H3 grid (not square grid)** — square grids distort at latitude and have uneven adjacency; H3 res 8 is the primary freshness-tracking grain, with res 9 derivable later.
- **WebSocket over SSE for real-time push** — chosen for learning value, not because SSE was insufficient.
- **Anonymous visitor ID up front** — not user-facing at MVP, but foundational plumbing for future favorites/history features.
- **`flutter_map` over `google_maps_flutter`** — avoids billing/API key friction; reversible, confined to the presentation layer.

## Explicitly Deferred (don't re-litigate)
- Smart H3 resolution graduation (res 8 → res 9)
- Deterministic tiebreak in nearby-search results
- Persistent viewport-tracking for real-time push
- Flutter web support (architecturally possible, deliberately postponed)

## Hard Constraints
- Visitor ID and precise location must never appear in the same log line.
- No-results is a `200` with an empty array, not a `404`.
- OSM/ODbL attribution required in-app.

## Source of Truth
Design decisions and open questions live in the repo as markdown, split into `designs/` and `decisions/` — check there before assuming something is unresolved or undecided.