---
name: release-cut
description: Use when the user asks to cut/prepare a release, tag a version, or generate release notes for this repo. Walks through determining the correct semantic version bump from actual changes since the last tag, drafting release notes, and confirming CI is green before tagging — following workflows/release-process.md and the repo's .goreleaser.yaml-driven release pipeline. Trigger on: "cut a release", "tag a version", "prepare release notes", "bump the version", "publish a new version".
---

# Release Cut

Concrete procedure for `../../workflows/release-process.md`, specific to this repo's tooling (`.github/workflows/release.yaml`, `.goreleaser.yaml`).

## Procedure
1. **Find the last tag**: `git describe --tags --abbrev=0`. If none exists, treat this as the first tagged release and confirm the intended starting version (typically `v0.1.0`) with the user rather than assuming.
2. **Review changes since that tag**: `git log <last-tag>..HEAD --oneline`. Categorize each commit by its Conventional Commits prefix (`feat:`, `fix:`, `refactor:`, `docs:`, `chore:`, etc.).
3. **Determine the version bump:**
   - Any breaking config/API/proto change (see `../../commands/api.md` and `proto-sync` skill's versioning note) → **MAJOR**.
   - Any `feat:` commit with no breaking change → **MINOR**.
   - Only `fix:`/`refactor:`/`chore:`-type commits → **PATCH**.
   - When in doubt, state the ambiguity to the user rather than silently picking the lower-risk-sounding option — an under-bumped version can mislead a self-hosting user about compatibility.
4. **Draft release notes** using `../../templates/release-notes.md` — every breaking change entry must include concrete upgrade guidance (config changes, migration steps), not just a description of what changed.
5. **Cross-check against `../../memory/completed-features.md` and `../../memory/known-bugs.md`** to make sure nothing significant that shipped in this window is missing from the notes, and that fixed bugs are described from the user's perspective, not just the internal fix description.
6. **Confirm CI is green** on the exact commit intended for the tag (`.github/workflows/ci.yaml`) — never tag a commit that hasn't passed the build+test gate.
7. **Confirm with the user before tagging/pushing** — tagging and triggering `.github/workflows/release.yaml` (which builds and publishes via `.goreleaser.yaml`) is a hard-to-reverse, externally-visible action; this is not something to do unprompted or without explicit confirmation of the exact version number.
8. **After release**, update `../../memory/completed-features.md` with what shipped in this version, tagged with the version number.

## Common Mistakes This Guards Against
- Bumping PATCH for a change that's actually a breaking config change, misleading users about upgrade safety.
- Tagging a commit that hasn't passed CI.
- Writing release notes from commit-message paraphrasing alone instead of verifying against the actual diff and `memory/completed-features.md`.
- Tagging/pushing a release without explicit user confirmation, since this triggers an irreversible, externally-visible publish action.
