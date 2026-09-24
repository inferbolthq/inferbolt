# Workflow: Security Review

## Inputs
- A diff, endpoint, or subsystem to review for security — see [../commands/security.md](../commands/security.md).

## Outputs
- OWASP-categorized findings with concrete exploit scenarios and remediation.

## Step-by-Step Process
1. **Map the attack surface** — entry point (route/RPC/CLI flag), trust boundary crossed (unauthenticated → authenticated → tenant-scoped → admin).
2. **Check authentication** on every protected route — both API-key and JWT paths.
3. **Check authorization/tenant isolation** — specifically test for IDOR: can tenant A reference tenant B's resource ID and succeed?
4. **Check for injection** — string-built SQL, shell commands, or template rendering with user input.
5. **Check secrets handling** — hardcoded, logged, or leaked-in-error-message secrets.
6. **Check rate limiting** — does this route sit behind the existing limiter stack?
7. **Check dependency risk** — new libraries with known CVEs or unmaintained status.
8. **Check input validation/output encoding** at every trust-boundary crossing.
9. **Summarize with a clear verdict**: safe to ship / needs fixes / needs a design change.

## Review Checklist
(Full OWASP Top 10 mapping — see [../knowledge/security-notes.md](../knowledge/security-notes.md) for this project's specific mapping.)
- [ ] AuthN enforced
- [ ] AuthZ/tenant isolation enforced at the query layer
- [ ] No injection vectors
- [ ] No secret leakage (logs, errors, commits)
- [ ] Rate limiting present
- [ ] Dependencies checked for known vulnerabilities
- [ ] Input validated, output encoded at every boundary

## Success Criteria
Every finding has a concrete exploit scenario (not just a hypothesis) and a specific remediation; nothing is marked "safe" based on current deployment topology alone (e.g., "internal only" is not a security control).
