# BEN-6 — Project Architecture Setup: Final Design

**Epic:** [BEN-6](https://benchfinder.youtrack.cloud/issue/BEN-6)
**Status:** Tool choices settled, CI wiring blocked on BEN-4

---

## 1. Purpose

Tooling and coding-standards setup for both the Go backend and the Flutter/Dart client — linting, formatting, testing conventions, CI. Deliberately kept distinct from the feature and architecture-design epics.

**Governing principle:** given the project's learning goal, favour stricter and more comprehensive tooling over minimal defaults. More rules firing surfaces more idioms of each language. A linter that never complains teaches nothing.

---

## 2. Go backend

### `golangci-lint` — primary static analysis

The meta-linter, aggregating the individual analysers behind one config and one invocation.

- Runs `staticcheck`, `govet`, `errcheck`, `gosimple`, `unused`, `revive`, `gosec` and others through a single aggregated config.
- **Start with a fairly permissive default config.** This is a learning project — the value is in seeing what gets flagged and understanding why, not in zero-tolerance enforcement that makes the tool something to fight.
- Add a `.golangci.yml` to the backend repo so the configuration is explicit and version-controlled rather than implicit in defaults.

### Individually notable analysers

| Tool | Why it earns a separate mention |
|---|---|
| `staticcheck` | Worth knowing standalone, not just as a golangci-lint component. Catches subtle correctness bugs and is a good introduction to what static analysis over a typed language can actually do. |
| `gosec` | Security-focused — SQL injection, unsafe usage patterns. Directly relevant since the API builds parameterised geo queries against user-supplied coordinates. |
| `goimports` | Formatting plus import grouping. Run as an editor save-hook. |

### Integration

- Wire into CI once a CI pipeline exists — depends on BEN-4.
- Optionally available as a pre-commit hook or a `make` target for local use.

---

## 3. Flutter / Dart client

### `very_good_analysis` — analyzer ruleset

A stricter drop-in replacement for the default `flutter_lints`, from Very Good Ventures.

Chosen over `flutter_lints` on the same principle as the Go side: more rules firing means more Dart idioms surfaced. The default ruleset is tuned to avoid annoying working teams, which is the opposite of what's useful when the point is to learn the language's conventions.

### `dart format`

Built into the Dart SDK, no configuration needed, no debate available. Run as a save-hook and in CI.

### Deferred

`custom_lint` + `riverpod_lint` — revisit if and when Riverpod is actually adopted for state management. Riverpod is the current leaning (with `AsyncNotifier` / `FutureProvider.family`), but the lint packages are only useful once the dependency exists.

---

## 4. Cross-cutting

A pre-commit hook or Makefile target running both the Go and Dart lint/format tools before commits, so the two toolchains are invoked through one entry point rather than remembered separately.

---

## 5. Relationship to other epics

- **BEN-4** — CI pipeline setup lives there; this epic's tools plug into it. This is the blocking dependency.
- **BEN-2 / BEN-3** — the `chi` router and the visitor-ID middleware are the first real consumers of the Go conventions established here.

---

## 6. Note on BEN-7

BEN-7 was created as a duplicate subtask of this epic. Its content was merged back into BEN-6 and the issue has since been deleted. Recorded here so the ID gap in the project is explicable.

---

## 7. Open items

1. **CI platform** — blocked on BEN-4. GitHub Actions is the obvious default given the repo is on GitHub and BEN-4 already contemplates Actions for scheduled ingestion.
2. **Testing conventions** — the epic scopes testing conventions as in-scope but doesn't yet specify any. Relevant existing decisions: BEN-2's repository interface exists specifically to enable service-layer tests without a database, so at minimum there's a stated intent to unit-test the service layer.
3. **Whether the ruleset tightens over time** — "start permissive" is decided, but there's no stated trigger for ratcheting up.
