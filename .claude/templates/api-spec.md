# API Spec Template

```markdown
## `<METHOD> <path>` (or `<rpc Name>` for gRPC)

**Auth required:** yes/no — <API key / JWT / both>
**Rate limit tier:** <which limiter(s) apply — IP, tenant API, tenant job>

### Request
```json
{
  "field": "type — required/optional — description"
}
```

### Response (2xx)
```json
{
  "field": "type — description"
}
```

### Error Responses
| Status | Condition | Body |
|---|---|---|
| 400 | <validation failure case> | |
| 401 | missing/invalid auth | |
| 403 | <authz failure, if applicable> | |
| 404 | <resource not found — must be tenant-scoped, never leak existence of another tenant's resource> | |
| 429 | rate limit exceeded | |
| 500 | unexpected error (no internal detail leaked) | |

### Idempotency
<Is this endpoint safe to retry? If it creates a resource, is there an idempotency key mechanism?>

### Notes
<Anything non-obvious: pagination behavior, versioning considerations, backward-compatibility constraints>
```

## Usage Notes
- Model after existing endpoints in [../context/api-contracts.md](../context/api-contracts.md) — use the same envelope (`internal/gateway/handler/response.go`) and error-shape conventions.
- Every new endpoint needs an explicit rate-limit-tier decision — "unlimited" is a decision too, but state it rather than leaving it implicit.
