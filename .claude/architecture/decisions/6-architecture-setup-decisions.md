# BEN-6 — Project Architecture Setup: Decision Log

Decisions leading to the design in `BEN-6-architecture-setup-design.md`.

---

## D1 — Tooling gets its own epic, separate from feature work

**Decision:** Keep linting, formatting, testing conventions, and CI in a dedicated epic rather than folding them into whichever feature epic happens to need them first.

**Rationale:** These decisions are cross-cutting and long-lived. Burying "we use golangci-lint" inside the nearby-search epic makes it undiscoverable later and implies it's scoped to that feature. A separate epic also gives tooling additions a natural home as they come up, rather than forcing each one into an awkward parent.

---

## D2 — Prefer strict tooling over minimal defaults

**Decision:** Across both languages, choose the stricter ruleset. Stated as the governing principle for this epic.

**Rationale:** Default lint configurations are tuned to avoid annoying working teams shipping production software — they're deliberately quiet. That's precisely wrong for a project whose stated purpose is learning two unfamiliar languages. Every rule that fires is a language idiom being pointed out, with a rationale attached, at the moment it's relevant.

This is the same "over-engineering encouraged within reason" principle that shows up elsewhere in the project, applied to tooling.

---

## D3 — `golangci-lint` as the primary Go analyser

**Decision:** Adopt `golangci-lint` as the aggregating meta-linter.

**Alternatives considered:** running `go vet` alone; running `staticcheck` alone; assembling individual analysers by hand.

**Rationale:** `golangci-lint` runs staticcheck, govet, errcheck, gosimple, unused, revive, gosec and others behind one config and one command. Assembling them individually means maintaining parallel invocations and configs for no benefit. It's also the de facto community standard, so the configuration knowledge transfers to any other Go codebase.

---

## D4 — Start permissive, not zero-tolerance

**Decision:** Begin with a fairly permissive `golangci-lint` config.

**Rationale:** This sits in slight tension with D2 and the resolution is worth recording. Enabling every available linter at maximum strictness on a brand-new codebase produces hundreds of findings, most of them noise, and the predictable outcome is that the tool becomes something to suppress rather than read. Starting permissive keeps the signal high enough that each finding is worth actually investigating — which is where the learning is.

The strictness principle in D2 is about *choosing capable tools*; this decision is about *not drowning in their output on day one*.

---

## D5 — Explicit `.golangci.yml` in the repo

**Decision:** Commit a config file rather than relying on defaults.

**Rationale:** Makes the tool's behaviour explicit, version-controlled, and reviewable. It also means the config is the same locally and in CI, which avoids the failure mode where CI flags things the developer's machine never did.

---

## D6 — `staticcheck` called out standalone despite golangci-lint including it

**Decision:** Learn and run `staticcheck` on its own as well.

**Rationale:** Deliberately redundant. `staticcheck` on its own is the clearest single demonstration of what static analysis over a statically typed language can catch — it finds real correctness bugs, not just style deviations. Running it directly, and reading its output unfiltered, is a better introduction to the concept than seeing its findings anonymised inside an aggregated report. Learning motivation, recorded as such.

---

## D7 — `gosec` specifically justified by the domain

**Decision:** Include `gosec`, with the reasoning tied to what this API actually does.

**Rationale:** The API takes user-supplied coordinates and builds spatial SQL from them. That's the exact shape of code where injection mistakes happen. `gosec` flags string-concatenated queries and unsafe usage patterns — directly relevant here rather than generically good practice.

---

## D8 — `very_good_analysis` over `flutter_lints`

**Decision:** Use Very Good Ventures' `very_good_analysis` as the Dart analyzer ruleset.

**Alternatives considered:** the default `flutter_lints`; `lints` (the base Dart set); a hand-rolled `analysis_options.yaml`.

**Rationale:** `flutter_lints` is deliberately conservative — it's the set chosen not to irritate the median Flutter project. `very_good_analysis` is a stricter drop-in with substantially more rules enabled. Given Dart is a language being learned from scratch here, the stricter set surfaces more idioms, more often, with the rule documentation one click away. Same reasoning as D2, applied to the client.

Hand-rolling a ruleset was rejected as requiring exactly the Dart knowledge that doesn't exist yet — the point is to be *told* the conventions, not to have to already know them.

---

## D9 — `dart format` as-is

**Decision:** Use `dart format` with no configuration.

**Rationale:** It's built into the SDK, it has essentially no knobs, and Dart's community norm is universal adoption. There's nothing to decide, which is the feature.

---

## D10 — `riverpod_lint` deferred until Riverpod is adopted

**Decision:** Note `custom_lint` + `riverpod_lint` as a future addition, contingent on actually adopting Riverpod.

**Rationale:** Riverpod is the current leaning for state management (`AsyncNotifier`, `FutureProvider.family`), but it isn't a committed dependency yet. Adding its lint package before the library exists in the project would be tooling for code that doesn't exist. Recorded as a named deferral rather than dropped, consistent with the project's pattern.

---

## D11 — Single entry point for both toolchains

**Decision:** A pre-commit hook or Makefile target that runs the Go and Dart lint/format tools together.

**Rationale:** A two-language repo has two toolchains with entirely different invocations. One command that runs both means neither gets forgotten, and it's the same command CI will run — so local and CI behaviour can't silently diverge.

---

## D12 — CI wiring deferred to BEN-4

**Decision:** Establish the tools now; wire them into CI once a pipeline exists under BEN-4.

**Rationale:** The tools are useful locally from day one and don't depend on CI to provide value. CI configuration, by contrast, depends on the hosting-platform decisions that are still open in BEN-4. Sequencing them this way means no rework when that lands. GitHub Actions is the likely target, since BEN-4 already contemplates scheduled Actions workflows for ingestion.

---

## D13 — BEN-7 consolidated into BEN-6 and deleted

**Decision:** BEN-7 was created as a duplicate subtask covering the same tooling ground. Its content was merged into BEN-6 and the issue deleted.

**Rationale:** Two issues describing the same tooling setup would guarantee they drift apart. Worth recording because it explains the ID gap in the project's issue sequence.

**Process note:** the YouTrack MCP integration exposes no delete operation, so the workaround was to consolidate content into the surviving issue and delete manually through the web UI. Confirmed done — BEN-7 no longer appears in the project.

---

## Unresolved at time of writing

- **CI platform** — blocked on BEN-4, though GitHub Actions is the obvious default.
- **Testing conventions** — in scope for this epic but not yet specified. The only concrete signal so far is BEN-2's interface-based repository boundary, which exists to make the service layer unit-testable without a database.
- **Ratchet trigger** — "start permissive" is decided, but nothing states when or whether the ruleset tightens.
