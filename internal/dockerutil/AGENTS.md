# Docker Engine transport

## Purpose and ownership

This package owns shared Engine endpoint normalization, transports, API negotiation and response-stream decoding.
Agent and service authorization stays with each caller.

## Contracts

- `managed_resources.go` owns reserved RustFS names and private volume-path recognition. Volume ownership remains the manager's responsibility; reserved names protect generic access and never authorize adoption. Runtime contracts live in `../rustfs/AGENTS.md`.

- Negotiate `GET /version` before a versioned operation, selecting the supported intersection (minimum 1.25 for managed-container Init, maximum `APIVersion`). Reject malformed, inaccessible or incompatible version metadata before mutation. Socket proxies need `VERSION=1`.
- `NewVersionTransportWithProbeTimeout` gives transports with slow connection setup (an Engine socket reached through SSH) a longer `/version` probe budget; `NewVersionTransport` keeps 5 s.
- `NewClientWithDialer` builds a pooled client over a caller-supplied dial function (TLS with client certificates, SSH tunnels) for the Docker Manager's hosts. Requests keep the `http://docker/v1.45/` URL for every kind, idle connections are reaped after a minute, `probeTimeout <= 0` keeps the 5 s probe and a nil dial function fails the request instead of panicking. Host records and secrets belong to `../dockerhost/AGENTS.md`.
- `Client.Dial` opens a raw connection with the client's own dial function (the endpoint dialer of `NewClient`, the caller's function of `NewClientWithDialer`; a missing function fails instead of panicking), `Client.APIVersion` returns the negotiated API version without the `v` ("1.45"; it probes once and follows the same re-probe rules as requests) and `Client.StreamingHTTPClient` is the client on the same transport without a total timeout, so streams (events, log follow) end with their context. Raw upgraded streams of the Docker Manager (exec attach) dial with `Dial` and prefix their paths with `APIVersion`, which is what makes them work on TLS, mutual TLS and SSH hosts.
- Bind negotiation state to one Engine transport. Honour cancellation while probing or waiting on another probe; failed probes must not poison future attempts. Forward idle-connection cleanup.
- A transport failure the caller did not cancel, or an HTTP 400 answer, clears the negotiated version so the next request probes again; the failed request itself is returned unchanged and never resent.
- Never replay a mutation after network, body-read or server failure. Only safe reads may retry. Streaming exec uses its caller context; cancellation does not prove remote process termination.
- Raw upgraded terminal streams and Invasion use the same negotiation policy. Preserve Unix sockets, Windows named pipes and explicitly configured TCP engines.
- Drain Docker JSON-message streams (pull, build, push) with `DrainJSONMessages`: an `errorDetail`/`error` event fails the operation even after HTTP 200, a stream cut inside a message or a line over `MaxJSONMessageLine` is an error, and non-2xx pull bodies are read through `ReadErrorBody`.
- Engine error text (a non-2xx body or a stream error event) goes through `EngineErrorMessage` (the JSON `message`, else the body) and `SanitizeOneLine` (printable runes, one line, cut at a rune boundary) before it reaches a log, an API answer or the agent (`DoJSON` keeps the raw body and applies only `SanitizeOneLine`).
- `Client.DoJSON` returns status 0 with an error when no HTTP response arrived (request build, transport, negotiation) and the real status with an error for a non-2xx answer; classify Engine answers by that code, never by matching error text.

## Verification

Run Docker transport and tool regressions, managed-sidecar package suites, server terminal tests and Linux race tests. Local protocol fixtures do not qualify provider or hardware behavior.

## Child DOX Index

None.
