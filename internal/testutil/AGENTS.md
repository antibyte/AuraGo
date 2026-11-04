# Test utilities

## Purpose

Shared fixtures for Go tests; production packages must not depend on these helpers.

## Ownership

This package owns local HTTP/TLS servers, test asset packaging and storage fixtures.

## Local Contracts

- HTTP fixtures bind IPv4 loopback. Unavailable local sockets may skip a fixture;
  validation runs requiring protocol coverage must check for skips.
- TrustHTTPSTestServer trusts only httptest's built-in certificate via the Go
  fallback root pool. Its temporary GODEBUG change requires serial tests. Inject
  RootCAs for custom certificates. Network grants remain explicit in each test.
- Fixtures use synthetic data and never contact live providers or mutate accounts.

## Work Guidance

Use the existing server helpers and register cleanup with the owning test.

## Verification

Run the caller's focused suite with local socket access and verify zero skips for
required protocol fixtures.

## Child DOX Index

None.
