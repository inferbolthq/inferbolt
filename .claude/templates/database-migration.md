# Database Migration Template

```sql
-- migrations/<NNN>_<short_description>.sql
-- Purpose: <one line — what this migration does and why>
-- Rollback: <how to reverse this if needed — additive migrations are usually rolled back by a follow-up migration, not a down-script, in this repo's convention>

CREATE TABLE IF NOT EXISTS public.<table_name> (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id   TEXT NOT NULL,
    -- domain columns here
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS <table_name>_tenant ON public.<table_name> (tenant_id, <dominant_filter_column>);

-- For TimescaleDB hypertables, include hypertable creation + compression policy in the same migration:
-- SELECT create_hypertable('<schema>.<table_name>', 'ts', if_not_exists => TRUE);
-- ALTER TABLE <schema>.<table_name> SET (timescaledb.compress, timescaledb.compress_segmentby = '<dims>');
-- SELECT add_compression_policy('<schema>.<table_name>', INTERVAL '<n> days');
```

## Usage Notes
- Follow existing numbering (`ls migrations/` for the next number) — never renumber or edit a previously-shipped migration.
- Every tenant-scoped table needs a `tenant_id`-leading index for its dominant query pattern (see [../context/database-schema.md](../context/database-schema.md)).
- New hypertables need a compression/retention policy in the same migration, not a follow-up.
- Additive-first: never `DROP COLUMN`/`DROP TABLE` in the same migration that a deploy depends on removing — sequence destructive changes across multiple deploys so old code isn't broken mid-rollout (see [../commands/database.md](../commands/database.md)).
- Get an `EXPLAIN ANALYZE` for any new query this migration is meant to support before considering it done.
