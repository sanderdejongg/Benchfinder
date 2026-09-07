---
name: decision-log
description: Capture a finalized design as two markdown files — a design doc in .claude/architecture/designs/ and a decision log in .claude/architecture/decisions/. Use whenever a plan or design reaches a settled state, whenever an issue tracker epic or design ticket is written or updated, and whenever an existing design is revised or superseded — even if the user doesn't say "document this".
---

# Decision Log

Two files per topic, sharing a stem:

```
.claude/architecture/designs/<slug>.md      what it is now
.claude/architecture/decisions/<slug>.md    why, and what it isn't
```

Slug: lead with the issue ID if the project has a tracker (`BEN-2-nearby-search`), otherwise a kebab-case topic. Match whatever is already in the directories. Create the directories under the project root if missing.

Skip both files for work with no design content — if no alternatives were rejected, there's no decision log to write.

## On rewrite

Read the existing pair first, then:

- **Design doc: replace.** Current state only. Old approaches appear solely in "Superseded".
- **Decision log: append.** Never delete a superseded entry — mark it `⚠ Superseded by D<n>` and leave it intact. A reversed decision is the record that an option was tried and failed; drop it and someone proposes it again next quarter. Next number in sequence, never reused.

## Design doc

```markdown
# <Topic> — Final Design
**Issue:** <link>  **Status:** <settled/partial/implemented>  **Updated:** <date>

## Purpose
## Constraints — what makes this non-trivial; include hard numbers
## Design — current shape, flows, responsibilities
## Contracts & data model — schemas, constants and their values
## Dependencies — other designs, by slug
## Explicitly deferred — named, with why (separates "chose not to" from "forgot")
## Superseded — one line each, pointing at the replacing decision
## Open items — numbered, undecided
```

## Decision log

```markdown
# <Topic> — Decision Log
Decisions leading to `../designs/<slug>.md`.

## D1 — <the decision, stated flatly>
**Alternatives considered:** table of option → why rejected
**Rationale:** the reasoning, so a reader can tell if it still holds
**Trade-off accepted:**
**Status:** Settled | ⚠ Superseded by D<n> (<date>) | Provisional

## Unresolved at time of writing
```

Drop fields that don't apply rather than padding them.

Record the reason that actually drove the choice, including "learning value" or "cheaper to change later" — a reader can evaluate honest soft reasoning and can't evaluate invented technical justification. State assumptions as falsifiable and name revisit triggers where they exist. Never invent deliberation that didn't happen; undecided goes in Open items.

## Before finishing

Skim the other docs in `.claude/architecture/` for contradictions, sideways constraints, stale cross-references, and unbounded recursion. Report what you find — don't silently fix it.

No code. Schemas, contracts and constants yes; implementations no.