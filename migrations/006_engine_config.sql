-- Engine tuning knobs (quantization, tensor parallelism, batch size) for a job.
-- The Python worker has always accepted an engine_config on BenchmarkJob and
-- passed it to engine.start(); the Go side never sent one, so every benchmark
-- silently ran with the worker's defaults and no config sweep was possible.
--
-- Additive: the DEFAULT keeps existing rows and any still-running older gateway
-- version valid, since neither writes this column.
ALTER TABLE public.jobs
    ADD COLUMN IF NOT EXISTS engine_config JSONB NOT NULL DEFAULT '{}'::jsonb;
