# Security primitives

## Purpose

Vault, token storage, URL trust and model-assisted security verdicts.

## Ownership

This package owns reusable primitives; server owns session and HTTP policy.

## Local Contracts

- Public-only egress rejects unspecified/private/reserved and IPv6 transition addresses; retain DNS pinning and redirect validation. Explicit LAN integration permissions remain separate.
- Administrator-selected integration bases require absolute HTTP(S) URLs without userinfo, query or fragment. Credentialed API redirects stay on the exact scheme/host/effective-port origin; public download redirects use a fresh pinned client without inherited headers.
- Cache only completed valid Guardian verdicts. Provider failures are unavailable scans, never cached domain decisions.
- Imported backup metadata cannot grant agent readability. Retain existing local grants only under the Vault's matching-value rules; new imported secrets default hidden.
- Token files use synced private temporary files and `fileutil` replacement. Cast media tickets expire, bind one path and invalidate after process restart; they grant GET/HEAD media reads only.

## Work Guidance

Register secrets for scrubbing; never print credentials in diagnostics or fixtures.

## Verification

`go test ./internal/security ./internal/fileutil` and server backup/logout/Cast tests.

## Child DOX Index

None.
