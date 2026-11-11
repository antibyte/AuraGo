# Shared shell command policy

## Purpose

Classify high-risk literal shell commands before local, isolated or Remote execution.

## Ownership

Owns command-head normalization and program-controlled execution checks; callers
retain execution grants, platform escape rules and OS isolation.

## Local Contracts

- Unwrap literal env/command/busybox/nohup/timeout prefixes before classification.
- Reject unresolved wrapper syntax, executing xargs, editor command modes and
  executable Git configuration/environment. Help and version queries remain reads.
- Preserve quotes, escaped separators, comments and benign literal arguments.
- This heuristic supplements isolation; it cannot sandbox arbitrary programs.

## Work Guidance

Reuse the shared literal-word reader. Never execute destructive test strings.

## Verification

`go test ./internal/shellpolicy ./cmd/remote`; shell-policy tests in `internal/tools`.

## Child DOX Index

None.
