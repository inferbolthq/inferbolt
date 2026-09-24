# Workflow: Release Process

## Inputs
- A set of merged changes ready to cut a release.

## Outputs
- A tagged release with generated binaries (via `.goreleaser.yaml`), semantic version bump, and release notes.

## Step-by-Step Process
1. **Review changes since last tag** (`git log <last-tag>..HEAD`) — categorize into breaking/feature/fix per Conventional Commits prefixes.
2. **Determine version bump**: any breaking config/API change → MAJOR; additive feature → MINOR; fix-only → PATCH (see [../CLAUDE.md](../CLAUDE.md) §7).
3. **Write release notes** using `templates/release-notes.md` — every breaking change needs concrete upgrade guidance.
4. **Confirm CI is green** on the release commit before tagging.
5. **Tag and let `.goreleaser.yaml`-driven CI build/publish binaries** via `.github/workflows/release.yaml`.
6. **Update `memory/completed-features.md`** with what shipped in this release.

## Review Checklist
- [ ] Version bump matches actual change severity, not just "felt like a minor bump"
- [ ] Every breaking change has upgrade guidance
- [ ] CI green on the exact commit being tagged
- [ ] Release notes reviewed for accuracy against the actual diff, not just commit-message paraphrasing

## Success Criteria
A tagged, built, documented release that a self-hosting user can safely upgrade to by following the release notes alone.
