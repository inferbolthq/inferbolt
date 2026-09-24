# Workflow: Incident Response

## Inputs
- An active incident: alert fired, user report, or manually observed degradation/outage.

## Outputs
- Mitigated incident, `templates/incident-report.md` filled in, and (for SEV1/2) a follow-up postmortem.

## Step-by-Step Process
1. **Assess severity and impact** — which services, which tenants, user-visible symptoms. Assign SEV level.
2. **Mitigate first, root-cause second** — stop the bleeding (rollback, restart, disable a feature flag/route) before fully understanding the root cause, unless mitigation itself is genuinely risky.
3. **Communicate status** — keep the incident report's timeline updated in real time, not reconstructed afterward from memory.
4. **Stabilize** — confirm health checks green, error rates back to baseline, no secondary effects (e.g., a queue backlog from the outage window draining without issue).
5. **Root-cause once stable** — apply [../workflows/bug-fixing.md](bug-fixing.md) process for the underlying defect.
6. **File the incident report** (`templates/incident-report.md`) with an accurate timeline.
7. **For SEV1/SEV2, schedule a blameless postmortem** (`templates/postmortem.md`) with tracked action items.
8. **Update `memory/known-bugs.md`** and `context/known-issues.md` if the incident reveals a standing gap.

## Review Checklist
- [ ] Severity/impact assessed accurately, not minimized
- [ ] Mitigation applied before full root-cause analysis (unless mitigation was itself risky)
- [ ] Timeline captured while fresh
- [ ] Regression test added for the root cause
- [ ] Postmortem scheduled for SEV1/2

## Success Criteria
Impact is stopped quickly, the incident is accurately documented, and the underlying defect is fixed with a regression test — not just the symptom mitigated.
