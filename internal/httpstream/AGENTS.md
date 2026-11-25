# HTTP stream budgets

## Purpose

Finite write deadlines compatible with sustained SSE and passive media streams.

## Ownership

This package wraps HTTP writers; server owns acceptance, cancellation and drain.

## Local Contracts

- Renew the existing write budget after successful stream writes/flushes for SSE, MJPEG and audio/video. Ordinary/error responses keep absolute deadlines. Failure cancels the request and cannot revive a stream. Reject stream writes after cancellation, including cancellation during deadline renewal.
- A handler deadline set through `http.ResponseController` can only shorten a stream's budget; a later deadline never extends it, and a zero deadline falls back to a fresh budget, never unlimited. Handler deadlines are absolute, as in `net/http`, so streaming handlers re-arm before each frame. When cancellation (including the shutdown drain) races a deadline change, the wrapper expires the connection's write deadline again. Ordinary responses pass non-zero handler deadlines through. Server wrappers (`statusRecorder`, `gzipResponseWriter`) must `Unwrap` so handlers reach the connection. Verify `TestWriteTimeoutKeepsStricterHandlerDeadline`.
- Preserve ResponseController access, Unwrap and WebSocket hijacking.

## Work Guidance

Do not replace a stream's bounded idle budget with an unlimited timeout.

## Verification

`go test ./internal/httpstream` (including `TestWriteTimeoutKeepsStricterHandlerDeadline`) and server/Tailscale streaming tests, including `TestSSEHandlerDeadlineDropsStalledClientThroughMiddleware` and `TestSSEHandlerDeadlineKeepsActiveClientThroughMiddleware`.

## Child DOX Index

None.
