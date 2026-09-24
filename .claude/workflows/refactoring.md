# Workflow: Refactoring

## Inputs
- A named code smell or maintainability pain point (not "clean this up" without specifics).

## Outputs
- Behavior-preserving structural improvement.
- Confirmation that existing tests still pass; new tests if untested behavior was exposed.

## Step-by-Step Process
1. **Confirm test coverage exists** for current behavior before touching anything. If it doesn't, write characterization tests first.
2. **Name the smell explicitly** (duplication, long function, feature envy, inappropriate coupling) — not just "this feels messy."
3. **Make the smallest change that removes the smell** — extract a function/interface rather than introducing a new framework/pattern.
4. **Run tests after each meaningful step**, not only at the end.
5. **Diff the public behavior surface** (API responses, CLI output, exported function signatures) — must be unchanged unless the refactor was explicitly scoped to include a behavior change.
6. **Update `memory/technical-debt.md`** — resolve or note partial progress on the relevant entry.

## Review Checklist
- [ ] Test coverage existed or was added before refactoring started
- [ ] Smell is named, not vague
- [ ] Change is scoped to the smell — no unrelated "while I was in there" edits
- [ ] Full test suite passes after
- [ ] Public behavior surface unchanged (or the change was explicit and approved)

## Success Criteria
Structure improves; behavior is provably unchanged (tests passing before and after); scope stayed disciplined.
