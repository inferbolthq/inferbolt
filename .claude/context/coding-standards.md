# Coding Standards (Project-Specific Addendum)

This file captures conventions observed in the actual codebase, as a quick-reference companion to [../CLAUDE.md](../CLAUDE.md) §2. When in doubt, match existing code over any generic style guide.

## Go
- Package-per-domain under `internal/`: `auth`, `gateway` (with `handler/` and `middleware/` subpackages), `drift`, `jobs`, `queue`, `engine`, `metrics`, `config`, `cli`.
- Middleware pattern: small, single-purpose files in `internal/gateway/middleware/` (`auth.go`, `logging.go`, `otel.go`, `ratelimit.go`, `recovery.go`, `requestid.go`, `timeout.go`) composed in `router.go`. New cross-cutting concerns follow this pattern — one file, one concern, composed at the router.
- Handlers split by resource under `internal/gateway/handler/` (`health.go`, `jobs.go`), with shared `types.go` (request/response structs) and `response.go` (envelope helpers).
- Tier/limit logic centralized in `internal/auth/tier.go` (`Tier` enum, `DefaultLimits` map, `String()`/`TierFromString()` round-trip) — don't duplicate tier-checking logic elsewhere; extend this file.
- Test files colocated (`detector.go`/`detector_test.go`, `handlers.go`/`handlers_test.go`, `middleware.go`/`middleware_test.go`).
- CLI built on `cobra` + `viper` in `internal/cli/` and `cmd/inferbolt/main.go` — new CLI subcommands follow the existing `client.go`/`config.go` split (transport vs. config resolution).

## Python (`worker/`)
- `snake_case` module and function names; Pydantic models in `PascalCase`.
- Engine adapters in `worker/engines/` implement a common protocol so the orchestrator/router can treat `vllm_engine.py`, the mock engine, and future `sglang_engine.py`/`llamacpp_engine.py` interchangeably — don't special-case a specific engine in orchestration code.
- Tests under `worker/tests/` per `pyproject.toml`'s `testpaths`, `asyncio_mode = "auto"`.

## SQL / Migrations
- Sequential numbered files: `NNN_description.sql` (`001_init.sql` … `005_api_keys.sql`). Never edit a shipped migration — add a new one.
- `TIMESTAMPTZ NOT NULL DEFAULT NOW()` for all timestamps.
- Explicit, descriptively-named indexes (`jobs_tenant_state`, `bench_results_job_id`, `api_keys_tenant`) — name new indexes `<table>_<columns>`.
- Tenant-scoped tables always get a `tenant_id`-leading index for the dominant query pattern.
- TimescaleDB hypertables get an explicit compression policy (see `002_timescale.sql`) — don't leave high-volume tables uncompressed/unbounded.

## Configuration
- Env-var driven, parsed once at startup into typed config structs (`internal/config/gateway.go`) — no ad hoc `os.Getenv` in business logic.
- Every new env var gets a documented default and an entry in the README's config table (until that table is migrated into `deployment.md` — see [known-issues.md](known-issues.md)).

## Naming carryovers to preserve
- `tenant_id` (not `tenantID` in JSON/DB contexts, not `account_id`, not `org_id`) — this is the established term across auth, jobs, and audit_log.
- `job_id` (not `jobID` in JSON, not `run_id` — note `jobs.run_id` is a distinct field referencing the *execution* run, don't conflate the two).
