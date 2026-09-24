# /refactor — Safe Refactoring

## Purpose
Improve internal structure (readability, duplication, coupling, complexity) **without changing external behavior**. Every refactor must be provably behavior-preserving.

## Inputs
- Target: file, package, or function to refactor.
- Optional: the specific smell driving it ("this handler does three things", "duplicated across router.go and collector.go").

## Outputs
- A restructured version of the target with an explanation of what changed and why.
- Confirmation that existing tests still pass; new tests added if the refactor exposed an untested branch.
- A note on what was deliberately *not* touched (scope discipline).

## Workflow
1. Confirm test coverage exists for current behavior *before* touching code. If it doesn't, write characterization tests first — you cannot safely refactor what you can't verify.
2. Identify the specific smell (duplication, long function, feature envy, primitive obsession, inappropriate coupling) — name it, don't just "clean up."
3. Make the smallest change that removes the smell. Prefer extracting a function/interface over introducing a new framework or pattern.
4. Run the full test suite (`make test` / `pytest`) after each meaningful step, not just at the end.
5. Diff the public behavior surface (API responses, CLI output, function signatures used elsewhere) — it must be unchanged unless the refactor was explicitly scoped to include an API change.
6. Update `memory/technical-debt.md` — remove the item if this refactor resolved it, or note partial progress.

## Examples
```
/refactor internal/gateway/handlers.go — split the monolithic ServeHTTP into per-route handlers
/refactor extract shared rate-limit logic between IPLimiter and TenantLimiter
/refactor worker/engines — reduce duplication between vllm_engine.py and the mock engine
```

## Best practices
- Refactor and feature work are different PRs. Never bundle "also renamed this while I was in there" with unrelated feature changes.
- Don't refactor code you're not also testing — that's how silent regressions ship.
- Stop at the boundary of the requested scope; a refactor that snowballs into a rewrite needs a conversation first, not silent expansion.
- Prefer reducing abstraction over adding it, unless the duplication is genuinely load-bearing (3rd occurrence, confirmed pattern).
