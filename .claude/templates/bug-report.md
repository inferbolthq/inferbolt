# Bug Report Template

```markdown
## Summary
<one sentence: what's wrong>

## Environment
- Service/component: <e.g. gateway, orchestrator, worker, operator>
- Deployment: <local docker-compose / k8s cluster / which environment>
- Version/commit: <git SHA or release tag>

## Steps to Reproduce
1.
2.
3.

## Expected Behavior


## Actual Behavior


## Evidence
- Error message / stack trace:
- Relevant logs (redact tenant-sensitive data):
- Request/response payload (redact secrets):

## Impact
- Tenant(s) affected: <specific tenant_id(s), "all", or "unknown">
- Severity: <Critical / High / Medium / Low — see /security severity bar for what "Critical" means>
- Frequency: <always / intermittent — include rough rate if known>

## Suspected Root Cause (if known)


## Related
- Known bug entry: `memory/known-bugs.md`
- Related PR/commit:
```

## Usage Notes
- A bug report without reproduction steps is a research task, not a bug report — see [../commands/debug.md](../commands/debug.md) for how to proceed when repro is unclear.
- Always redact tenant-sensitive payloads and secrets before pasting logs/payloads into a bug report, per [../CLAUDE.md](../CLAUDE.md) §5 (Logging sensitive data).
