# LLM transport

## Purpose

Provider clients, transport policy, retry and failover.

## Ownership

This package owns actual provider target trust and mutable client state.

## Local Contracts

- Parse provider URLs with net/url and reject userinfo. TLS relaxation applies only to the actual loopback HTTPS origin, with pinned loopback dialing and same-origin redirects. Gateway rewrites decide trust from their real endpoint.
- Explicit custom provider endpoints survive AI Gateway configuration, including Workers AI. Rewrite only canonical HTTPS provider origins and paths; diagnostics use the same decision and report custom_endpoint with a redacted endpoint.
- Scrub userinfo/query/fragment before logging provider URLs. Use bounded response-header waits while preserving active long streams and caller cancellation.
- Reconfigure failover thresholds/backoff as one snapshot, applying defaults when new values are unset; do not retain stale previous limits.
- Retry interval reloads with no valid configured intervals restore the built-in 30s/2m schedule; an unset per-attempt timeout restores 120s (the configured timeout floor remains 120s).
- `CompletionStream` health is based on terminal reads: success requires finish markers for all requested choices, read errors and EOF before finish count once, and cancellation or an early caller close stays neutral. Once a stream is returned, never replay its content. Task-route streams do not mutate global failover health.
- Context limits prefer exact overrides/registry data, then provider model metadata, then the legacy family-prefix estimate. Ollama `/api/show` `model_info.*.context_length` is metadata and does not establish an active `num_ctx` runtime limit. Keep heuristic metadata labeled `legacy_prefix`; managed-local caps and the global cap remain in force.

## Work Guidance

Use synthetic local HTTP/TLS fixtures for network policy tests.

## Verification

`go test ./internal/llm`, including real self-signed loopback fixtures and failover tests.

## Child DOX Index

None.
