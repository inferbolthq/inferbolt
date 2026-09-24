# Workflow: Feature Development

## Inputs
- Feature request (see `templates/feature-request.md`) or direct user ask.
- Relevant `context/` docs (architecture, api-contracts, database-schema) and `memory/active-tasks.md`.

## Outputs
- Working, tested, documented feature merged to a feature branch.
- Updated `memory/completed-features.md`, `memory/active-tasks.md`, relevant `context/` docs.
- Session summary per [../CLAUDE.md](../CLAUDE.md) §14.

## Step-by-Step Process
1. **Clarify.** Restate the requirement; ask about ambiguous points (tenant-tier impact, performance target, scope boundary) rather than assuming.
2. **Check context.** Read `memory/current-state.md`, `memory/project-decisions.md`, and the relevant `context/*.md` files — confirm consistency with existing architecture before designing.
3. **Plan.** For non-trivial work, run `/plan` — sequence of steps, called-out trade-offs, risk points.
4. **Implement bottom-up (typical shape):** migration (if needed) → domain logic → transport layer (HTTP/gRPC/CLI) → tests at each layer, per [../commands/implement.md](../commands/implement.md).
5. **Self-review.** Run `/review` against the full checklist ([../CLAUDE.md](../CLAUDE.md) §3) before considering it done.
6. **Test.** Confirm `make test` (and `pytest` if worker touched) passes; confirm new edge cases are covered.
7. **Document.** Update README/`context/` docs affected by the change in the same PR.
8. **Update memory.** `memory/completed-features.md`, `memory/active-tasks.md`, `memory/technical-debt.md` (if any accepted), `memory/project-decisions.md` (if a real decision was made).
9. **Summarize.** Give the user a concise summary: what was built, decisions made, debt introduced, recommended next step — per [../CLAUDE.md](../CLAUDE.md) §14.

## Review Checklist
- [ ] Requirement was clarified, not assumed
- [ ] Consistent with existing architecture/patterns
- [ ] Tests cover happy path + edge cases + (if applicable) tenant isolation
- [ ] Docs updated in the same change
- [ ] Memory updated

## Success Criteria
Feature works end-to-end, is tested, is documented, and leaves the codebase at least as consistent as it found it.
