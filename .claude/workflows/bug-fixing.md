# Workflow: Bug Fixing

## Inputs
- Bug report (see `templates/bug-report.md`) or direct symptom description.
- Reproduction steps, logs, or failing test.

## Outputs
- Root-cause fix (not a symptom patch).
- Regression test.
- `memory/known-bugs.md` entry, updated to "fixed" with commit reference.

## Step-by-Step Process
1. **Reproduce.** If you can't, say so and ask for more detail rather than guessing — per [../commands/debug.md](../commands/debug.md).
2. **Log the bug.** Add an entry to `memory/known-bugs.md` immediately, even before it's fixed.
3. **Trace root cause.** Follow the actual execution path; don't pattern-match on the error string alone.
4. **Write the regression test first** — it should fail against the current (buggy) code.
5. **Fix at the root cause**, not the symptom — no defensive null-checks papering over invalid state entering the system.
6. **Verify the regression test now passes**, and that no other tests broke.
7. **Check for the same bug class elsewhere** (sibling handler, similar pattern in another engine adapter) — mention it even if out of scope to fix now; add to `memory/technical-debt.md` if deferred.
8. **Update `memory/known-bugs.md`** to fixed status with commit reference and regression-test location.

## Review Checklist
- [ ] Reproduced (or explicitly noted as not reproducible with next steps to get there)
- [ ] Root cause identified and explained, not just patched
- [ ] Regression test added, verified to fail-then-pass
- [ ] Related occurrences of the same bug class checked
- [ ] `memory/known-bugs.md` updated

## Success Criteria
The specific bug can't recur silently (regression test exists), and the underlying cause — not just its symptom — is gone.
