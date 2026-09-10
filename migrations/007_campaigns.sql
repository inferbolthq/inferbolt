-- Optimization campaigns: a goal-directed sequence of benchmarks planned by the
-- agent. Campaigns used to live in the CLI process and die with it; these tables
-- give them a durable home so they survive a disconnect, can be watched from a
-- browser, and can be read back after the fact.
--
-- Not a hypertable: campaigns are low-volume records with a lifecycle, not
-- time-series measurements. The measurements they produce still land in
-- metrics.bench_results like any other benchmark.
CREATE TABLE IF NOT EXISTS public.campaigns (
    id             TEXT PRIMARY KEY,
    tenant_id      TEXT NOT NULL,
    goal           TEXT NOT NULL,
    model          TEXT NOT NULL,
    gpu_profile    TEXT NOT NULL,
    engines        TEXT[] NOT NULL,
    workload       JSONB NOT NULL,
    budget         JSONB NOT NULL,
    planner_model  TEXT NOT NULL,
    state          TEXT NOT NULL DEFAULT 'pending',
    recommendation JSONB,
    trials         JSONB,
    usage          JSONB,
    error_msg      TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at     TIMESTAMPTZ,
    completed_at   TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS campaigns_tenant_state   ON public.campaigns (tenant_id, state);
CREATE INDEX IF NOT EXISTS campaigns_tenant_created ON public.campaigns (tenant_id, created_at DESC);

-- One row per step the campaign took. Clients poll with ?after_seq=N and replay
-- from any point, which is what makes a live view survive a reconnect.
CREATE TABLE IF NOT EXISTS public.campaign_events (
    campaign_id TEXT NOT NULL REFERENCES public.campaigns(id) ON DELETE CASCADE,
    seq         INT NOT NULL,
    kind        TEXT NOT NULL,
    text        TEXT,
    trial       JSONB,
    result      JSONB,
    elapsed_ms  BIGINT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (campaign_id, seq)
);
