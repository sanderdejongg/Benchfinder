# Project Architecture Setup — Decision Log

Decisions leading to `../designs/6-architecture-setup.md`.

**Updated:** 2026-10-08

---

## D1 — Tooling gets its own design, separate from feature work

**Decision:** Keep linting, formatting, testing conventions, repository layout and CI in a dedicated design, not folded into whichever feature design needs them first.

**Rationale:** These decisions are cross-cutting and long-lived. Burying tool choices inside a feature design makes them undiscoverable and implies they are scoped to that feature. A separate design gives later tooling a natural home.

**Status:** Settled.

---

## D2 — Prefer strict tooling over minimal defaults

**Decision:** Across both toolchains, choose the stricter ruleset. This is the governing principle for this design.

**Rationale:** Default configurations are tuned to avoid annoying working teams shipping production software, so they are deliberately quiet. That is the wrong setting for a project whose purpose is learning Dart and the newer Symfony components. Each rule that fires is a language idiom pointed out at the moment it matters, with its rationale attached.

This is the same "over-engineering encouraged within reason" principle that applies elsewhere in the project, applied to tooling.

**Status:** Settled.

---

## D3 — Start permissive, not zero-tolerance

**Decision (original):** Begin with a fairly permissive static-analysis configuration on the new codebase, and tighten it later.

**Rationale (original):** Maximum strictness on a brand-new codebase produces many findings, most of them noise. The tool then becomes something to suppress rather than read. The strictness principle (D2) is about choosing capable tools. This decision was about not drowning in their output on day one.

**Status:** ⚠ Superseded by D17 (2026-10-08). The analysers start at maximum level from day one.

---

## D4 — Explicit configuration file committed to the repository

**Decision:** Each toolchain's configuration is a committed file, not an implicit default.

**Rationale:** The configuration is explicit, version-controlled and reviewable. It is the same locally and in CI, so CI never flags something the developer's machine did not.

**Status:** Settled.

---

## D5 — `very_good_analysis` over `flutter_lints`

**Decision:** Use Very Good Ventures' `very_good_analysis` as the Dart analyzer ruleset.

**Alternatives considered:** the default `flutter_lints`; the base `lints` set; a hand-rolled `analysis_options.yaml`.

**Rationale:** `flutter_lints` is deliberately conservative. `very_good_analysis` is a stricter drop-in with substantially more rules enabled. Dart is being learned from scratch here, so the stricter set surfaces more idioms, with the rule documentation one step away. A hand-rolled ruleset would require the Dart knowledge the project is trying to acquire.

**Status:** Settled.

---

## D6 — `dart format` as-is

**Decision:** `dart format` with no configuration.

**Rationale:** It ships with the SDK, has essentially no knobs, and the Dart community norm is universal adoption. There is nothing to decide.

**Status:** Settled.

---

## D7 — `riverpod_lint` deferred until Riverpod is adopted

**Decision (original):** Note `custom_lint` and `riverpod_lint` as a future addition, contingent on adopting Riverpod.

**Rationale (original):** Riverpod was the current leaning, but was not yet a committed dependency. Lint tooling for code that does not exist yet would be wasted effort.

**Status:** ⚠ Superseded by `7-flutter-client` D7 (2026-10-08). Riverpod is adopted, so the lint packages are in scope.

---

## D8 — Single entry point for both toolchains

**Decision:** A pre-commit hook or Makefile target runs the backend and client lint and format tools together.

**Rationale:** A two-toolchain repository has two sets of invocations. One command means neither is forgotten, and it is the command CI runs, so local and CI behaviour cannot silently diverge.

**Status:** Settled. Mechanism (hook or Makefile target) is an open item.

---

## D9 — CI wiring deferred to the deployment design

**Decision (original):** Establish the tools first. Wire them into CI once a pipeline exists.

**Rationale (original):** The tools are useful locally from day one. CI configuration depended on hosting decisions still open at the time.

**Status:** Resolved by D10 (2026-09-08).

---

## D10 — CI platform: GitHub Actions

**Decision:** GitHub Actions runs lint, static analysis and tests on every pull request.

**Rationale:** The repository is on GitHub, and GitHub Actions is the CI platform this design uses. The original rationale also cited scheduled jobs on the same platform. Those jobs have moved to Symfony Scheduler (`4-deployment-shape` D16), so CI is now the platform's only use here.

**Status:** ⚠ Partially superseded by `4-deployment-shape` D16 (2026-10-08). The scheduled-jobs part of the rationale is replaced; rest Settled.

---

## D11 — Dart testing: `flutter_test` and `mocktail`

**Decision:** `flutter_test` for widget tests. `mocktail`, not `mockito`, for mocking client-side repository interfaces once they exist.

**Alternatives considered:** `mockito`.

**Rationale:** `mockito` needs a code-generation build step (`build_runner`). `mocktail` gives the same capability without one. A project with no other reason to adopt code generation should not adopt it for mocks.

**Status:** Settled.

---

## D12 — Lint ratchet triggered by backend MVP completion

**Decision (original):** Start with a permissive configuration. Ratchet to a stricter one once designs 1, 2 and 3 are implemented and passing under the current configuration.

**Rationale (original):** "Start permissive" needed a checkable trigger for when it ends. Backend MVP completion is a real milestone.

**Status:** ⚠ Superseded by D17 (2026-10-08). There is no permissive start, so there is no ratchet.

---

## D13 — Backend in Symfony

**Decision:** The backend is Symfony on FrankenPHP, using Doctrine, Messenger, Scheduler and Console. The newer parts of Symfony (Messenger, Scheduler) are an explicit learning target.

**Alternatives considered:** none evaluated. Chosen as part of the stack.

**Rationale:** Tech choices in this project are weighed for learning value. Symfony's newer components are where the learning is.

**Status:** Settled.

---

## D14 — Monorepo layout and doc naming

**Decision:** One repository. `/backend` holds the Symfony project. `/app` holds the Flutter project. Architecture docs live in `.claude/architecture/designs/` and `.claude/architecture/decisions/`. `CLAUDE.md` sits at the root. Doc filenames are numeric topic slugs. The tracker ID appears only in each design's `**Issue:**` header line.

**Rationale:** The 2026-10-08 design discussion fixes the layout. The tracker ID stays in the header line, so filenames do not depend on the tracker.

**Status:** Settled.

---

## D15 — PHPUnit as the backend test framework

**Decision:** PHPUnit, the Symfony default.

**Alternatives considered:** none evaluated. Chosen as part of the stack.

**Status:** Settled.

---

## D16 — Test database and fixtures

**Decision:** Integration tests run against a PostGIS container with h3-pg, started by docker compose locally and by a GitHub Actions service container in CI. dama/doctrine-test-bundle wraps each test in a transaction that is rolled back. Messenger's in-memory transport is used in tests. Overpass responses are golden JSON fixtures, served through MockHttpClient.

**Alternatives considered:**

| Option | Why rejected |
|---|---|
| A testcontainers library for PHP | Slower, and less established than the compose-plus-service-container approach. |
| Compose without transaction rollback | Slower: each test needs a fresh database state. |

**Rationale:** The database tests need PostGIS and h3-pg, which a container provides. Rollback keeps the suite fast without sharing state between tests. The in-memory transport keeps the tests off the real broker. Golden fixtures avoid the network.

**Trade-off accepted:** the freshness and KNN tests need the database container. They cannot run as pure unit tests. This follows from `1-1-h3-grid-freshness` D5.

**Status:** Settled.

---

## D17 — Static analysis at maximum from day one

**Supersedes:** D3, D12.

**Decision:** PHPStan at its maximum level from the first commit. PHP-CS-Fixer for formatting. Rector for automated upgrades and idiom changes. Each is stricter than its defaults.

**Rationale:** Applies D2. More rules firing surface more idioms. This supersedes D3 (start permissive) and D12 (ratchet trigger), because there is no permissive start and nothing to ratchet from.

**Trade-off accepted:** day-one findings will be noisier than a permissive start would produce. Those findings are accepted, to be resolved as they appear rather than suppressed in bulk.

**Status:** Settled. Rulesets and extensions are open items in the design.

---

## Unresolved at time of writing

- PHPStan extensions and baseline policy at maximum level (design Open item 1).
- PHP-CS-Fixer and Rector rule sets (design Open items 2 and 3).
- Pre-commit hook or Makefile target (design Open item 4).
