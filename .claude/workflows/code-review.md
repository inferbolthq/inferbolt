# Workflow: Code Review

## Inputs
- A diff, PR, or file set to review.

## Outputs
- Severity-ranked findings (Blocking / Should-fix / Consider).
- A clear ship/ship-with-fixes/needs-rework verdict.

## Step-by-Step Process
1. **Scope the diff.** `git status`/`git diff --stat` — review what's actually in scope, not the whole repo.
2. **Read with context.** Pull in enough surrounding code (callers/callees) to judge correctness, not just the changed lines in isolation.
3. **Walk the full checklist** ([../CLAUDE.md](../CLAUDE.md) §3): bugs, performance, readability, maintainability, security, edge cases, error handling, memory leaks, concurrency, scalability, accessibility (frontend), test coverage.
4. **Cross-check consistency** against `context/architecture.md`, `context/design-decisions.md`, and `memory/project-decisions.md` — flag contradictions rather than silently accepting or silently "fixing" them.
5. **Check against known-fragile areas** — `memory/known-bugs.md`, `memory/technical-debt.md`.
6. **Verify test coverage** for new/changed behavior, and that tests assert real behavior (not vacuous).
7. **Rank findings**: Blocking (bug/security/data-loss) → Should-fix (perf/maintainability/missing test) → Consider (style/naming).
8. **Give a clear verdict** and, if requested, apply fixes directly.

## Review Checklist
(See [../CLAUDE.md](../CLAUDE.md) §3 for the full enumerated list — applied every time, not selectively.)

## Success Criteria
Every Blocking-severity issue is caught before merge; findings are specific (file:line + concrete fix), not vague; the review doesn't rubber-stamp.
