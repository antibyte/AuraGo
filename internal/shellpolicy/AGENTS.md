# Shared shell command policy

## Purpose

Classify high-risk literal shell commands before local, isolated or Remote execution.

## Ownership

Owns command-head normalization and program-controlled execution checks; callers
retain execution grants, platform escape rules and OS isolation.

## Local Contracts

- Unwrap literal env/command/busybox/nohup/timeout/nice/stdbuf/time prefixes,
  including nested wrappers and their documented literal options, before classification.
  Under Cmd/PowerShell, native timeout /t, /nobreak and /? remain ordinary
  wait/help commands; the native indefinite wait accepts -1 with optional /nobreak.
  GNU timeout durations still unwrap the executed command.
- Reject unresolved wrapper syntax, executing xargs, editor command modes and
  executable Git configuration/environment. Help and version queries remain reads.
- Preserve quotes, escaped separators, comments and benign literal arguments.
- Reject executable environment writes through POSIX export/declaration builtins
  (including append assignments), Cmd set/setx (including compact arithmetic/input)
  and PowerShell environment assignments or Set-Item/New-Item. Read-only queries
  and unrelated variables remain allowed.
- This heuristic supplements isolation; it cannot sandbox arbitrary programs.
- Executable Git settings include proxies, hooks, fsmonitor, signing programs,
  diff/text conversion, imported configuration and external protocols; environment
  checks include SSH/askpass helpers and GIT_EXEC_PATH. Ordinary config reads stay allowed.

## Work Guidance

Reuse the shared literal-word reader. Never execute destructive test strings.

## Verification

`go test ./internal/shellpolicy ./cmd/remote`; shell-policy tests in `internal/tools`.

## Child DOX Index

None.
