# Project Architecture Setup — Decision Log

Decisions leading to `../designs/6-architecture-setup.md`.

**Updated:** 2026-10-15

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

**Status:** Settled. Mechanism resolved by D23 (Makefile).

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

**Status:** Settled. PHPStan configuration in D19, PHP-CS-Fixer rules in D20, PHP_CodeSniffer added in D21, Rector sets in D22.

---

## D18 — PHP 8.5 and the latest stable Symfony 8.x

**Decision:** PHP 8.5. Symfony at the latest stable 8.x minor at scaffold time.

**Rationale:** The newer parts of Symfony are an explicit learning target (D13), so the newest stable release is the default. FrankenPHP supports PHP 8.5 (confirmed 2026-10-15). The PHP version also fixes the PHP-CS-Fixer migration set (D20) and the Rector PHP sets (D22).

**Status:** Settled.

---

## D19 — PHPStan: level max only, no extensions, no baseline

**Decision:** PHPStan at level `max` with no extensions (no phpstan-symfony, phpstan-doctrine, phpstan-phpunit, phpstan-strict-rules, phpstan-deprecation-rules) and no `bleedingEdge`. No baseline.

**Alternatives considered:**

| Option | Why rejected |
|---|---|
| Level max plus the Symfony, Doctrine, PHPUnit, strict-rules and deprecation-rules extensions, with `bleedingEdge` (recommended in discussion) | Not taken for now. The user chose level max alone ("for now"); no further reason given. |
| A baseline | Greenfield codebase: there is nothing to baseline, so a baseline would only hide new errors. |

**Rationale:** User decision to start with the bare analyser.

**Trade-off accepted:** without the Symfony and Doctrine extensions, PHPStan does not know framework return types. Calls such as container parameter lookups and query results resolve to `mixed`, which level max flags. These are handled with inline type assertions.

**Status:** Provisional ("for now"). Revisit point set by D27 (2026-10-15).

---

## D20 — PHP-CS-Fixer rule sets

**Decision:** `@Symfony`, `@Symfony:risky`, the PHP migration set matching PHP 8.5 (D18), and `declare_strict_types` on.

**Alternatives considered:**

| Option | Why rejected |
|---|---|
| Adding `@PER-CS` | `@Symfony` already covers it. |

**Rationale:** The Symfony sets match the framework's own conventions. The risky set and strict types apply D2.

**Status:** Settled.

---

## D21 — PHP_CodeSniffer as a separate tool alongside PHP-CS-Fixer; no ECS

**Decision:** PHP-CS-Fixer and PHP_CodeSniffer run as two standalone tools, each with its own config. ECS is not used.

**Alternatives considered:**

| Option | Why rejected |
|---|---|
| ECS only, running the PHP-CS-Fixer sets and CodeSniffer sniffs in one tool (recommended in discussion) | Not chosen. One tool hides each underlying tool's native config and CLI. |
| ECS for the PHP-CS-Fixer rules, standalone PHP_CodeSniffer for the sniffs | Reduces ECS to a wrapper around PHP-CS-Fixer, keeping its extra layer without its benefit (running both rule families together). |
| ECS plus standalone PHP-CS-Fixer and PHP_CodeSniffer | The same rules run twice, with a risk of conflicting fixes. |

**Rationale:** User decision. The case for this option made in discussion: each tool is learned on its own terms, with its own configuration and CLI, which fits the project's learning goal.

**Trade-off accepted:** two configs that can disagree on formatting. The PHP_CodeSniffer standard must not contradict PHP-CS-Fixer output, or the two tools fight over the same lines.

**Status:** Settled. The PHP_CodeSniffer standard is chosen in D25.

---

## D22 — Rector sets

**Decision:** `withPhpSets()`, `withComposerBased(symfony: true, doctrine: true, phpunit: true)`, and the prepared sets `deadCode`, `codeQuality`, `typeDeclarations` and `earlyReturn`. CI runs Rector with `--dry-run`.

**Rationale:** The PHP sets follow the pinned PHP version (D18). The Composer-based sets follow the installed framework versions, so upgrades surface new idioms automatically. The prepared sets apply D2. `--dry-run` in CI reports changes without making them.

**Status:** Settled.

---

## D23 — The single entry point is a Makefile; no pre-commit hook

**Resolves:** the mechanism left open in D8.

**Decision:** A root Makefile holds the targets for both toolchains. CI calls the same targets.

**Alternatives considered:**

| Option | Why rejected |
|---|---|
| Pre-commit hook | Nothing needs it yet. The project rules say not to build what is not needed now. |

**Rationale:** One file both a developer and CI invoke satisfies D8 directly.

**Status:** Settled. Target names are set by D26.

---

## D24 — CI database job added with the first integration test

**Decision:** The first CI workflow runs static analysis and unit tests only. The integration-test job, and the PostGIS + h3-pg image it needs, are added together with the first integration test. The image is built from a Dockerfile in the repository and published to GHCR. It pins Postgres 17 and the h3-pg version found on the managed instance (`4-deployment-shape` D17).

**Rationale:** No official PostGIS image ships h3-pg, and GitHub Actions service containers need a prebuilt image, so the test database costs a Dockerfile plus a publishing workflow. Building it before any test uses it is speculative. This sequences D16; it does not change it.

**Status:** Settled.

---

## D25 — PHP_CodeSniffer standard: curated Slevomat sniffs, no formatting

**Resolves:** the standard left open in D21.

**Decision:** Slevomat Coding Standard, with sniffs enabled one by one and no inherited base standard. The ruleset holds only non-formatting sniffs (types, complexity, class and function structure, unused code, exceptions, `==`). The list is in the design. Ownership: anything PHP-CS-Fixer can fix belongs to PHP-CS-Fixer. CI runs PHP_CodeSniffer report-only; `phpcbf` is a local Makefile target. The check target runs PHP-CS-Fixer first, then PHP_CodeSniffer, and must come out clean; a sniff that breaks this is not added.

**Alternatives considered:**

| Option | Why rejected |
|---|---|
| Doctrine Coding Standard | Contradicts `@Symfony` (D20) directly: Doctrine forbids Yoda comparisons that `@Symfony` requires, and writes `! $x` where `@Symfony` writes `!$x`. Adopting it means adopting Doctrine style, reversing D20. |
| PSR-12 or PER-CS | Already covered by `@Symfony`. Duplicate reports, nothing new. |
| Third-party Symfony PHPCS standards | Duplicate `@Symfony` formatting, for the same reason. |
| All Slevomat sniffs | Not possible: several are mutually exclusive (e.g. `RequireYodaComparison` and `DisallowYodaComparison`). |
| Switching to ECS (D21's option A) to avoid choosing | Reconsidered on 2026-10-15 and kept out. ECS changes where the sniffs are configured, not which are needed: Slevomat is not bundled, and contradictions still have to be resolved by hand. |

**Rationale:** PHP_CodeSniffer earns its place in D21 only by checking what PHP-CS-Fixer, PHPStan and Rector do not: native type coverage, cognitive complexity, class structure and logic smells. Keeping formatting out of the ruleset removes the conflict D21 accepted as a trade-off. Report-only in CI means the only fixer that can disagree with PHP-CS-Fixer never runs there.

**Trade-off accepted:** a curated list is maintained by hand. Each sniff is read before it is enabled, which fits the learning goal (D2).

**Assumption, falsifiable:** `Classes.RequireAbstractOrFinal` is compatible with Doctrine entities. Checked before the sniff covers `src/Entity`; if it is not, entities are excluded from that sniff.

**Status:** Settled. Ownership rule 2 is clarified by D26.

---

## D26 — Makefile targets: `check`, `fix`, `test`; backend only until `/app` exists

**Resolves:** the target names left open in D23. Clarifies D25 rule 2.

**Decision:** The root Makefile has three targets:

- `check` writes no files. It runs PHP-CS-Fixer in dry-run mode, then PHP_CodeSniffer, PHPStan, and Rector in dry-run mode. CI and local runs use it.
- `fix` writes files. It runs Rector, then PHP-CS-Fixer, then phpcbf. Local only.
- `test` runs the PHPUnit unit suite. The integration suite joins it when D24 adds it. CI and local runs use it.

No target exists for `/app` until `/app` is scaffolded. The tool list per target is in the design, section 7.

Clarification of D25: CI never writes files. "Only PHP-CS-Fixer fixes in CI" means PHP-CS-Fixer is the only fixer that may run, and it runs dry-run in CI.

**Alternatives considered:**

| Option | Why rejected |
|---|---|
| Per-tool targets as the public interface (`make phpstan`, `make php-cs-fixer`, …) | CI would need one step per tool, and a developer would need to know the tool list. Each tool is run with its own CLI when needed, which is what D21 intends. Not needed now, since CI needs two commands. |
| `format` in place of `fix` | The target also runs Rector and phpcbf, which change code beyond formatting. |
| `lint` in place of `check` | Does not say whether the target writes files. `check` does. |
| A `ci` target wrapping `check` and `test` | CI calls `check` and `test` directly. A wrapper would be a name with no caller. |
| `check` that writes files, or CI running `fix` | CI does not write files: Rector runs `--dry-run` (D22), and only the local `fix` target runs phpcbf (D25). |
| Client targets now (for example `analyze`) | `/app` does not exist, so there is nothing to run. Project rule: no speculative targets. |
| Separate `test-unit` and `test-integration` targets now | Only the unit suite exists. Whether integration gets its own target is part of design Open item 1. |

**Rationale:** The `check` and `fix` split follows whether a target writes files, so CI can call the one that cannot. The names are the verbs a developer types. Recommended in discussion: the three-target set and the `check` contents. Tool order in `check` follows D25: PHP-CS-Fixer runs before PHP_CodeSniffer. The `fix` order (Rector, PHP-CS-Fixer, phpcbf) keeps the same PHP-CS-Fixer-before-PHP_CodeSniffer order, and lets PHP-CS-Fixer format the code Rector changes.

**Trade-off accepted:**

- Make stops at the first failing tool. A PHP-CS-Fixer finding hides PHP_CodeSniffer, PHPStan and Rector output until it is fixed. The cost is a second `check` run.
- `fix` does not run `check`. If phpcbf or Rector output fails `check`, the `fix` order is revisited.
- Until `/app` exists, the Makefile covers the backend only. D8's two-toolchain intent has no client coverage before then.

**Assumption, falsifiable:** each tool in `check` exits non-zero on findings, so a finding fails the target. Checked when the tools are first installed. A tool that exits zero on findings needs an explicit check in the target.

**Status:** Settled for target names and contents. Where the targets run is design Open item 1.

---

## D27 — Revisit point for D19: the first Doctrine entity or repository

D19's decision is unchanged and remains Provisional. This decision adds only its revisit point.

**Decision:** D19 is revisited no later than the first merged change that adds a Doctrine entity or a repository class under `backend/src/`. That change is the review point. The owner records the outcome in this log, either as a new decision or as a status note on D19. At the review, the owner decides whether to add phpstan-doctrine and phpstan-symfony, and whether to add or keep out the strictness extensions. This is a latest point, not an earliest one: a framework `mixed` need that appears earlier may reopen D19 earlier.

**Alternatives considered:**

| Option | Why rejected |
|---|---|
| A count of inline type assertions or `@var` overrides for framework types | Observable, but no number has been derived from anything, so the threshold would be invented. It also fires only after the cost has been paid in code. |
| A date | The cost depends on code, not on the calendar. A date can pass with no Doctrine code written, or land in the middle of the nearby-search build. |
| The milestone "nearby search shipped" | In practice the same event, since `2-nearby-search` introduces the first entity. "Shipped" is judged after the merge, so the merge itself is the more precise observable. |
| No trigger; revisit ad hoc | This is the state D19 is in now. It has no end point, which is the gap this decision closes. |

**Rationale:** D19's stated cost is framework types. Doctrine repositories and query results are the types phpstan-doctrine covers, and there is nothing for it to type before the first entity or repository exists. That event is visible in git history. Symfony container typing (phpstan-symfony) can matter earlier, which is why the trigger is a latest point. The strictness extensions (strict-rules, deprecation-rules, bleedingEdge) are not about framework code. They are placed at the same review rather than given a trigger of their own, because no separate trigger has been argued for them.

**Trade-off accepted:** the review can come after inline assertions already exist, and those assertions are its input. The trigger also fires for a repository that uses only native SQL through DBAL. The KNN repository (`2-nearby-search` D7, D16) runs native SQL through DBAL, alongside ORM-mapped entities used for schema diffing (D16). Whether phpstan-doctrine types raw DBAL result rows is not checked in this change. The review must check it before it adds the extension.

**Status:** Settled.

---

## Unresolved at time of writing

- Where the Makefile targets run, and how integration tests reach their database (design Open item 1; D26).
- h3-pg version for the test image (design Open item 2; D24).
- Trigger for phpstan-phpunit, and whether the strictness extensions need a trigger of their own (design Open item 3; D27).
