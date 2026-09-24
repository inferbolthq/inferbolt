# /deploy — Deployment & Release

## Purpose
Prepare, review, or troubleshoot a deployment: Docker images, Helm chart, k8s manifests, CI/CD pipeline changes.

## Inputs
- Target: a service to deploy/update, a Helm values change, a CI pipeline change, or a deployment failure to diagnose.

## Outputs
- Updated Dockerfile/Helm/CI configuration.
- A rollout plan noting health-check behavior, rollback path, and any required migrations (which must run before the new version expects the new schema).
- Verification steps (health check URLs, expected log lines/metrics post-deploy).

## Workflow
1. Confirm which environment this targets (local `docker-compose`, k8s via Helm) and what's actually changing (image, config, replica count, resource limits).
2. For schema changes: confirm migration ordering — migrations must be backward-compatible with the currently-running version during a rolling deploy (additive columns before code that requires them, not simultaneous).
3. For new services: ensure a `/health` endpoint and matching healthcheck (Dockerfile/compose/k8s probe) exist before wiring dependents on it.
4. For Helm/k8s changes: check resource requests/limits are set (no unbounded pods), and that secrets are sourced from a secret store/K8s Secret, never inlined in values files.
5. For CI changes (`.github/workflows/ci.yaml`, `release.yaml`, `.goreleaser.yaml`): confirm build+test still gates merge, and that release tagging follows semantic versioning (§7).
6. State the rollback plan explicitly: what does reverting this deploy require (previous image tag, migration down path if truly necessary)?

## Examples
```
/deploy add a k8s liveness/readiness probe for the operator
/deploy update Dockerfile.collector to a smaller base image
/deploy diagnose why the router pod fails its healthcheck after the last release
```

## Best practices
- Never treat a migration and the code that depends on it as atomic in a rolling deploy — sequence them so old code tolerates the new schema mid-rollout.
- Resource limits and health checks are not optional additions "for later" — every new deployable needs them from the first version.
- Secrets never enter a Dockerfile `ENV`/`ARG` at build time — only at runtime.
