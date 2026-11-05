# Test utilities

## Purpose

Shared fixtures for Go tests; production packages must not depend on these helpers.

## Ownership

This package owns local HTTP/TLS servers, test asset packaging and storage fixtures.

## Local Contracts

- HTTP fixtures bind IPv4 loopback. Unavailable local sockets may skip a fixture;
  validation runs requiring protocol coverage must check for skips.
- Default-client TLS fixtures start with RunTLSFixtureSubprocess and return when
  it reports true. Only that child may call TrustHTTPSTestServer; its irreversible
  fallback-root change must never affect the parent test process. The parent
  joins the child and reports its output/errors within the test deadline.
- Child trust covers httptest's built-in certificate only. Inject RootCAs for
  custom certificates. Network grants remain explicit in each test.
- Fixtures use synthetic data and never contact live providers or mutate accounts.

## Work Guidance

Use the existing server helpers and register cleanup with the owning test.

## Verification

Run the caller's focused suite with local socket access and verify zero skips for
required protocol fixtures. TestTLSFixtureTrustStaysInSubprocess verifies that
fixture trust works in the child and remains absent in the parent afterward.

## Child DOX Index

None.
