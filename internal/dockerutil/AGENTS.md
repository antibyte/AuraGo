# Docker Engine transport

## Purpose and ownership

This package owns shared Engine endpoint normalization, transports and API negotiation.
Agent and service authorization stays with each caller.

## Contracts

- Negotiate `GET /version` before a versioned operation, selecting the supported intersection (minimum 1.25 for managed-container Init, maximum `APIVersion`). Reject malformed, inaccessible or incompatible version metadata before mutation. Socket proxies need `VERSION=1`.
- Bind negotiation state to one Engine transport. Honour cancellation while probing or waiting on another probe; failed probes must not poison future attempts. Forward idle-connection cleanup.
- Never replay a mutation after network, body-read or server failure. Only safe reads may retry. Streaming exec uses its caller context; cancellation does not prove remote process termination.
- Raw upgraded terminal streams and Invasion use the same negotiation policy. Preserve Unix sockets, Windows named pipes and explicitly configured TCP engines.
- Drain Docker JSON-message streams (pull, build, push) with `DrainJSONMessages`: an `errorDetail`/`error` event fails the operation even after HTTP 200, a stream cut inside a message or a line over `MaxJSONMessageLine` is an error, and non-2xx bodies are read through `ReadErrorBody`.

## Verification

Run Docker transport and tool regressions, managed-sidecar package suites, server terminal tests and Linux race tests. Local protocol fixtures do not qualify provider or hardware behavior.

## Child DOX Index

None.
