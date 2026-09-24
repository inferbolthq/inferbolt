# /git — Git Workflow Assistance

## Purpose
Handle git operations following [../CLAUDE.md](../CLAUDE.md) §7 (Conventional Commits, small commits, feature branches) and the project's actual commit history style.

## Inputs
- The git action needed: commit message drafting, branch strategy, resolving a merge conflict, preparing a PR description.

## Outputs
- A Conventional Commits-formatted message matching the existing style (`fix: set river job timeout to 60 minutes`, `feat: ...`).
- A clean, reviewable diff — split into logical commits if the working tree mixes unrelated changes.

## Workflow
1. Run `git status` and `git diff` before proposing any commit — never assume what's staged.
2. Group changes into logical, independently revertable commits rather than one mixed commit, when the working tree contains unrelated changes.
3. Write commit messages in Conventional Commits format, focused on *why*, matching the terse, lowercase style already used in this repo's history.
4. For PRs: summarize the *why* in 1-3 bullets, include a test plan checklist, follow the existing PR template (`templates/pull-request.md`).
5. Never force-push, rebase interactively, reset --hard, or skip hooks without explicit user confirmation for that specific action.
6. For merge conflicts: understand both sides' intent before resolving — don't default to "ours" or "theirs" without checking which is actually correct.

## Examples
```
/git draft a commit message for the drift baseline recomputation change
/git split these changes into logical commits
/git prepare a PR description for the SGLang adapter work
```

## Best practices
- Match this repo's existing commit tone: short, lowercase, imperative, prefixed (`fix:`, `feat:`, `refactor:`, `chore:`).
- Never include unrelated changes (formatting-only diffs, unrelated file touches) in a commit meant to convey one logical change.
- Always confirm before any destructive or history-rewriting git operation — see the Git Safety Protocol in the top-level agent instructions.
