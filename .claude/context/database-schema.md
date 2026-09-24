# Database Schema

Source of truth: `migrations/*.sql`. This file is a navigable summary — always check the actual migration files before relying on this for exact column types.

## `public.jobs` (001_init.sql)
Primary job record. `id TEXT PRIMARY KEY`, `tenant_id`, `model`, `engines TEXT[]`, `workload_config JSONB`, `engine_config JSONB` (added 006, `DEFAULT '{}'` — engine tuning knobs: quantization, tensor_parallel, max_batch_size, max_model_len, gpu_memory_utilization; an absent key means "use the worker's default", so zero values are omitted rather than written), `gpu_profile`, `state` (`pending|running|done|failed`-style lifecycle, see `internal/jobs/state.go` for the authoritative transitions), `run_id`, `error_msg`, `created_at`/`updated_at`.
Indexes: `jobs_tenant_state (tenant_id, state)`, `jobs_updated_at (updated_at DESC)`.

## `public.recommendations` (001_init.sql)
One row per job's best-found configuration: `id UUID`, `job_id → jobs.id`, `best_engine`, `best_config JSONB`, `cost_per_mtok`, `tok_per_sec`, `reasoning`, `created_at`.

## `public.audit_log` (001_init.sql)
Append-only tenant action log: `id UUID`, `tenant_id`, `action`, `resource_id`, `created_at`. Index: `audit_log_tenant (tenant_id, created_at DESC)`.

## `metrics.bench_results` (002_timescale.sql) — TimescaleDB hypertable
Time-series benchmark results: `ts` (hypertable time column), `job_id`, `engine`, `model`, `ttft_p50_ms`, `ttft_p99_ms`, `itl_ms` (inter-token latency), `tok_per_s`, `gpu_mem_mb`, `kv_cache_hit`, `error_rate`, `cost_per_mtok`, `config JSONB`.
Indexes: `bench_results_job_id (job_id, ts DESC)`, `bench_results_engine_model (engine, model, ts DESC)`.
Compression: segment-by `engine,model`, compression policy after 7 days — query patterns on data older than 7 days should account for compressed-chunk query cost.

## River queue tables (003_river.sql)
Created programmatically via `rivermigrate` in `internal/queue/client.go` — not hand-written SQL. Don't add manual migrations for River's internal tables; let River manage its own schema versioning.

## `public.baselines` (004_baselines.sql)
Per-(engine, model) performance baseline for drift detection: `engine`, `model` (composite `PRIMARY KEY`), `ttft_p50_ms`, `ttft_p99_ms`, `tok_per_sec`, `gpu_mem_mb`, `sample_count`, `set_at`. Index: `baselines_set_at (set_at DESC)`.
Consumed by `internal/drift/baseline.go` and `internal/drift/detector.go`.

## `public.api_keys` (005_api_keys.sql)
`id UUID`, `tenant_id`, `key_hash TEXT UNIQUE` (never store raw keys), `scopes TEXT[]`, `expires_at`, `created_at`, `revoked_at` (nullable — null means active). Indexes: `api_keys_tenant`, `api_keys_hash`.

## Schema Conventions to Follow for New Tables
- `TIMESTAMPTZ NOT NULL DEFAULT NOW()` for all timestamps; `revoked_at`/`expires_at`-style nullable timestamps only when null has a specific meaning (not yet revoked/expired).
- Tenant-scoped tables lead their dominant index with `tenant_id`.
- `JSONB` for genuinely variable-shape data (`workload_config`, `best_config`, hypertable `config`); typed columns for anything filtered, sorted, or joined on.
- New hypertables need an explicit compression/retention policy in the same migration that creates them — don't defer this to a follow-up.
