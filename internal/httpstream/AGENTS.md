# HTTP stream budgets

## Purpose

Finite write deadlines compatible with sustained SSE and passive media streams.

## Ownership

This package wraps HTTP writers; server owns acceptance, cancellation and drain.

## Local Contracts

- Renew the existing write budget after successful stream writes/flushes for SSE, MJPEG and audio/video. Ordinary/error responses keep absolute deadlines. Failure cancels the request and cannot revive a stream.
- Preserve ResponseController access, Unwrap and WebSocket hijacking.

## Work Guidance

Do not replace a stream's bounded idle budget with an unlimited timeout.

## Verification

`go test ./internal/httpstream` and server/Tailscale streaming tests.

## Child DOX Index

None.
