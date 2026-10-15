# Project Architecture Setup — Final Design

**Issue:** [BEN-6](https://benchfinder.youtrack.cloud/issue/BEN-6)  **Status:** settled, not implemented  **Updated:** 2026-10-15

---

## 1. Purpose

Tooling, coding standards, testing conventions, repository layout and CI for both the Symfony backend and the Flutter client. Kept separate from the feature designs, because these decisions are cross-cutting and long-lived.

**Governing principle:** given the learning goal, favour stricter and more comprehensive tooling over minimal defaults. More rules firing surface more idioms of each language. A linter that never complains teaches nothing.

---

## 2. Constraints

- **Strict by default.** Stricter tooling over minimal defaults, in both toolchains (D2, D17).
- **One entry point for both toolchains.** A single local command runs the backend and client lint and format tools, and CI runs the same commands (D8, D23).
- **Test database matches production.** The CI and local PostGIS image runs the same Postgres major (17) and h3-pg version as the managed instance (D24, `4-deployment-shape` D17).

---

## 3. Repository layout

```
/CLAUDE.md                               project orientation
/Makefile                                single entry point for both toolchains (D23)
/.claude/architecture/designs/           current state, one file per topic
/.claude/architecture/decisions/         why, append-only
/.claude/skills/decision-log/SKILL.md    the skill that writes design pairs
/.claude/rules/                          project rules
/backend/                                Symfony
/app/                                    Flutter (later phase)
```

Monorepo. Backend and client share one repository and one set of docs.

Doc naming: numeric topic slugs (`2-nearby-search`, children `1-1-…`). The tracker ID is never part of a filename. It appears only in each design's `**Issue:**` header line. Cross-references use slugs.

---

## 4. Backend (Symfony)

### Stack

PHP 8.5 and the latest stable Symfony 8.x minor at scaffold time (D18). Symfony on FrankenPHP, Doctrine with jsor/doctrine-postgis, Symfony Messenger on RabbitMQ, Symfony Scheduler, Symfony Console. Chosen as part of the stack for learning value, with the newer parts of Symfony as the explicit learning target (D13).

### Static analysis and formatting

Four standalone tools, each with its own committed config (D4). No ECS wrapper (D21).

| Tool | Role | Configuration |
|---|---|---|
| PHPStan | Static analysis | Level `max`. No extensions, no `bleedingEdge`, no baseline (D19, provisional; revisit point D27). |
| PHP-CS-Fixer | Formatting (fixes) | `@Symfony`, `@Symfony:risky`, the PHP migration set matching PHP 8.5, `declare_strict_types` on (D20). |
| PHP_CodeSniffer | Non-formatting sniffs: types, complexity, structure | Slevomat Coding Standard, curated list below, no formatting sniffs. Report-only in CI; `phpcbf` runs only from a local Makefile target (D25). |
| Rector | Automated upgrades and idiom changes | `withPhpSets()`, `withComposerBased(symfony, doctrine, phpunit)`, prepared sets `deadCode`, `codeQuality`, `typeDeclarations`, `earlyReturn`. CI runs `--dry-run` (D22). |

### PHP_CodeSniffer sniffs (D25)

Slevomat Coding Standard (`slevomat/coding-standard`), sniffs enabled one by one. No base standard is inherited.

| Area | Sniffs (`SlevomatCodingStandard.*`) |
|---|---|
| Types | `TypeHints.ParameterTypeHint`, `TypeHints.PropertyTypeHint`, `TypeHints.ReturnTypeHint`, `TypeHints.DisallowMixedTypeHint` |
| Complexity | `Complexity.Cognitive` |
| Classes | `Classes.RequireConstructorPropertyPromotion`, `Classes.ForbiddenPublicProperty`, `Classes.RequireAbstractOrFinal` (Doctrine entity compatibility checked before it covers `src/Entity`) |
| Functions | `Functions.StaticClosure`, `Functions.UnusedParameter`, `Functions.DisallowEmptyFunction` |
| Variables | `Variables.UnusedVariable`, `Variables.UselessVariable` |
| Exceptions | `Exceptions.DeadCatch`, `Exceptions.ReferenceThrowableOnly`, `Exceptions.RequireNonCapturingCatch` |
| Operators | `Operators.DisallowEqualOperators` |

Ownership rules between PHP-CS-Fixer and PHP_CodeSniffer:

1. Anything PHP-CS-Fixer can fix is owned by PHP-CS-Fixer. No whitespace, brace, import-ordering or Yoda sniffs in the PHP_CodeSniffer ruleset.
2. Only PHP-CS-Fixer fixes in CI. PHP_CodeSniffer reports; `phpcbf` is a local target only.
3. Conflict test: the Makefile check target runs PHP-CS-Fixer, then PHP_CodeSniffer, over the whole codebase, and the result must be clean. A sniff that breaks this is not added.

`DisallowMixedTypeHint` complements D19: framework `mixed` is handled with inline assertions, and project code may not declare `mixed` itself.

Starting permissive was considered and set aside (D3, superseded). The principle in D2 is that stricter tooling teaches more.

Known cost of D19: without the Symfony and Doctrine PHPStan extensions, framework APIs that return `mixed` (container parameters, query results) are flagged at max level and need inline type assertions.

### Testing

- **PHPUnit**, the Symfony default (D15).
- **Integration tests against a real database.** A PostGIS container with h3-pg runs under docker compose locally, and as a GitHub Actions service container in CI (D16). The image is built from a Dockerfile in the repository and published to GHCR, since no official PostGIS image ships h3-pg and service containers need a prebuilt image (D24).
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

GitHub Actions runs on every pull request (D10). Each job calls a Makefile target, so CI and local runs execute the same commands (D8, D23).

- **Backend, from the first commit:** PHPStan, PHP-CS-Fixer check, PHP_CodeSniffer (report only), Rector `--dry-run`, unit tests.
- **Backend, added with the first integration test:** integration tests against the PostGIS + h3-pg service container (D24).
- **Client, added when `/app` is scaffolded:** very_good_analysis, dart format check, flutter_test.

---

## 7. Contracts & data model

No schema. Configuration files, one per tool, committed to the repository (D4).

Makefile targets are the contract between local runs and CI (D23). The check target runs PHP-CS-Fixer before PHP_CodeSniffer (D25). Target names are not fixed yet (open item 1).

---

## 8. Dependencies

- `4-deployment-shape`: GitHub Actions is the CI platform. Scheduled jobs run on Symfony Scheduler instead. Postgres 17 Standard Edition and its h3-pg version fix the test image (D17 there).
- `2-nearby-search`: the repository seam is what makes service-layer unit testing possible.
- `3-visitor-identity`, `1-1-h3-grid-freshness`: test approach for database-backed features.
- `7-flutter-client`: client test and lint tooling.

---

## 9. Explicitly deferred

- **Lint and static-analysis ratchets.** None. The analysers start at their strictest level (D17), so there is no later tightening step.
- **PHPStan extensions and `bleedingEdge`.** Left out for now (D19). Not a ratchet: level stays at max; this is about framework type knowledge. Revisit point: the first merged Doctrine entity or repository class under `backend/src/`, proposed in D27. That merge is a latest point for the review, not an earliest one.
- **Pre-commit hook.** Not built. The Makefile is the entry point (D23); nothing yet needs a hook on top.
- **CI database job and the PostGIS + h3-pg image.** Added with the first integration test, not before (D24).

---

## 10. Superseded

- Permissive static-analysis start, with a later ratchet: replaced by maximum level from day one (D3, D12, D17).
- Lint packages for the client deferred until Riverpod is adopted: superseded, since Riverpod is adopted (D7).

---

## 11. Open items

1. **Makefile target names.** E.g. `check`, `fix`, `test`. Not fixed.
2. **h3-pg version for the test image.** Read from the provisioned managed instance (`pg_available_extensions`) and pinned in the Dockerfile (`4-deployment-shape` open item 1).
3. **Trigger for phpstan-phpunit and the strictness extensions.** D27 sets the review point for phpstan-doctrine and phpstan-symfony. phpstan-phpunit, phpstan-strict-rules, phpstan-deprecation-rules and `bleedingEdge` are not given a trigger of their own. Whether they need one is open.
