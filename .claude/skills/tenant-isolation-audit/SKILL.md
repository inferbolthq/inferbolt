---
name: tenant-isolation-audit
description: Use when reviewing or writing any query/handler that touches tenant-scoped data (jobs, recommendations, audit_log, api_keys, baselines) in this repo, or before merging a PR that adds a new query pattern. Systematically checks for IDOR — the #1 risk class in this multi-tenant codebase — by verifying every tenant-scoped query filters by tenant_id at the query layer, not just the handler layer. Trigger on: "add an endpoint that fetches by ID", "new query against jobs/recommendations/audit_log/api_keys", "can a tenant access another tenant's data", "IDOR", "tenant isolation review".
---

# Tenant Isolation Audit

This repo's primary security risk is IDOR: an authenticated tenant (even free-tier) referencing another tenant's resource by ID and succeeding, because isolation is enforced logically (a `tenant_id` column + application-level filtering) rather than physically (no per-tenant schemas, no Postgres RLS). See `../../knowledge/security-notes.md` and `../../knowledge/database-notes.md`.

## When to run this
- Any new or changed query against `jobs`, `recommendations`, `audit_log`, `api_keys`, or `baselines`.
- Any new handler that accepts a resource ID from the request path/body (`job_id`, API key `id`, etc.).
- As a standing part of `/security` and `/review` when the diff touches `internal/gateway/handler/` or any `internal/*` package issuing SQL.

## Procedure
1. **Locate every SQL query** in the diff (or the target file/package) that reads or writes a tenant-scoped table.
2. **For each query, verify:**
   - The `WHERE` clause includes `tenant_id = $N` (or joins through a table that does), not just the resource's primary key.
   - The `tenant_id` value comes from the authenticated request context (`internal/auth/context.go`'s accessor), never from a request body/query-param field the caller could set arbitrarily.
   - If the query is a join, confirm every joined tenant-scoped table is also filtered — a join that widens to another tenant's rows via an unfiltered intermediate table is still an IDOR.
3. **For each new handler accepting an ID param:**
   - Confirm the handler resolves the ID *and* the caller's tenant together (e.g., `SELECT ... WHERE id = $1 AND tenant_id = $2`), returning 404 (not 403 — don't leak existence of another tenant's resource) when the ID exists but belongs to a different tenant.
4. **Check `recommendations`** specifically: it references `job_id` without its own `tenant_id` column (per `001_init.sql`) — any query fetching a recommendation by `job_id` alone must join through `jobs` and filter on `jobs.tenant_id`, since `recommendations` has no direct tenant column to filter on itself.
5. **Report findings** the same way `/security` does: concrete exploit scenario ("tenant A can pass tenant B's `job_id` to `GET /v1/jobs/{id}/recommendations` and read it"), not just "missing tenant_id."
6. If a gap is found and not immediately fixed, log it in `../../memory/known-bugs.md` (if already shippable/shipped) or block the PR — don't let it pass silently.

## Common Mistakes This Catches
- A query written as `SELECT * FROM jobs WHERE id = $1` (found via convenience/copy-paste from a non-tenant-scoped context) instead of `WHERE id = $1 AND tenant_id = $2`.
- A handler that resolves `tenant_id` from context but then queries a joined table that isn't itself tenant-filtered (the `recommendations` table gap above).
- Returning 403 instead of 404 for cross-tenant access attempts, which confirms the resource exists to an attacker probing IDs.
