# LLM transport

## Purpose

Provider clients, transport policy, retry and failover.

## Ownership

This package owns actual provider target trust and mutable client state.

## Local Contracts

- Parse provider URLs with net/url and reject userinfo. TLS relaxation applies only to the actual loopback HTTPS origin, with pinned loopback dialing and same-origin redirects. Gateway rewrites decide trust from their real endpoint.
- Scrub userinfo/query/fragment before logging provider URLs. Use bounded response-header waits while preserving active long streams and caller cancellation.
- Reconfigure failover thresholds/backoff as one snapshot, applying defaults when new values are unset; do not retain stale previous limits.

## Work Guidance

Use synthetic local HTTP/TLS fixtures for network policy tests.

## Verification

`go test ./internal/llm`, including real self-signed loopback fixtures and failover tests.

## Child DOX Index

None.
