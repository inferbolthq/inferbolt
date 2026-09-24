# Testing

## Current State
- Go: table-driven tests colocated with source (`internal/drift/detector_test.go`, `internal/drift/baseline_test.go`, `internal/gateway/handlers_test.go`, `internal/gateway/middleware_test.go`), using `stretchr/testify` for assertions.
- Python: `pytest` + `pytest-asyncio` (`asyncio_mode = "auto"`), tests under `worker/tests/` per `pyproject.toml`.
- Run via `make test` (Go) — see `Makefile` — and `uv run pytest` (worker).

## What's Covered vs. Not
Drift detection and gateway handlers/middleware have colocated tests as of the most recent commits. Newer areas (orchestrator wiring, River job dispatch, the operator's reconciliation loop) should be checked for test coverage before being extended — don't assume parity with the more mature gateway/drift packages. Confirm current coverage with `go test ./... -cover` rather than trusting this doc's snapshot.

## Integration Testing Approach
Use the `docker-compose.yml` Postgres/TimescaleDB service for tests that need real query behavior (index usage, JSONB queries, hypertable behavior) — don't mock the database for correctness-of-query tests. Mock only at genuine architectural seams (`OrchestratorClient`, `engine.Engine`).

## API Testing
Every handler test should exercise: happy path, missing/invalid auth, validation failure (e.g., `workload.concurrency <= 0`), and rate-limit-exceeded behavior — see `internal/gateway/handlers_test.go` and `middleware_test.go` for the existing pattern to extend.

## Load Testing
No load-testing tooling is currently wired into CI. When a change touches a hot path (gateway request handling, orchestrator dispatch), call out in the PR what a reasonable load test would check (e.g., `hey -z 30s -c 50 http://localhost:8080/v1/jobs`) even if it's not automated yet — see [../commands/performance.md](../commands/performance.md).

## Regression Test Discipline
Every bug fix ships with a test that fails on the pre-fix code and passes after. Name the test after the bug/scenario, not just the function under test, so future readers know why it exists.

## Gaps to Track
Record any discovered coverage gap in `memory/technical-debt.md` rather than silently accepting it — especially around: River job retry/idempotency behavior, operator reconciliation edge cases, and Python engine adapter failure paths (engine unavailable, malformed config).
