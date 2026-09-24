---
name: proto-sync
description: Use when adding or changing a gRPC service/message in proto/inferx/v1/, or when generated code in gen/ looks out of sync with the .proto source. Regenerates Go gRPC code via `make proto` and verifies the generated output matches the proto source instead of having been hand-edited. Trigger on: "add a gRPC method", "change the proto", "regenerate proto", "gen/ is out of date", "update inferx.proto".
---

# Proto Sync

The `.proto` files under `proto/inferx/v1/` are the source of truth for the gRPC contract (see `../../context/api-contracts.md`); `gen/` holds generated Go code that must never be hand-edited. This skill is the procedure for keeping them in sync correctly.

## Procedure
1. **Edit the `.proto` file(s) only** under `proto/inferx/v1/` — never touch files under `gen/` directly, even for a "quick fix."
2. **Follow existing message/service naming conventions** already established in the proto package — check the current file for the naming and field-numbering style before adding new messages/RPCs.
3. **Never reuse or renumber an existing field number** in a message — protobuf field numbers are part of the wire format; reusing one that was previously used (even if removed) risks silent data corruption for any client still encoding the old field. Reserve removed field numbers explicitly (`reserved N;`) instead of leaving them open for reuse.
4. **Regenerate** by running `make proto` (per the README's documented Makefile target) — this invokes `protoc` against the `.proto` sources and writes output to `gen/`.
5. **Diff the generated output.** Confirm the change to `gen/` is exactly what's expected from the `.proto` diff — no unrelated formatting churn suggesting a `protoc`/plugin version mismatch with what's normally used in this repo.
6. **Update callers.** Any Go code constructing/consuming the changed message needs updating in the same change — a proto field rename/removal is a breaking change for every caller, not just the generated code.
7. **Update `../../context/api-contracts.md`** to reflect the new/changed RPC contract.
8. **Version consideration.** If this is a breaking change to an already-deployed RPC (removed/renamed field, changed semantics), treat it with the same versioning discipline as REST `/v1/` → `/v2/` (see `../../CLAUDE.md` §2 API design) — additive fields are safe to add directly; breaking changes need a new versioned service/message or an explicit, coordinated rollout plan since gRPC has no URL-path versioning equivalent by default.

## Common Mistakes This Guards Against
- Hand-editing generated code in `gen/` (gets silently overwritten on the next `make proto`, and diverges from what the proto source actually describes).
- Reusing a field number after removing a field, causing wire-format ambiguity for older/newer client-server pairs.
- Forgetting to update Go callers after a message shape change, leading to a compile error caught late or, worse, a runtime type mismatch if using a dynamic/reflection-based path.
