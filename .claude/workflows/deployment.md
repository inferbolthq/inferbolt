# Workflow: Deployment

## Inputs
- A change ready to ship: new service, config change, Docker/Helm update, or a scheduled release.

## Outputs
- Deployed change with verified health, and a stated rollback path.

## Step-by-Step Process
1. **Confirm target environment** (local `docker-compose`, k8s via Helm) and exactly what's changing.
2. **Sequence schema changes correctly** — additive migrations land before code that depends on them; nothing currently-running old code needs gets removed in the same deploy.
3. **Confirm health checks and resource limits** exist for every deployable before it ships.
4. **Confirm secrets are runtime-injected**, never baked into an image or committed to a values file.
5. **Run CI** (build + test gate) — never bypass it.
6. **Deploy**, then verify: hit `/health`, check expected OTel traces/metrics, check expected log lines.
7. **State the rollback path explicitly** before considering the deploy complete — previous image tag, and migration-down plan only if truly necessary (prefer forward-fix migrations).

## Review Checklist
- [ ] Migration sequencing safe for a rolling deploy
- [ ] Health checks + resource limits present
- [ ] No secrets in images/values files
- [ ] CI gate passed, not bypassed
- [ ] Rollback path stated

## Success Criteria
Deploy completes with passing health checks, observable telemetry, and a known, tested way back if something's wrong.
