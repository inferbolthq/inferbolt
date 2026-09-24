# Postmortem Template (Blameless)

```markdown
# Postmortem: <incident title>

**Date of incident:** <YYYY-MM-DD>
**Author(s):**
**Status:** Draft / Reviewed / Actions Tracked

## Summary
<2-3 sentences: what happened, impact, resolution>

## Impact
<Quantified: duration, tenants/requests affected, any data implications>

## Root Cause Analysis
<The technical chain of events leading to the incident — use "5 whys" if the first-level cause isn't the real root cause. Blameless: focus on system/process gaps, not individual fault.>

## What Went Well
<Detection speed, mitigation effectiveness, communication — give credit where due>

## What Went Poorly
<Gaps in detection, mitigation, communication, or the system design itself>

## Where We Got Lucky
<Anything that could have made this worse but didn't, by chance — these are hidden risks worth addressing proactively>

## Action Items
| Action | Owner | Priority | Status |
|---|---|---|---|
| | | | |

(Every action item should trace to a specific gap identified above — no generic "improve monitoring" without specifics.)

## Lessons for the System
<Feed into `memory/technical-debt.md`, `memory/known-bugs.md`, and `context/known-issues.md` as appropriate>
```

## Usage Notes
- Blameless means the analysis targets systems and processes, not people — "the on-call didn't check X" reframes to "the runbook didn't specify checking X."
- Action items without an owner and priority don't get done — don't let this postmortem end without both.
- This is a living record — update `Status` on action items until they're all closed, don't let the document go stale immediately after publication.
