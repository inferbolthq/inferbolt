# /docs — Documentation Generation & Maintenance

## Purpose
Generate or update production-quality documentation: README sections, architecture docs, API docs, runbooks — kept in sync with actual code behavior.

## Inputs
- Target: which doc (README, `context/architecture.md`, API reference, a runbook) or which recent change needs documenting.

## Outputs
- Updated documentation that reflects current, verified behavior — not aspirational behavior.
- Explicit callouts where docs and code have diverged (see `context/known-issues.md` for the current README/implementation drift).

## Workflow
1. Read the actual code/config the doc claims to describe — never document from memory or assumption.
2. Identify the audience (new contributor, operator running this in production, external API consumer) and write for that reader specifically.
3. Prefer concrete examples (real request/response, real config values) over abstract description.
4. Cross-link related docs (`context/`, `knowledge/`) instead of duplicating content.
5. For API docs: verify against the actual handler/proto definition, including error responses and rate-limit behavior, not just the happy path.
6. Flag and fix stale content you encounter along the way, even if it's outside the immediate ask — but call it out rather than silently rewriting unrelated sections.

## Examples
```
/docs update README config tables to match internal/config/gateway.go
/docs write context/api-contracts.md for the /v1/jobs endpoint family
/docs generate a runbook for drift-detector false-positive alerts
```

## Best practices
- Docs describing config must list defaults and env var names verbatim from the source, not paraphrased.
- Every doc claiming a roadmap/status ("coming soon", "in progress") must be checked against actual code state before publishing — don't propagate stale roadmap claims (see the current README roadmap vs. actual implemented features).
- Runbooks are for 3am-on-call — action-first, no narrative preamble.
