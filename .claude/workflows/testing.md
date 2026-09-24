# Workflow: Testing

## Inputs
- New/changed code needing tests, or an existing suite needing a gap review.

## Outputs
- Table-driven Go tests or pytest tests matching existing conventions.
- A gap analysis if reviewing existing coverage.

## Step-by-Step Process
1. **Identify what's actually being verified** — behavior/contracts, not implementation details that make tests brittle to harmless refactors.
2. **Enumerate cases**: happy path, boundaries (zero/negative/max), nil/empty, concurrent access (if relevant), auth/validation/rate-limit failure (for handlers).
3. **Match existing style**: table-driven + `testify` for Go, `pytest`/`pytest-asyncio` for the worker.
4. **Use real Postgres/TimescaleDB** (via `docker-compose.yml`) for query-correctness integration tests; mock only at true architectural seams.
5. **Verify assertions are meaningful** — flag any test that only checks "no error" as a gap.
6. **Run the full suite** (`make test`, `pytest`) before considering the task done.

## Review Checklist
- [ ] Every new branch/condition has a test
- [ ] Edge cases enumerated, not just happy path
- [ ] Auth/validation/rate-limit cases covered for new endpoints
- [ ] No skipped/commented-out/vacuous tests
- [ ] Regression tests named after the bug they prevent

## Success Criteria
`make test`/`pytest` passes; new code's behavior is pinned down by tests that would catch a real regression, not just tests that exist for coverage-metric appearances.
