# LLM transport

## Purpose

Provider clients, transport policy, retry and failover.

## Ownership

This package owns actual provider target trust and mutable client state.

## Local Contracts

- Parse provider URLs with net/url and reject userinfo. TLS relaxation applies only to the actual loopback HTTPS origin, with pinned loopback dialing. Gateway rewrites decide trust from their real endpoint.
- Every credentialed client in this package (chat, loopback HTTPS, `NewClientWithTransport`, probes via `newProbeHTTPClient`) uses `httporigin.SameOriginRedirect`; never use `http.DefaultClient` or bare `http.Client` literals here; a rejected redirect is non-retryable.
- Explicit custom provider endpoints survive AI Gateway configuration, including Workers AI. Rewrite only canonical HTTPS provider origins and paths; diagnostics use the same decision and report custom_endpoint with a redacted endpoint.
- Keep the logical provider endpoint for usage/pricing and OpenAI prompt-cache policy separate from the actual gateway origin used for TLS, redirects and authentication. Anthropic gateway paths end in `/anthropic/v1/messages`; the earlier `/v1/` gateway prefix is not the provider API version.
- Per-run provider switches use the complete `TaskProviderConfig` projection, including account/gateway identity and model metadata, before applying explicit model overrides. Cached helper clients include `TaskConfigFingerprint`; config reload compares the complete gateway configuration, including its token and custom-provider slug.
- Explicit `cloudflare-rest` uses its own provider account/gateway/token and canonical model ID, preserves OpenAI chat format, and applies shared privacy/retry policy even with native rewriting disabled. Native and REST clients never exchange credentials. Gateway retries are always one; metadata-only logging must not force Cloudflare logging on.
- Recovery metadata probes use the effective runtime endpoint and its gateway headers, never a direct-provider fallback. Gateway response-start limits may be shorter than generic client floors; active streams and caller cancellation retain their existing ownership.
- Scrub userinfo/query/fragment before logging provider URLs (`redactProviderURL`; log transport and request errors through `redactProviderError`, because `*url.Error` repeats the URL with username and query); read at most 4 KiB of a 429 body into `RateLimitError`. This package must not import `internal/security` (cycle); `internal/server/llm_secrets.go` registers the LLM keys for log scrubbing. Use bounded response-header waits while preserving active long streams and caller cancellation.
- Reconfigure failover thresholds/backoff as one snapshot, applying defaults when new values are unset; do not retain stale previous limits.
- Each manager owns an immutable retry-policy snapshot; capture it once per retry operation and freeze it in task routes. Standalone clients use a consistent default snapshot. `circuit_breaker.llm_timeout_seconds` defaults to 600 seconds total; shorter caller deadlines win. Streaming opening budgets stop on acceptance, preserving the separate three-minute inactivity limit. Non-streaming transport deadlines use the configured attempt limit, not a fixed three-minute cap.
- Recognize typed transport timeouts before general context errors. Owned attempt/transport deadlines count toward retry/failover; caller cancellation is neutral. Classify the original error, redact URL userinfo/query/fragment only in diagnostics, and honor Retry-After with cancellable jitter and a maximum of ten attempts.
- Runtime health carries the actual physical route's provider ID when known; never attribute a failed main attempt to a fallback selected afterward. Passive health reads trigger no probes; reload updates warning thresholds.
- Explicit provider IDs use only their own `RuntimeAPIKey`, including OAuth tokens, never a parent's credentials or inactive static key. Invalid references/masks are blocked before transport; custom transports cannot bypass that block.
- Preserve the invalid-credential signal when projecting task routes: an empty key may be a valid local provider, but a masked key, client secret or OAuth token must fail locally. Caller cancellation wins over a concurrent timeout or late response before any health update; close unaccepted streams.
- Exact provider/model knowledge precedes stable sorted model-only fallbacks. Label those `model_only_fallback` and foreign provider pricing as estimated; retain source/estimate metadata through saved prices.
- Retry interval reloads with no valid configured intervals restore the built-in 30s/2m schedule; an unset per-attempt timeout restores 120s (the configured timeout floor remains 120s).
- `CompletionStream` health is based on terminal reads: success requires finish markers for all requested choices, read errors and EOF before finish count once, and cancellation or an early caller close stays neutral. Once a stream is returned, never replay its content. Task-route streams do not mutate global failover health.
- Context limits prefer exact overrides/registry data, then provider model metadata, then the legacy family-prefix estimate. Ollama `/api/show` `model_info.*.context_length` is metadata and does not establish an active `num_ctx` runtime limit. When that show payload also reports a positive `num_ctx`, the probe uses the smaller of the two; `context_length` alone still does not prove the running window. Keep heuristic metadata labeled `legacy_prefix`; managed-local caps and the global cap remain in force.
- `ProviderCapabilityResult.Multimodal` stays the broad flag: true when any of image, file, pdf, audio, or video is accepted. `Image` is separate and uses json tag `image`. A non-empty catalog `Input` list sets `Image` only for `image` or `images` and wins over the model-name heuristic. An empty catalog input list sets `Image` from `Multimodal`. The models.dev fallback sets `Image` from `SupportsMultimodal`. OpenRouter sets `Image` only for input modalities `image` or `images`. Manual capability results set `Image` from `Multimodal`. A stored auto result does the same unless its source is the oh-my-pi catalog and that model's input list is known, in which case `Image` comes from the catalog. Heuristic-only results set `Image` from the model-name check and keep `Multimodal` equal to `Image`. Managed local providers (`aurago-local` or `config.LocalLLMProviderID`) keep tool calling on and structured outputs and reasoning off; the lock also sets `Known` so callers that refuse unknown models still apply those flags, without overwriting `Source` when a catalog or models.dev result was found. A manual multimodal flag sets both `Multimodal` and `Image`. `routeSupportsRequest` uses `Image` for image parts.

- Anthropic caches only the system prefix before `# TURN CONTEXT`; dynamic suffixes are sent live without cache markers. Manual thinking requires 1024 tokens and a larger output cap; finite smaller requests disable it. Adaptive Claude families omit manual budgets. Preserve sampling/tool-choice compatibility with the selected thinking mode.
- Anthropic signed `thinking` and `redacted_thinking` blocks use the versioned internal `ReasoningContent` envelope through the SDK. Persist that string intact and use `ReasoningText` for display; never truncate, reinterpret, or synthesize signatures. Streaming signature deltas and unknown native block fields survive continuation.
- The retry owner tags attempt contexts with their parent: an attempt deadline counts toward failover, initiating caller cancellation stays neutral. Permanent 4xx errors do not retry; 429 respects Retry-After through a cancellable wait. Returned streams are never replayed.
- Recovery requires one finite generation confirmation after metadata succeeds or is unsupported. Auth, rate limits and server failures do not trigger additional generation calls. Probes are shutdown-cancellable, limited to thirty seconds, honor Retry-After and retain generation fencing. Wire `SetProbeUsageRecorder` to book every physical probe response.
- Exact provider/model registry prices precede family fallbacks. Provider usage partitions retain cache reads/writes; `EstimateMeasuredCost` returns nil for incomplete measurements or unsupported service tiers. Shared budget booking consumes each request-local capture once, with fallback estimates explicitly counted.

## Work Guidance

Use synthetic local HTTP/TLS fixtures for network policy tests.

## Verification

`go test ./internal/llm`, including real self-signed loopback fixtures and failover tests.

## Child DOX Index

None.
