# /api — API Design & Implementation

## Purpose
Design or implement HTTP/gRPC API surface consistent with the existing gateway conventions (`internal/gateway/`, `proto/inferx/v1/`).

## Inputs
- Target: new endpoint/RPC, or a review of an existing one.

## Outputs
- Handler implementation (or proto definition + generated stub usage) following existing patterns.
- Request/response shapes using the shared envelope (`internal/gateway/handler/response.go`).
- Auth, validation, and rate-limit wiring included, not deferred.
- Updated `context/api-contracts.md`.

## Workflow
1. Decide REST vs. gRPC based on existing precedent (external/tenant-facing → REST via gateway; internal service-to-service where proto already models it → gRPC).
2. Define the request/response contract first — fields, types, required/optional, error cases — before writing the handler body.
3. Wire through the existing middleware stack: `Recovery → Logger → IPLimiter → Auth → TenantLimiter` (see README's middleware diagram) — new protected routes must not bypass any layer.
4. Validate all input at the handler boundary; return specific 4xx errors for validation failures, never a generic 500.
5. Use the `OrchestratorClient`-style interface pattern (`internal/gateway/interfaces.go`) when the handler needs to call another service — define the interface at the consumer.
6. Add handler tests covering: happy path, missing/invalid auth, validation failure, rate-limit exceeded, and the relevant tenant-tier behavior.
7. For gRPC: regenerate stubs via `make proto` after editing `.proto` files; never hand-edit generated code in `gen/`.
8. Version breaking changes (`/v2/...`); additive fields are safe on `/v1/`.

## Examples
```
/api add GET /v1/jobs/{id}/recommendations
/api design a gRPC streaming RPC for live benchmark progress
/api review the /v1/jobs POST contract for missing validation
```

## Best practices
- Every new endpoint needs a rate limit tier decision (OSS/free/paid/enterprise) — don't leave it implicitly unlimited.
- Prefer extending an existing resource before inventing a new top-level route.
- Document error responses (4xx/5xx bodies), not just the 200 case — API consumers need both.
