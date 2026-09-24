# Contributing to InferBolt

Thanks for considering a contribution. This document covers how to get a working
development environment, what the review bar is, and the conventions this codebase
already follows.

## Table of contents

- [Getting set up](#getting-set-up)
- [Running things](#running-things)
- [Tests](#tests)
- [Coding conventions](#coding-conventions)
- [Commits and pull requests](#commits-and-pull-requests)
- [Database migrations](#database-migrations)
- [Reporting bugs and requesting features](#reporting-bugs-and-requesting-features)

## Getting set up

You need **Go 1.24+**, **Python 3.11+**, and **Docker** with Compose.

```bash
git clone https://github.com/inferbolthq/inferbolt
cd inferbolt

go mod download
pip install -e '.[dev]'     # Python worker plus pytest

make build                   # compiles every cmd/ binary into bin/
```

Optional but recommended, since CI enforces both:

```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
pip install ruff
```

## Running things

The fastest path to a working stack is the one-command benchmark, which starts the
services, spawns a worker, and runs a benchmark against the mock engine. It needs no
GPU:

```bash
inferbolt run test-model --gpu cpu --engines mock
```

For the optimization agent you also need an Anthropic API key, which belongs to the
**orchestrator**, not the client:

```bash
export ANTHROPIC_API_KEY=...
docker compose up -d
inferbolt agent "find the best throughput config" \
  --model test-model --gpu cpu --engines mock --max-trials 4
```

Other useful targets: `make start` / `make stop` / `make logs` for the Compose stack,
`make clean` to tear down volumes.

> **Migrations only run on a fresh database.** Compose mounts `migrations/` at
> `/docker-entrypoint-initdb.d`, which Postgres executes only when initializing an
> empty data directory. If you pull changes that add a migration, run
> `docker compose down -v` before `up`, or apply the SQL by hand. There is no
> migration runner yet.

## Tests

```bash
make test          # Go unit tests
make test-race     # Go unit tests under the race detector
make lint          # golangci-lint + ruff
make test-python   # pytest
```

CI runs all four on every pull request. A change is not ready for review if any of
them fail.

Three rules the project actually enforces:

1. **New logic ships with a test.** Table-driven tests are the established Go style
   here — see `internal/drift/detector_test.go` or `internal/gateway/handlers_test.go`.
2. **Bug fixes ship with a regression test** that would have caught the bug.
3. **Anything that touches SQL gets an integration test against a real Postgres**,
   not a mock. Query correctness is exactly what mocks cannot verify. Use the
   Compose services.

Go tests live next to the code they test (`detector.go` / `detector_test.go`).
Python tests live under `worker/tests/`.

## Coding conventions

The full engineering charter is in [`.claude/CLAUDE.md`](.claude/CLAUDE.md). It was
written for AI assistants working in this repo but applies equally to humans, and it
is the standard code review is held to. The parts worth knowing before your first
PR:

**Architecture.** Dependencies point inward. HTTP handlers depend on interfaces, not
on concrete infrastructure, and never import a database driver directly.
`internal/gateway/interfaces.go` is the model. `cmd/*` is wiring only — business
logic lives in `internal/*`.

**Interfaces are defined at the consumer, not the producer.** The gateway declares
what it needs from the orchestrator; the orchestrator package does not know the
gateway exists.

**Errors are values.** Wrap with `fmt.Errorf("doing X: %w", err)` so the chain
survives. Never `panic` in a request path. Never swallow an error without a comment
explaining why it is genuinely safe to ignore.

**Comments explain why, never what.** If a comment restates the code, delete it and
rename the variable instead.

**Every tenant-scoped query filters by `tenant_id`.** Isolation is enforced at the
query layer, not the handler layer. A table without its own tenant column must join
through `jobs` to get one. This has already been the source of one cross-tenant data
leak — see `.claude/memory/known-bugs.md`.

**Every new endpoint is authenticated and rate-limited** unless it is `/health`.

Naming follows `gofmt`/`golint` for Go and PEP 8 for Python. Booleans read as
predicates (`isEnabled`, not `enabled`). Exported Go identifiers get a doc comment
beginning with the identifier's name.

## Commits and pull requests

Commits follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat(agent): add a goal-directed benchmark optimization agent
fix(security): scope GET /v1/metrics to the authenticated tenant
docs: rewrite the README to match the implementation
```

Types in use: `feat`, `fix`, `refactor`, `docs`, `test`, `chore`, `ci`, `perf`.
Release tooling reads these, so the type prefix is not decorative.

Keep commits small enough to revert in isolation. Work on a feature branch — never
commit directly to `master`.

Before opening a PR, self-review against the checklist in the charter's §3: bugs,
performance (N+1 queries, unindexed lookups), readability, security, edge cases
(empty input, max input, concurrent access, partial failure), error handling,
goroutine and connection leaks, and whether the change assumes a single instance.

Fill in the PR template. If your change touches a hot path — gateway request
handling, orchestrator dispatch, drift queries — say what load test would validate
it.

## Database migrations

Migrations are sequentially numbered SQL files in `migrations/`, applied in order.
Follow the existing conventions:

- `TIMESTAMPTZ` for timestamps, never `TIMESTAMP`.
- Indexes on tenant-scoped tables lead with `tenant_id`.
- Hypertables get a TimescaleDB compression policy.
- Every new query pattern gets its matching index in the same migration, not a
  follow-up.

Migrations are forward-only and must be safe to apply to a database that already has
data.

## Reporting bugs and requesting features

Use the issue templates. For bugs, the two things that make a report actionable are
the exact command or request that triggered it and the relevant log lines — logs are
structured and carry `tenant_id` and `request_id`, which is usually enough to trace
a request end to end.

**Do not open a public issue for a security vulnerability.** See
[SECURITY.md](SECURITY.md).
