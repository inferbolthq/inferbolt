# /database — Schema & Query Work

## Purpose
Design, review, or optimize database schema and queries against Postgres/TimescaleDB, following the migration conventions in `migrations/`.

## Inputs
- Target: new table/column, a slow query, a migration to write, or a schema review request.

## Outputs
- A migration file (`migrations/NNN_description.sql`) following the existing numbering convention.
- Query changes with `EXPLAIN ANALYZE` reasoning for anything non-trivial.
- Index recommendations tied to actual query patterns, not speculative ones.

## Workflow
1. Read existing migrations (`migrations/001_init.sql` through `005_api_keys.sql`) to match naming, style, and conventions (e.g., `TIMESTAMPTZ NOT NULL DEFAULT NOW()`, explicit index naming like `jobs_tenant_state`).
2. For new tables: define primary key, foreign keys with `REFERENCES`, `NOT NULL` where the domain requires it, and at least one index matching the expected query pattern (usually `tenant_id` + a filter column).
3. For TimescaleDB-backed tables (metrics, drift baselines): confirm whether the table should be a hypertable and whether continuous aggregates are appropriate for the query pattern.
4. For query changes: get the query plan (`EXPLAIN ANALYZE`) before and after; don't add an index speculatively without confirming it's used.
5. Never write a migration that silently drops or alters data destructively — additive migrations by default; destructive changes need explicit confirmation and a rollback plan.
6. Keep tenant isolation in mind for every new table: it must be filterable/indexed by `tenant_id` if it holds tenant-scoped data.
7. Update `context/database-schema.md` after any schema change.

## Examples
```
/database add a table for per-tenant cost reports
/database optimize the drift baseline lookup query
/database review migrations/004_baselines.sql for indexing gaps
```

## Best practices
- Every foreign key gets a matching index unless there's a documented reason not to (small/rarely-joined table).
- Timestamps are `TIMESTAMPTZ`, never naive `TIMESTAMP` — this codebase runs across time zones and services.
- Prefer `JSONB` for genuinely variable-shape data (`workload_config`, `best_config`) and typed columns for anything queried/filtered/indexed on.
- Migrations are forward-only in this repo's convention — write a new migration to correct a mistake, don't edit a shipped one.
