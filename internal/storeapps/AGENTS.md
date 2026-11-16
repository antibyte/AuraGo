# Store application clients

## Purpose

Bounded clients for the explicitly connected Store apps.

## Ownership

This package owns upstream API contracts and projection of external responses.
`internal/server` owns Vault settings, verification, grants and revocation.
`internal/agent` owns schemas, dispatch and external-data isolation.

## Local Contracts

- Caller contexts carry the verified connection lifetime. Never retain credentials
  or bypass cancellation. Register secrets before constructing network clients.
- Use the same-origin HTTP client, fixed API paths, bounded bodies and safe errors.
  Mutations are never retried; transport/response failures can mean an unknown outcome.
- Node-RED exposes flow names/IDs and selected Inject nodes only. Trigger an existing
  Inject without a payload override; acceptance never proves flow completion.
- OliveTin uses its Connect JSON API, selected binding IDs and upstream argument
  validation. No configuration, shell, deployment, restart or arbitrary RPC surface.

## Work Guidance

- Verify API fields against official source before changing a client. Project
  responses into the minimum tool result; do not return upstream configuration.

## Verification

- Run `go test ./internal/storeapps` and the Store server/agent permission tests.
  Fixtures do not establish image compatibility or actual workflow completion.

## Child DOX Index
