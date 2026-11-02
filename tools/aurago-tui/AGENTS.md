# AuraGo TUI

## Purpose

Rust terminal client for AuraGo's authenticated HTTP API and chat event stream.

## Ownership

- `src/api/` owns HTTP requests, response decoding, SSE and session persistence.
- `src/main.rs`, `src/actions.rs` and `src/app.rs` own startup, user actions and UI state.
- The server owns authorization; the TUI retains its existing API gates and headers.

## Local Contracts

- Bound connection establishment to 30 seconds. Ordinary API requests use the
  shared request builder's 30-second total timeout; SSE has no total lifetime
  deadline and retains the existing reconnect behavior.
- Persist session cookies in the platform configuration directory. Unix files
  use mode `0600`, including existing files; repair permissions before truncating
  or writing credentials. Windows retains inherited profile/file ACLs.
- Never log or commit session cookies. Keep Windows, Linux and macOS support;
  platform-specific permission APIs stay behind `cfg(unix)`.

## Work Guidance

- Reuse the shared API request methods and `src/api/test_server.rs` fixtures.
- Keep API headers and session cookies consistent between ordinary requests and SSE.
- Keep regression tests beside the affected code and use synthetic credentials.

## Verification

- Run `cargo test --locked` after client changes.
- Transport tests require local loopback TCP and include a real 31-second stream.
- Run the suite on Unix to verify `0600` creation and existing-file repair;
  Windows tests verify persistence but do not execute Unix permission branches.

## Child DOX Index

None.
