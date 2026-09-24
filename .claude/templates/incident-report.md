# Incident Report Template

```markdown
## Incident Summary
**Severity:** SEV1 / SEV2 / SEV3
**Start time:** <timestamp, UTC>
**Detection time:** <when it was noticed — gap between start and detection is itself a signal>
**Resolution time:** <timestamp>
**Duration:** <total impact window>

## Impact
- Services affected: <gateway / orchestrator / router / collector / operator / worker>
- Tenants affected: <specific / all / estimated %>
- User-visible symptoms: <errors seen, degraded behavior, data impact>

## Timeline (UTC)
| Time | Event |
|---|---|
| | Incident began (or first plausible root-cause trigger) |
| | Detected — how (alert, user report, manual observation) |
| | Mitigation action taken |
| | Resolved |

## Root Cause
<The actual underlying cause — not just "the server was down," but why>

## Detection
<How was this caught? If it wasn't caught by monitoring/alerting, that's itself a finding — note it>

## Mitigation
<What stopped the bleeding — may differ from the permanent fix>

## Follow-Up Actions
- [ ] <action item> — owner — due date
- [ ] Regression test added (link)
- [ ] Monitoring/alert added or improved (link)

## Related
- `memory/known-bugs.md` entry:
- Postmortem (for SEV1/SEV2): `templates/postmortem.md`
```

## Usage Notes
- Write this while details are fresh — timeline accuracy degrades fast.
- Every SEV1/2 incident report should graduate to a full postmortem; SEV3 may not need one, use judgment.
