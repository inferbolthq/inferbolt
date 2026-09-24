# /test — Test Generation & Review

## Purpose
Write or review tests against [../CLAUDE.md](../CLAUDE.md) §6 (Testing Standards): unit, integration, API, edge case, and regression coverage.

## Inputs
- Target: a file/function/endpoint needing tests, or an existing test suite to review for gaps.

## Outputs
- Table-driven Go tests (matching `detector_test.go`/`handlers_test.go` style) or pytest tests (matching `worker/tests/` conventions) as appropriate.
- A gap analysis if reviewing existing tests: what's untested, what's tested but weakly (no real assertions).

## Workflow
1. Identify what's actually being verified — behavior and contracts, not implementation details that would make the test brittle to harmless refactors.
2. Enumerate cases: happy path, boundary values (zero, negative, max), nil/empty, concurrent access where relevant, auth/validation failure for handlers.
3. For Go: prefer table-driven tests; use `testify` (already a dependency) for assertions, matching existing style.
4. For handlers: test through the actual routing/middleware where feasible so auth and validation are exercised, not bypassed.
5. For anything touching Postgres/TimescaleDB: use the `docker-compose.yml` services for integration tests rather than mocking the database when testing query correctness; mock only at true architectural seams (e.g., `OrchestratorClient`).
6. For the Python worker: cover engine adapter behavior including failure paths (engine unavailable, malformed model config).
7. Flag any test that doesn't assert anything meaningful (e.g., only checks "no error") as a gap, not a pass.

## Examples
```
/test internal/drift/detector.go — add edge cases for zero-sample baselines
/test the POST /v1/jobs handler — add rate-limit-exceeded case
/test worker/engines/vllm_engine.py failure paths
```

## Best practices
- A regression test names the bug it prevents in its test name/comment — future readers need to know why it exists.
- Don't test framework/library behavior (e.g., that `chi` routes correctly) — test this codebase's logic.
- Integration tests are slower; keep the fast unit-test suite comprehensive so `make test` stays usable in a tight inner loop.
