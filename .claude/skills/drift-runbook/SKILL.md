---
name: drift-runbook
description: Use when a drift-detection Slack alert fires, when investigating a suspected performance regression for a specific (engine, model) pair, or when asked to triage/explain a drift alert. Walks through this repo's actual drift-detection pipeline (internal/drift/baseline.go, detector.go, notifier.go, public.baselines, metrics.bench_results) to find the real cause rather than treating the alert as self-explanatory. Trigger on: "drift alert", "baseline deviation", "why did drift detector fire", "performance regression for <engine>/<model>", "false positive drift".
---

# Drift Alert Runbook

Companion to `../../knowledge/business-domain.md` (why drift detection matters) and `../../context/deployment.md` (the collector's `DRIFT_*` config). This skill is the concrete triage procedure for an actual fired alert — written for the 3am-on-call case: action-first, no narrative preamble.

## Procedure
1. **Identify the alert's (engine, model) pair and which metric crossed threshold** (TTFT, throughput, GPU memory, etc. — see the columns in `metrics.bench_results` via `../../context/database-schema.md`) from the Slack message (`internal/drift/notifier.go` formats this).
2. **Pull the current baseline** for that (engine, model) from `public.baselines` — note `sample_count` and `set_at`. A baseline set from very few samples or a long time ago is itself a likely false-positive source (regressed relative to a stale or noisy baseline, not because current performance actually degraded).
3. **Pull recent samples** from `metrics.bench_results` for the same (engine, model), over the collector's configured `DRIFT_LOOKBACK` window. Check:
   - Is the deviation sustained across the window, or driven by one or two outlier samples? (`internal/drift/detector.go` should already guard against single-sample noise — confirm it's actually doing so, per `detector_test.go`'s test cases.)
   - Does the deviation correlate with a specific `job_id`/tenant/time-of-day (e.g., GPU contention from concurrent jobs) rather than a genuine engine/model regression?
4. **Check for an external cause** before assuming a real regression: recent deploy of `worker/engines/*.py`, a driver/infra change, a model version bump, or a change to `internal/config` affecting benchmark parameters.
5. **Classify:**
   - **Real regression** — confirmed sustained deviation with no benign external cause. Escalate per severity (warning vs. critical threshold, see `../../context/deployment.md`'s `DRIFT_WARNING_PCT`/`DRIFT_CRITICAL_PCT`) and open a bug/incident as warranted.
   - **Stale/noisy baseline** — the baseline itself needs recomputation (`internal/drift/baseline.go`), not the underlying engine.
   - **False positive** — single-sample noise or an environmental blip; no baseline change needed, but if this recurs, the detector's sensitivity/window may need tuning — log as a `../../memory/technical-debt.md` entry, don't just silence the alert.
6. **Record the outcome** in `../../memory/known-bugs.md` (if a real regression) or note the false-positive pattern if the detector needs tuning, so recurring noisy alerts get addressed structurally instead of individually dismissed each time.

## Common Mistakes This Guards Against
- Treating a stale/low-sample-count baseline as ground truth instead of questioning it first.
- Reacting to a single noisy sample as a confirmed regression.
- Fixing the symptom (silencing the alert) instead of the cause (recomputing a bad baseline, or fixing an actual regression, or tuning genuine detector oversensitivity).
