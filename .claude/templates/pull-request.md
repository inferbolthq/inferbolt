# Pull Request Template

```markdown
## Summary
<1-3 bullet points on what changed and why — focus on the "why," the diff already shows the "what">

## Changes
- <bullet per logical change>

## Architecture / Design Notes
<Only if this PR involves a non-trivial trade-off or deviates from an existing pattern — link an ADR if one was written>

## Testing
- [ ] Unit tests added/updated for new logic
- [ ] Integration tests added/updated (if touching Postgres/TimescaleDB)
- [ ] `make test` passes locally
- [ ] Manually verified: <describe the manual check, e.g. "hit POST /v1/jobs with a valid and invalid payload">

## Security / Performance Considerations
<Call out explicitly if this touches auth, tenant isolation, a hot path, or a new dependency — "none" is a valid answer, but state it>

## Rollout / Migration Notes
<Migration ordering, config changes required, backward-compatibility notes — "none" is valid but state it>

## Screenshots (if UI change)

## Related Issues / Context
Closes #
```

## Usage Notes
- Keep the Summary section to what a reviewer needs to decide whether to dig into the diff — not a restatement of every line changed.
- Testing and Rollout sections are not optional boilerplate — a PR without them is missing information the reviewer needs, per [../CLAUDE.md](../CLAUDE.md) §6-7.
