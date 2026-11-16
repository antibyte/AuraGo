# Discord bot

## Purpose

Own Discord ingress, API delivery and connection lifecycle.

## Ownership

- `bot.go`: admission, conversation routing, agent turns and bound delivery bridges.
- `lifecycle.go`: start/stop serialization, generation cancellation, gateway reconnect and worker drain.
- `transport.go`: API origin, request budget and rate-limit retries; `connection.go` owns one-shot configuration probes.
- Root owns global permissions; `internal/tools` owns pending-question authority and tool gates; `internal/server` owns configuration reload and shutdown.

## Local Contracts

- Each generation owns its context, gateway, turns, queued workers and REST calls. Stop prevents new admission, cancels and drains all work. A later stop invalidates a prepared start. Old callbacks keep their original session and cannot send through a replacement.
- Disable SDK recursive REST retries and uncancellable gateway reconnects. REST calls use the exact Discord HTTPS origin without redirects, at most two additional 429 attempts and a 30-second total budget. Preserve Retry-After, including waits longer than the remaining budget; never replay 502 or uncertain network mutations.
- The transport owns rate-limit waits; remove SDK rate-limit scheduling headers so its bucket locks cannot start independent, uncancellable sleeps. Gateway reconnect backoff and handshake are generation-bound.
- Register tokens before SDK construction; scrub provider errors. Preserve suppressed link embeds, attachment confinement and delivery-failure counters.
- Sender/channel checks precede worker admission. Authorized stop and pending-question text bypass saturation; source identity and question ID remain mandatory. Keep separate Discord conversation contexts and operator commands limited to authorized DMs.

## Work Guidance

- Pass generation/request contexts and the originating session through helpers. Never use a global replacement session from an old turn.
- Keep fixtures local; no real Discord credentials or API calls in tests.

## Verification

- `go test ./internal/discord ./internal/server` (focused Discord/channel/question selectors when appropriate).
- Linux `go test -race -p 1 ./internal/discord`.
- Lifecycle, gateway-handshake and HTTP fixtures cover cancellation/drain, stale startup, 429 replay/budget, 502 single-send, origin/redirect rejection and credential redaction.

## Child DOX Index

None.
