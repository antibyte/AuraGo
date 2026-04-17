# LDAP Integration Remediation Plan

> **Date:** 2026-04-17
> **Source review:** `reports/ldap-integration-review-2026-04-17.md`
> **Related baseline plan:** `plans/ldap-integration-plan.md`
> **Goal:** Close the confirmed LDAP integration gaps without regressing security, agent output formatting, or UI expectations.

---

## 1. Review validation

The review is broadly useful and correctly identifies the main implementation gaps. After checking the report against the current code, the plan should be based on the following interpretation:

- **Confirmed:** duplicate `Tool Output:` prefix, missing `ReadOnly` enforcement in dispatch, missing `ldap_bind_password` Python blacklist entry, missing `enabled` guard in `handleLDAPTest`, missing tests, overly broad list filters, redundant `GetGroup` filter, missing write operations versus planned scope.
- **Adjusted:** the filter-escaping recommendation should not manually extend RFC 4515 with extra operator escapes. Instead, switch to the library helper `ldap.EscapeFilter(...)` or an equivalent RFC-4515-compliant implementation.
- **Adjusted:** `RequestTimeout` is not completely unused today; it is already passed into LDAP search `TimeLimit`. The real gap is missing connection/socket timeout handling and the lack of a clearly enforced request timeout strategy.
- **Accepted with caution:** the `Authenticate` rebind behavior should be changed, but the fix should preserve clear logging and avoid silently masking connection-state problems.

### Recommended scope decision

Keep the existing `readonly` UI/config contract and complete the missing write-path support instead of removing the toggle. The current plan, schema, and UX already imply eventual write support, so removing it now would create churn and more follow-up work.

---

## 2. Execution order

### Phase 1: Immediate safety and output fixes

**Objective:** remove confirmed high-risk issues that can affect agent behavior or secret isolation.

Files:
- `internal/tools/ldap.go`
- `internal/agent/agent_dispatch_services.go`
- `internal/tools/python_secrets.go`
- `internal/server/ldap_handlers.go`

Tasks:
- Remove the `Tool Output:` prefixing from `tools.LDAP()` and return plain JSON strings only.
- Keep the centralized prefixing in dispatch so LDAP matches the existing built-in tool pattern.
- Add `ReadOnly` gating in the LDAP dispatch path for all mutating operations, even before the write operations are implemented.
- Add `ldap_bind_password` to `blockedSecretExact`.
- Reject `/api/ldap/test` requests when `cfg.LDAP.Enabled == false`.
- Tighten configuration validation in the LDAP tool/handler path so `BindDN` failures produce a clear user-facing error.

Acceptance criteria:
- LDAP tool responses appear once as `Tool Output: {...}` in agent logs and model-visible output.
- Python tool secret injection cannot request `ldap_bind_password`.
- The test endpoint and dispatch both honor LDAP enablement and read-only policy.

### Phase 2: Correctness and protocol hardening

**Objective:** make lookup behavior safer and more predictable for real LDAP/AD deployments.

Files:
- `internal/ldap/client.go`
- `internal/tools/ldap.go`

Tasks:
- Replace the custom `escapeFilter()` usage with `ldap.EscapeFilter(...)` to align with the upstream library and RFC 4515 handling.
- Narrow `ListUsers()` and `ListGroups()` filters to user/group object classes instead of `(objectClass=*)`.
- Simplify `GetGroup()` to a non-redundant filter, optionally with `sAMAccountName` support if that is desired for AD compatibility.
- Change `Authenticate()` so a successful user bind remains a successful authentication even if the follow-up service-account rebind fails; log the rebind issue explicitly.
- Implement real connection timeout handling for dial/setup and define how request timeout should be enforced consistently (`TimeLimit`, socket timeout, or both).

Acceptance criteria:
- User/group lookup functions return directory objects of the intended type.
- Filter escaping uses the library-standard path instead of a partial custom implementation.
- Authentication semantics no longer report false negatives after a successful user bind.

### Phase 3: Close the scope gap for write operations

**Objective:** bring the implementation in line with the existing LDAP integration plan and UI expectations.

Files:
- `internal/tools/ldap.go`
- `internal/ldap/client.go`
- `internal/agent/native_tools_integrations.go`
- `internal/agent/tool_args_ldap.go`
- `internal/agent/agent_dispatch_services.go`
- `prompts/tools_manuals/ldap.md`

Tasks:
- Implement `add_user`, `update_user`, `delete_user`, `add_group`, `update_group`, and `delete_group`.
- Extend the client layer with add/modify/delete helpers if they are not already present.
- Make schema, argument decoding, dispatch, and tool manual behavior consistent with the supported operations.
- Ensure all write operations are blocked when `ldap.readonly: true`.
- Decide and document the minimal supported attribute model for initial writes so the first version is deterministic.

Acceptance criteria:
- Every operation listed in the LDAP tool schema is actually implemented or explicitly removed from the schema.
- Read-only mode blocks every mutation path consistently.
- Tool manual examples match the real supported arguments and behavior.

### Phase 4: Test coverage and regression safety

**Objective:** lock in behavior before the feature expands further.

Files:
- `internal/ldap/client_test.go`
- `internal/tools/ldap_test.go`
- `internal/server/ldap_handlers_test.go`
- `internal/agent/dispatch_ldap_test.go`

Tasks:
- Add client tests for search filters, object-type list filters, and authentication behavior.
- Add tool tests that validate raw JSON output shape and scrubbed error handling.
- Add dispatch tests for `enabled`, `readonly`, unknown operation handling, and output prefix behavior.
- Add HTTP handler tests for disabled LDAP, missing host/bind config, vault password lookup, and connection-test response formatting.

Acceptance criteria:
- The confirmed review findings are covered by automated tests.
- LDAP regressions in output shape, policy enforcement, and handler behavior are caught by package tests.

---

## 3. Suggested delivery slices

1. **Slice A: hardening patch**
   Deliver Phase 1 plus the Phase 2 escaping/filter fixes.
2. **Slice B: behavior patch**
   Deliver authentication timeout cleanup and remaining protocol corrections.
3. **Slice C: write-path patch**
   Deliver the mutating operations behind the existing `readonly` policy.
4. **Slice D: coverage patch**
   Finish test coverage and align docs/manuals.

This split keeps the highest-risk security and output issues shippable first, without waiting for the full write-operation feature set.

---

## 4. Verification checklist

- `go test ./internal/ldap ./internal/tools ./internal/server ./internal/agent`
- Manual UI check for the LDAP config page and `/api/ldap/test`
- Manual agent smoke test for:
  - `search`
  - `get_user`
  - `get_group`
  - `list_users`
  - `list_groups`
  - `authenticate`
- Manual policy test with `ldap.enabled: false`
- Manual policy test with `ldap.readonly: true`

---

## 5. Implementation notes

- Do not add more custom filter escaping logic unless the upstream helper is proven insufficient.
- Keep all success and error payloads scrubbed consistently when secrets could appear.
- Prefer moving LDAP dispatch into a dedicated helper file once the write-path logic grows further; `agent_dispatch_services.go` is already large.
