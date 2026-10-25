# Prompt self-tests

## Purpose

Versioned prompt catalog parsing, bounded run coordination, private evidence and reports.

## Ownership

- This package owns run IDs, case state, storage, retention and deterministic aggregation.
- `internal/agent/self_test_*.go` owns normal-runtime execution, exact argument gates and local fixtures. `internal/server/self_test_*.go` owns admin access, lifecycle and reviewed saved-configuration probes. `prompts/self_tests/` owns the prompt content. Dashboard controls live in `ui/js/dashboard/self-tests.js`.

## Local Contracts

- One active run per server. Cancellation and server shutdown drain work before dependencies close. Never replay interrupted cases; only an in-flight case becomes UNKNOWN.
- Use rooted private storage, synced temporary files and `fileutil.RenameRootContext`. IDs are opaque, names are validated and reports never accept a client path.
- Persist only catalog prompts, capability metadata, typed results, assertions, hashes and opaque fixture receipts. Exact human bindings remain in process memory; never persist model prose or raw tool payloads.
- PASS requires actual execution evidence and explicit assertions. Connection probes and typed tool success prove only their stated checks. Count primary schema selector values, not a Cartesian product or untested sub-operations.
- Keep blocked, disabled, unknown and unreviewed units visible. Read profiles never automatically execute unknown custom/MCP operations. Targeted bindings still obey normal capabilities, read-only settings, action policy and live revocation.
- Retain unresolved effects/cleanup receipts. Bound history to 14 days/100 resolved runs and 100 MiB; stop admitting runs when preserved records exhaust the storage budget.

## Work Guidance

- Reuse the normal minimal loop and dispatch. No new dependency, tool bypass or fake success handler.
- Keep the catalog deterministic and prompts complete. Human/laboratory cases require appropriate fixtures; absence is a reported blocker.

## Verification

- `go test ./internal/selftest`
- `go test ./internal/agent ./internal/server -run SelfTest`
- `go test ./ui -run SelfTest` and `node ui/testdata/selftests.cjs`

## Child DOX Index

None.
