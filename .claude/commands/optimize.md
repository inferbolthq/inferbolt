# /optimize — General Optimization Pass

## Purpose
A broader sibling to `/performance`: optimize for a named dimension that isn't purely latency/throughput — e.g., cost, memory footprint, binary size, developer velocity (build time, test time).

## Inputs
- Target and dimension to optimize (cost, memory, build time, cold-start time, etc.).

## Outputs
- Baseline measurement for the chosen dimension.
- A ranked list of optimizations by impact-to-effort ratio.
- Applied changes with before/after numbers where measurable.

## Workflow
1. Name the dimension explicitly — "optimize this" is not actionable, "reduce worker cold-start time" or "reduce Postgres row-storage cost for metrics" is.
2. Measure the current state (build time via `make build`, image size via `docker images`, TimescaleDB table size, etc.).
3. Generate candidate optimizations and rank by impact/effort — e.g., for cost: retention policies on TimescaleDB hypertables before considering a different storage engine.
4. Apply the highest-ratio change first; re-measure; decide whether to continue.
5. Document the trade-off if the optimization costs something elsewhere (e.g., a shorter retention window loses historical drift data).

## Examples
```
/optimize reduce Dockerfile.inferbolt image size
/optimize reduce CI build time in .github/workflows/ci.yaml
/optimize reduce TimescaleDB storage cost for benchmark metrics
```

## Best practices
- Always state which dimension is being optimized — optimizing for one dimension (image size) can regress another (build time via extra compression steps).
- Prefer configuration/retention changes over architectural rewrites when they close most of the gap.
- Don't optimize a dimension nobody asked about at the expense of one that matters more right now.
