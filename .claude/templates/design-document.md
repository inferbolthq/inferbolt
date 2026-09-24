# Design Document Template

```markdown
# Design Doc: <feature/system name>

**Author(s):**
**Status:** Draft / In Review / Approved / Implemented
**Date:**

## Problem Statement
<What need is this solving, for whom — cite the actual tenant tier(s)/persona(s) if relevant>

## Goals
- <explicit, checkable goal>

## Non-Goals
- <explicitly out of scope — as important as the goals list>

## Background / Current State
<What exists today and why it's insufficient — link to relevant `context/` docs>

## Proposed Design
<The core design — diagrams welcome. Describe data flow, new/changed components, API surface.>

## Alternatives Considered
1. **<Option>** — why not chosen
2. **<Option>** — why not chosen

## Scalability & Performance
<Expected load, how this scales, what breaks first under 10x load>

## Security & Privacy
<Auth/authz model, tenant isolation approach, data sensitivity>

## Reliability & Failure Modes
<What happens when each dependency fails; rollback strategy>

## Testing Strategy
<Link to `templates/testing-plan.md` filled in for this feature>

## Rollout Plan
<Phased rollout, feature flags, migration sequencing>

## Open Questions
<Anything unresolved at doc-review time>

## Related
- ADR (if a specific decision within this design warrants one): `templates/adr.md`
- Roadmap entry: `context/roadmap.md`
```

## Usage Notes
- Reserve full design docs for genuinely significant work (new service, major API surface, cross-cutting architecture change) — see [../commands/architecture.md](../commands/architecture.md) and [../commands/plan.md](../commands/plan.md) for lighter-weight alternatives when a full doc is overkill.
- The Non-Goals section is not optional — it's often what prevents scope creep during implementation.
