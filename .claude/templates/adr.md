# Architecture Decision Record Template

```markdown
# ADR-<number>: <short title>

**Status:** Proposed | Accepted | Superseded by ADR-<n> | Deprecated
**Date:** <YYYY-MM-DD>
**Deciders:** <who>

## Context
<What problem/situation forced this decision? What constraints applied (scale, team size, cost, timeline)?>

## Decision
<What was decided, stated plainly and unambiguously>

## Alternatives Considered
1. **<Option A>** — pros / cons
2. **<Option B>** — pros / cons
(Include the option NOT chosen and why, not just the winner.)

## Consequences
- **Positive:**
- **Negative / trade-offs accepted:**
- **Follow-up work required:**

## Related
- `memory/project-decisions.md` entry:
- Affected files/packages:
```

## Usage Notes
- Write an ADR for decisions that are expensive to reverse or affect multiple services/teams — not for routine implementation choices (see [../commands/architecture.md](../commands/architecture.md) for when this threshold is met).
- Every accepted ADR gets a corresponding summary entry in `memory/project-decisions.md` — the ADR is the detailed record, memory is the searchable index.
- If a later decision reverses this one, don't edit this ADR's Decision section — mark it Superseded and write a new ADR, preserving the historical reasoning.
