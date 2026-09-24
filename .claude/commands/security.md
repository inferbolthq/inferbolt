# /security — Security Review

## Purpose
Run a focused security pass over a diff, endpoint, or subsystem against [../CLAUDE.md](../CLAUDE.md) §5 and the OWASP Top 10. Distinct from `/review`, which covers security as one of many dimensions — this command goes deep on security alone.

## Inputs
- Target: diff, endpoint, or subsystem (e.g., "the auth middleware", "the new API key rotation flow").

## Outputs
- Findings categorized by OWASP category where applicable.
- Severity (Critical / High / Medium / Low) with concrete exploit scenario for each finding — not just "this could be a risk."
- Concrete remediation, not just "add validation."

## Workflow
1. Map the attack surface: what's the entry point (HTTP route, gRPC method, CLI flag), what's the trust boundary (unauthenticated → authenticated → tenant-scoped)?
2. Check authentication: can this route be reached without valid credentials? Check both API-key and JWT paths (`internal/auth/`).
3. Check authorization: does every query/action verify `tenant_id` ownership, not just "is authenticated"? Look for IDOR — can tenant A reference tenant B's `job_id`?
4. Check injection: any string-built SQL, shell command, or template render with user input.
5. Check secrets handling: anything hardcoded, logged, or returned in an error message that shouldn't be.
6. Check rate limiting: does this route sit behind the existing limiter stack, or does it bypass it?
7. Check dependency risk: any newly added library with known CVEs or that's unmaintained.
8. Check input validation and output encoding for every field crossing a trust boundary.
9. Summarize with a clear verdict: safe to ship / needs fixes before ship / needs a design change.

## Examples
```
/security review the API key rotation endpoint
/security audit tenant isolation across internal/gateway/handler/jobs.go
/security check for secrets in docker-compose.yml and Dockerfiles
```

## Best practices
- Think like an attacker with the same access as a legitimate free-tier tenant — that's the realistic threat model for a multi-tenant SaaS, not just "random internet attacker."
- Never mark something "safe" because "it's internal only" — internal today is exposed tomorrow; auth and validation don't get to be optional based on current deployment topology.
- A finding without a concrete exploit path is a hypothesis — verify it before reporting it as a real vulnerability.
