# Testing Plan Template

```markdown
## Feature/Change Under Test


## Test Levels

### Unit Tests
- [ ] <case: happy path>
- [ ] <case: boundary — zero/empty/max>
- [ ] <case: nil/invalid input>

### Integration Tests
- [ ] <case: real Postgres/TimescaleDB interaction, if applicable>
- [ ] <case: cross-service call, if applicable — e.g. gateway → orchestrator>

### API Tests (if applicable)
- [ ] Happy path (2xx)
- [ ] Missing/invalid auth (401)
- [ ] Validation failure (400)
- [ ] Rate limit exceeded (429)
- [ ] Tenant isolation — cannot access another tenant's resource (403/404)

### Regression Tests (if this is a bug fix)
- [ ] Test fails on pre-fix code, passes on fixed code
- [ ] Named/commented with the bug it prevents

### Load/Performance (if hot path)
- [ ] Suggested load test: <tool + target, e.g. "hey -z 30s -c 50 http://localhost:8080/v1/jobs">
- [ ] Baseline measurement taken before change
- [ ] Target: <specific p50/p99 or throughput number>

## Out of Scope
<What's explicitly not covered by this plan and why>
```

## Usage Notes
- A testing plan without a tenant-isolation case for any tenant-scoped feature is incomplete — this is the #1 risk class in this codebase (see [../knowledge/security-notes.md](../knowledge/security-notes.md)).
- See [../commands/test.md](../commands/test.md) for the full testing workflow this plan feeds into.
