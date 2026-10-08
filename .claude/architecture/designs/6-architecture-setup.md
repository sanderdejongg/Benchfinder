# Project Architecture Setup — Final Design

**Issue:** [BEN-6](https://benchfinder.youtrack.cloud/issue/BEN-6)  **Status:** settled, not implemented  **Updated:** 2026-10-08

---

## 1. Purpose

Tooling, coding standards, testing conventions, repository layout and CI for both the Symfony backend and the Flutter client. Kept separate from the feature designs, because these decisions are cross-cutting and long-lived.

**Governing principle:** given the learning goal, favour stricter and more comprehensive tooling over minimal defaults. More rules firing surface more idioms of each language. A linter that never complains teaches nothing.

---

## 2. Constraints

- **Strict by default.** Stricter tooling over minimal defaults, in both toolchains (D2, D17).
- **One entry point for both toolchains.** A single local command runs the backend and client lint and format tools, and CI runs the same commands (D8).

---

## 3. Repository layout

```
/CLAUDE.md                               project orientation
/.claude/architecture/designs/           current state, one file per topic
/.claude/architecture/decisions/         why, append-only
/.claude/skills/decision-log/SKILL.md    the skill that writes design pairs
/.claude/rules/                          project rules
/backend/                                Symfony (later phase)
/app/                                    Flutter (later phase)
```

Monorepo. Backend and client share one repository and one set of docs.

Doc naming: numeric topic slugs (`2-nearby-search`, children `1-1-…`). The tracker ID is never part of a filename. It appears only in each design's `**Issue:**` header line. Cross-references use slugs.

---

## 4. Backend (Symfony)

### Stack

Symfony on FrankenPHP, Doctrine with jsor/doctrine-postgis, Symfony Messenger on RabbitMQ, Symfony Scheduler, Symfony Console. Chosen as part of the stack for learning value, with the newer parts of Symfony as the explicit learning target (D13).

### Static analysis and formatting

- **PHPStan at maximum level from day one** (D17). Stricter than its defaults, so more idioms surface. Extensions and rule set are an open item.
- **PHP-CS-Fixer** for formatting.
- **Rector** for automated upgrades and idiom changes.

Starting permissive was considered and set aside (D3, superseded). The principle in D2 is that stricter tooling teaches more.

### Testing

- **PHPUnit**, the Symfony default (D15).
- **Integration tests against a real database.** A PostGIS container with h3-pg runs under docker compose locally, and as a GitHub Actions service container in CI (D16).
- **Per-test rollback** with dama/doctrine-test-bundle, so each test runs in a transaction that is rolled back (D16).
- **Messenger's in-memory transport in tests:** dispatched messages are captured and asserted without a broker. Handler tests invoke the handler directly, or run the worker once (D16).
- **Overpass fixtures**: golden JSON responses, served through Symfony's MockHttpClient (D16).
- **Unit tests** for the search service run against the in-memory repository fake (`2-nearby-search` D7). They need no database.
- **Freshness logic** (`1-1-h3-grid-freshness`) is tested against PostGIS with h3-pg, since h3-pg is in the database (`1-1-h3-grid-freshness` D5).

---

## 5. Client (Flutter / Dart)

### Analysis and formatting

- **very_good_analysis** as the analyzer ruleset, a stricter drop-in for `flutter_lints` (D5).
- **dart format**, with no configuration (D6).
- **riverpod_lint** with custom_lint, now in scope because Riverpod is adopted (`7-flutter-client` D7).

### Testing

- **flutter_test** for widget tests.
- **mocktail** for mocking client-side repository interfaces once they exist, without a code-generation step (D11).

---

## 6. CI

GitHub Actions runs on every pull request (D10). Jobs:

- Backend: static analysis (PHPStan, PHP-CS-Fixer, Rector check), unit tests, integration tests with the PostGIS service container.
- Client: very_good_analysis and dart format check, flutter_test.

A single local entry point runs both toolchains, and CI runs the same commands (D8).

---

## 7. Contracts & data model

No schema. Configuration files, one per toolchain, committed to the repository (D4).

---

## 8. Dependencies

- `4-deployment-shape`: GitHub Actions is the CI platform. Scheduled jobs run on Symfony Scheduler instead.
- `2-nearby-search`: the repository seam is what makes service-layer unit testing possible.
- `3-visitor-identity`, `1-1-h3-grid-freshness`: test approach for database-backed features.
- `7-flutter-client`: client test and lint tooling.

---

## 9. Explicitly deferred

- **Lint and static-analysis ratchets.** None. The analysers start at their strictest level (D17), so there is no later tightening step.

---

## 10. Superseded

- Permissive static-analysis start, with a later ratchet: replaced by maximum level from day one (D3, D12, D17).
- Lint packages for the client deferred until Riverpod is adopted: superseded, since Riverpod is adopted (D7).

---

## 11. Open items

1. **PHPStan extensions and level configuration.** Which extensions, and whether any baseline is needed at max level.
2. **PHP-CS-Fixer rule set.** Not chosen.
3. **Rector sets.** Not chosen.
4. **Pre-commit hook or Makefile target.** Which mechanism the single local entry point uses (D8).
