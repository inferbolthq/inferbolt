---
name: migration-scaffold
description: Use when adding a new database table, column, or index to this repo. Scaffolds a correctly-numbered migration file under migrations/ following this repo's established conventions (sequential numbering, TIMESTAMPTZ timestamps, tenant_id-leading indexes, TimescaleDB compression policies for hypertables) and checks it for the common mistakes this codebase's schema style guards against. Trigger on: "add a migration", "new table", "add a column", "create an index", "new hypertable".
---

# Migration Scaffold

Companion to `../../commands/database.md` and `../../templates/database-migration.md` — this skill is the concrete, step-by-step procedure for producing a migration file that matches this repo's exact conventions, verified against the actual migration history (`migrations/001_init.sql` through `005_api_keys.sql`).

## Procedure
1. **Determine the next migration number.** List `migrations/*.sql`, take the highest `NNN` prefix, increment by one, zero-padded to 3 digits. Never reuse or renumber an existing file.
2. **Name the file** `<NNN>_<short_snake_case_description>.sql`, matching the style of `004_baselines.sql`, `005_api_keys.sql`.
3. **Decide table type:**
   - Durable/transactional data (referenced by application logic, needs joins/FKs) → plain Postgres table in `public.` (or a dedicated schema if it's metrics-adjacent, matching the `metrics.` schema precedent).
   - High-volume time-series data → TimescaleDB hypertable (see step 6).
4. **Write the table definition** following the established shape:
   - Primary key: `UUID PRIMARY KEY DEFAULT uuid_generate_v4()` for most tables, or a composite natural key when one genuinely identifies the row (see `baselines`' `PRIMARY KEY (engine, model)`).
   - All timestamps: `TIMESTAMPTZ NOT NULL DEFAULT NOW()` (or nullable only when null carries meaning, like `revoked_at`).
   - Variable-shape data: `JSONB`. Anything filtered/sorted/joined on: a typed column.
   - Foreign keys: `REFERENCES <table>(<col>)` explicitly, matching `recommendations.job_id → jobs.id`.
5. **Add indexes.** Every tenant-scoped table gets a `tenant_id`-leading index matching its dominant query pattern, named `<table>_<columns>` (e.g., `jobs_tenant_state`). Every foreign key gets a supporting index unless there's a stated reason not to (small/rarely-joined table).
6. **If this is a hypertable:** in the same migration, add `SELECT create_hypertable(...)`, a compression policy (`ALTER TABLE ... SET (timescaledb.compress, ...)` + `add_compression_policy`), matching `002_timescale.sql`'s pattern — don't defer this to a follow-up migration.
7. **Confirm additive-safety.** This migration must not break the currently-deployed code version — no dropping a column/table another running version still reads. If a destructive change is genuinely required, sequence it as a separate, later migration after the dependent code has shipped, and flag this explicitly to the user rather than proceeding silently.
8. **Update `../../context/database-schema.md`** with the new table/column summary in the same change.
9. **Verify** with `EXPLAIN ANALYZE` against the query pattern the new index is meant to support, once the migration has been applied locally (`docker compose up -d` mounts `migrations/` at `/docker-entrypoint-initdb.d`).

## Common Mistakes This Guards Against
- Forgetting the `tenant_id`-leading index on a new tenant-scoped table (creates a future full-scan hot path).
- Using naive `TIMESTAMP` instead of `TIMESTAMPTZ`.
- Creating a hypertable without a compression/retention policy, letting it grow unbounded.
- Editing a previously-shipped migration file instead of adding a new one.
