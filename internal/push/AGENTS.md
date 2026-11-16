# Web Push

## Purpose

Browser notification subscriptions and encrypted Web Push delivery.

## Ownership

`manager.go` owns the existing subscription database, VAPID keys and delivery. Server owns administrator authentication for subscribe/unsubscribe routes.

## Local Contracts

- Validate public HTTPS endpoints, 16-byte auth secrets and valid P-256 public keys before storage; failed updates preserve existing rows. Never log full endpoints or provider errors containing them.
- Revalidate stored subscriptions before each send and use a fresh DNS-pinned strict public client without proxies or redirects. Invalid legacy rows remain stored but never send.
- Before-send observers run at the prepared HTTP request, after local encryption/VAPID preparation; local failures never mark a possible send.
- Only 2xx counts as delivery. Only 404/410 removes a subscription; transient failures retain it. No per-user database or automatic grant migration.

## Work Guidance

Tests use generated keys and synthetic HTTP transports; never send real notifications.

## Verification

`go test ./internal/push` and server `TestPushMutationsRequireAdministrator`; security strict-public DNS/redirect tests cover the shared transport.

## Child DOX Index

None.
