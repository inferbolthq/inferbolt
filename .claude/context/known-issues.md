# Known Issues

> **Status as of 2026-09-10.** The three long-standing entries below - the two
> parallel gateways, the README/implementation drift, and the InferX/InferBolt
> naming split - were all resolved in the `feat/optimization-agent` branch. They are
> kept, struck through, because the reasoning is still useful precedent. New issues
> are listed first.

## `metrics.bench_results` has no tenant column (found + fixed 2026-09-10)
`GET /v1/metrics` returned every tenant's benchmark results, because the table has no
`tenant_id` and the query filtered only on engine/model/timestamp. Fixed by joining
through `public.jobs`. The lesson generalizes: **any table without its own
`tenant_id` must be reached through a join that has one** - that is true of
`recommendations` (already documented) and was equally true of `bench_results`. When
adding a tenant-facing read, check the table's columns, not just the handler. Full
detail in `memory/known-bugs.md`.

## Drift baselines are global, not per-tenant (found 2026-09-10, open)
`public.baselines` is keyed `PRIMARY KEY (engine, model)`. Two tenants benchmarking
the same model share one baseline, so one tenant's results can move - or mask -
another's drift alerts. Needs a product decision before a migration.

## No gRPC server (2026-09-10)
`proto/inferx/v1/` and `gen/` still exist and `make proto` still works, but the server
that used them was deleted with the dead gateway path. Do not assume a gRPC surface
exists because the protos do.

## No pre-auth rate limiting (2026-09-10)
The only limiter runs after authentication. `/health`, `/dashboard` and failed auth
attempts are unbounded. The IP limiter that the old README described lived in the
dead middleware package and was never wired up.

## The planner needs `ANTHROPIC_API_KEY` (2026-09-10)
`inferbolt agent` is the only command that calls a third-party API. Without a
credential it fails on the first turn with an annotated 401. Every other command is
unaffected.


## ~~Two Parallel Gateway Implementations — One Is Dead Code~~ (found 2026-07-05, RESOLVED 2026-09-10 by deleting the dead path)
There are genuinely **two complete, independent implementations** of the gateway's HTTP layer in this repo, and they disagree on the auth model:

1. **`internal/gateway/handlers.go`** + **`internal/auth/{apikey.go,middleware.go}`** — JWT-with-scopes auth (`ScopeJobsWrite`, `ScopeJobsRead`, `ScopeMetricsRead`, `ScopeConfigWrite`, `ScopeAdminAll`), a flat 100-req/min-per-tenant rate limit, no tier concept at all. **This is the one actually wired into `cmd/gateway/main.go`** — confirmed by reading `main.go`'s route table (`gateway.NewHandler(...)`, `iauth.AuthMiddleware`, `iauth.RequireScope`). This is the real, live contract the CLI (`internal/cli/client.go`) and the Python worker actually talk to.
2. **`internal/gateway/router.go`** + **`internal/gateway/handler/`** subpackage + **`internal/auth/tier.go`** — a completely different, tier-based auth model (`oss`/`cloud_free`/`cloud_paid`/`enterprise`, `DefaultLimits`, `internal/gateway/middleware/{auth,ratelimit}.go`'s `KeyLookupFunc`/`TenantLimiter`). **Nothing in `cmd/` references `newRouter` or `handler.JobHandler` — this entire path is dead code**, unreachable from any binary.

**This means:** the tier-based rate-limit table in the README (and, previously, in `context/dependencies.md` — now corrected) describes a system that **does not run**. The real gateway has no tenant tiers and no tier-based rate limiting; every tenant gets the same flat 100 req/min. Do not implement new features against `tier.go`/`router.go`/`handler/` — they're orphaned. Before adding anything to that path, confirm with the user whether it's (a) a genuinely abandoned earlier design that should be deleted, or (b) an in-progress migration that was meant to replace `handlers.go` and got left half-done — the answer changes whether the right move is deletion or completion. Don't silently delete or silently finish it without that confirmation.

## `worker/engines/mock_engine.py` Was Broken/Unregistered (found and fixed 2026-07-05)
`MockEngine` implemented a completely different interface (`run_benchmark(workload)`) than the one `worker/engines/base.py`'s `BaseEngine` ABC actually requires (`name()`, `start()`, `teardown()`, `get_gpu_memory_mb()`, `get_kv_cache_hit_rate()`, `_send_request()`), and imported names (`BenchmarkResult`, `Workload`) that don't exist in `base.py`. It also wasn't registered in `worker/engines/__init__.py`'s `ENGINES` dict, and `"mock"` wasn't in the gateway's `validEngines` allowlist (`internal/gateway/handlers.go`) either — so the no-GPU testing path the README recommends (`ENGINE=mock`) was entirely non-functional through the actual API, on top of referring to the older standalone `worker/benchmark.py` entrypoint rather than the current `worker/main.py` service. **Fixed**: rewrote `mock_engine.py` against the current `BaseEngine` ABC, registered it in `ENGINES`, and added `"mock"` to `validEngines`. `--engines mock --gpu cpu` is now a real, working, no-GPU path — used by `inferbolt run`'s own smoke-testing story.

## Manual Intervention Required End-to-End (mostly resolved 2026-07-05)
Previously, running any benchmark required, in order: `docker compose up -d` (which didn't even include the `gateway` service), manually running the gateway (`go run ./cmd/gateway`), grepping gateway startup logs for a one-time dev API key (unrecoverable after the first restart — `bootstrapDevKey` only fired once and never persisted the plaintext anywhere), and manually starting `worker/main.py` with the right env vars so it could register with the orchestrator before any job could dispatch. **Fixed** via `inferbolt run` (see `memory/completed-features.md` for the full list of changes): a `Dockerfile.gateway` + compose `gateway` service, a `DEV_TOKEN_FILE`-based recoverable dev-key bootstrap, a new `GET /internal/workers` endpoint, and CLI-side logic to bring up `docker compose`, resolve/bootstrap a credential, and spawn a local worker on demand. Remaining manual step: starting Docker Desktop itself (or having Postgres reachable) — `inferbolt run` cannot start the Docker daemon.

## ~~README / Implementation Drift~~ (RESOLVED 2026-09-10 — README rewritten against the source)
The README describes an architecture using **ClickHouse** (metrics) and **Redis** (job queue). The actual implementation uses **TimescaleDB** (a Postgres extension, `metrics.bench_results` hypertable) and **River** (a Postgres-backed durable job queue). This is a legitimate simplification (one datastore instead of three) but the README hasn't been updated to reflect it. Don't "fix" this by silently rewriting the README's architecture section — flag it and confirm with the user, since it may be intentional pending a broader doc pass, and the public-facing README may have external readers who'd be confused by a large silent rewrite.

The README's roadmap checklist also appears stale relative to recent commit history — see [roadmap.md](roadmap.md).

## ~~`JWT_SECRET` Default~~ (RESOLVED 2026-09-10 — `internal/config/gateway.go` deleted; `cmd/gateway` requires the variable and rejects secrets under 32 chars)
`internal/config/gateway.go`'s default for `JWT_SECRET` is the literal string `change-me-in-production`. This is fine for local dev but is a real security risk if a deployment forgets to override it — any deployment/security review should explicitly check this is overridden, not assume it.

## Sparse Test Coverage in Newer Areas
Gateway handlers/middleware and drift detection have solid colocated test coverage. The orchestrator wiring and River job dispatch paths are newer (per recent commit messages about timeouts and dispatch fixes) and should be checked for actual test coverage before being treated as stable — verify with `go test ./... -cover` rather than assuming.

## Empty Scaffolded Directories
`cost/` and `drift/` (top-level, distinct from `internal/drift/`) contain only `.gitkeep` — these are planned but unimplemented. Don't assume functionality exists there because the directory exists. Note `worker/cost/modeler.py` and `worker/search/config_search.py` do have real implementations; the latter has no caller.

## ~~Naming: InferX vs. InferBolt~~ (RESOLVED 2026-09-10 — InferBolt is canonical)
The product is named "InferX" in the README, but the Go module path is `github.com/inferbolthq/inferbolt`, the built binary is `inferbolt`/`inferbolt.exe`, and Dockerfiles are named `Dockerfile.inferbolt` etc. This suggests an in-progress rebrand (InferX → InferBolt) or a naming decision that hasn't been fully propagated. Don't "fix" this inconsistency unprompted — ask which name is canonical before doing a repo-wide rename, since it touches package paths, binary names, Docker image tags, and possibly external references (domain, docs site) outside this repo's visibility.

## How to Add to This File
When you discover a real discrepancy between documented and actual behavior, a footgun in a default, or a fragile area — add it here with enough detail for a future session (or another engineer) to act on it without re-discovering it from scratch.
