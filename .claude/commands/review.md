# /review — Code Review

## Purpose
Run a Staff-Engineer-grade review of a diff, PR, or file against the standards in [../CLAUDE.md](../CLAUDE.md) §3 (Code Review Standards). Produces actionable, prioritized findings — not a rubber stamp.

## Inputs
- Target: a git diff (`git diff`, `git diff main...HEAD`), a specific PR number, or an explicit file/path list.
- Optional: focus area (e.g., "just security", "just the new query paths").

## Outputs
- A prioritized findings list: **Blocking** (bugs, security, data loss) → **Should fix** (perf, missing tests, maintainability) → **Consider** (style, naming, minor readability).
- Each finding: file:line, what's wrong, why it matters, concrete fix (not just "consider fixing this").
- A one-line verdict: ship / ship with fixes / needs rework.

## Workflow
1. Identify the diff scope (`git status`, `git diff --stat`) — don't review the whole repo when only a diff was asked for.
2. Read changed files with enough surrounding context to understand callers/callees, not just the changed lines.
3. Walk the checklist in [../CLAUDE.md](../CLAUDE.md) §3: bugs, performance, readability, maintainability, security, edge cases, error handling, memory leaks, concurrency, scalability, accessibility (frontend), test coverage.
4. Cross-check against `context/architecture.md` and `context/design-decisions.md` — does this diff contradict an established decision? Flag it rather than silently accept or silently "fix" it.
5. Check `memory/known-bugs.md` and `memory/technical-debt.md` — does this change touch known-fragile code?
6. Verify tests exist for new/changed behavior and actually assert something (not `assert True`).
7. Report findings ranked by severity; do not soften blocking issues to be polite.
8. If invoked with `--fix`, apply the Blocking and Should-fix items directly, then re-list what was changed.

## Examples
```
/review
/review --focus security
/review internal/gateway/handler/jobs.go
/review PR 42
```

## Best practices
- A review that finds nothing is suspicious — re-read before reporting a clean bill of health.
- Don't nitpick style that a formatter (`gofmt`, `ruff`) already enforces.
- Always state *why* a finding matters (failure scenario), not just that it's a deviation from a rule.
- Prefer one strong, correct finding over five speculative ones.
- If the diff is large, review it in logical chunks (by package/feature) rather than skimming everything shallowly.
