# /implement — Feature Implementation

## Purpose
Build a new feature end-to-end: production code, tests, docs, and the accompanying design reasoning — following [workflows/feature-development.md](../workflows/feature-development.md).

## Inputs
- A feature description or ticket-equivalent (what the user needs, not just "add a function").
- Relevant constraints: tenant tiers affected, performance targets, which service(s) it touches.

## Outputs
- Working, tested code across all touched layers (handler → domain → persistence, or CLI → config → execution).
- Tests: unit + integration where persistence/network is involved.
- Doc updates (`context/`, README) if the feature changes public behavior.
- A short design note if the feature involved a non-obvious trade-off (candidate for `templates/adr.md`).

## Workflow
1. Clarify requirements that are ambiguous — don't assume defaults on things like rate limits, tenant scoping, or error behavior.
2. State the approach before writing code: which packages are touched, what the data flow looks like, what the interface boundary is.
3. Check `context/architecture.md` and `memory/project-decisions.md` for consistency — does this fit existing patterns (e.g., River for async work, `pgx` for persistence, the shared response envelope)?
4. Implement bottom-up or top-down consistently — usually: data model/migration → domain logic → transport layer → tests at each layer.
5. Add validation and auth/rate-limiting at the boundary per [../CLAUDE.md](../CLAUDE.md) §2 and §5.
6. Write tests alongside the code, not after — table-driven Go tests, pytest for the worker.
7. Self-review against §3 before declaring done (or run `/review`).
8. Update `memory/completed-features.md`, `memory/active-tasks.md`, and relevant `context/` files.

## Examples
```
/implement rate limit override endpoint for enterprise tenants
/implement SGLang engine adapter matching the vLLM adapter's interface
/implement drift baseline recomputation as a scheduled River job
```

## Best practices
- A feature isn't done at "it compiles" — it's done at "it's tested, documented, and consistent with the rest of the system."
- If the feature touches a hot path (gateway request handling, orchestrator dispatch), explicitly run through the Performance Checklist (§4) before finishing.
- If two reasonable designs exist, present the trade-off briefly and pick one rather than silently deciding without surfacing it — this is a judgment call, not every micro-decision needs a discussion.
