# Release Notes Template

```markdown
# vX.Y.Z — <YYYY-MM-DD>

## Breaking Changes
- <none, or explicit list with migration guidance>

## New Features
- <feature — one line, user-facing framing>

## Improvements
- <perf/UX/reliability improvement>

## Bug Fixes
- <bug — one line, what was wrong and now isn't>

## Deprecations
- <anything being phased out, with removal timeline>

## Upgrade Notes
<Migration steps, config changes required, anything an operator needs to do before/during/after upgrading>

## Internal / Technical Debt
<Optional — only if relevant to downstream consumers, e.g. a dependency bump with security implications>
```

## Usage Notes
- Version bump follows semantic versioning per [../CLAUDE.md](../CLAUDE.md) §7: breaking config/API changes → MAJOR, additive features → MINOR, fixes → PATCH.
- Generated alongside `.goreleaser.yaml`-driven releases — write these as changes land (see `memory/completed-features.md`), not reconstructed from git log at release time from memory alone.
- Every "Breaking Changes" entry must include concrete upgrade guidance, not just "this changed."
