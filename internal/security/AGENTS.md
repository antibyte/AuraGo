# Security primitives

## Purpose

Vault, token storage, URL trust and model-assisted security verdicts.

## Ownership

This package owns reusable primitives; server owns session and HTTP policy.

## Local Contracts

- OAuth rotation uses compare-and-swap against the exact previous Vault value under both process mutex and file lock; revocation or a new login cannot be overwritten. Rotated values remain agent-hidden.

- Public-only egress rejects unspecified/private/reserved and IPv6 transition addresses; retain DNS pinning and redirect validation. Explicit LAN integration permissions remain separate.
- Administrator-selected integration bases require absolute HTTP(S) URLs without userinfo, query or fragment. Credentialed API redirects stay on the exact scheme/host/effective-port origin; public download redirects use a fresh pinned client without inherited headers.
- The same-origin policy is implemented in the leaf package `internal/httporigin` (shared with `internal/llm`, which this package imports); `SameHTTPOrigin` and `SameOriginRedirect` here are wrappers around it.
- `NewSSRFProtectedHTTPClient` re-validates and re-pins every redirect hop but follows any public origin (307/308 re-send the body); keep it for uncredentialed fetches such as returned-media downloads. Credentialed callers use `NewSSRFProtectedHTTPClientSameOrigin`: `httporigin.SameOriginRedirect` first (a foreign target is refused unresolved), then the unchanged SSRF re-check. `internal/audit` `TestCredentialedHTTPClientsBindRedirectsToOrigin` guards both client kinds for the literal header names it knows (see its blind-spot comment); its `credentialedRedirectPending` ratchet of unbound files may only shrink.
- Cache only completed valid Guardian verdicts. Provider failures are unavailable scans, never cached domain decisions.
- Imported backup metadata cannot grant agent readability. Retain existing local grants only under the Vault's matching-value rules; new imported secrets default hidden.
- Token files use synced private temporary files and `fileutil` replacement. Cast media tickets expire, bind one path and invalidate after process restart; they grant GET/HEAD media reads only.
- Only the stored scope set `["cyd"]`, compared exactly like `Validate`, gets the short on-glass token (`scopesExactlyCYD`); any other set, including `cyd` mixed with another scope, gets a full-entropy token. A short token's stored prefix is `aura_...` and reveals no body characters. Verify `TestCreateMixedScopesNeverUsesShortToken` and `TestCYDTokenPrefixHidesBody`.

## Work Guidance

Register secrets for scrubbing; never print credentials in diagnostics or fixtures.

## Verification

`go test ./internal/security ./internal/fileutil` and server backup/logout/Cast tests.

## Child DOX Index

None.
