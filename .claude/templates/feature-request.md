# Feature Request Template

```markdown
## Problem
<What need isn't currently met? Who has this problem — which tenant tier/persona?>

## Proposed Solution
<High-level description — not necessarily the final implementation approach>

## Alternatives Considered
<At least one alternative and why it was or wasn't chosen — even a brief "did X, rejected because Y">

## Scope
- In scope:
- Explicitly out of scope:

## Tenant Tier Impact
<Does this change rate limits, feature availability, or behavior differently per tier (oss/cloud_free/cloud_paid/enterprise)?>

## Success Criteria
<How do we know this is done and working — specific, checkable>

## Dependencies / Risks
<Anything this blocks on, or anything it puts at risk (e.g., touches auth, touches a hot path)>

## Related
- Roadmap item: `context/roadmap.md`
- ADR (if a significant design decision is involved): `templates/adr.md`
```

## Usage Notes
- A feature request without a clear "who has this problem" is underspecified — clarify before scoping implementation, per [../commands/plan.md](../commands/plan.md).
- Scope section exists specifically to prevent creep — state what's explicitly NOT being done, not just what is.
