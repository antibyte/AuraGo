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
- Both SSRF redirect checks drop credential headers (`Authorization`, `Proxy-Authorization`, `Cookie`, `X-Api-Key`, `X-Auth-Token`, `X-Goog-Api-Key`, and any header whose name contains `key`, `token`, `secret` or `auth`) on every hop that leaves the first request's exact scheme/host/effective-port origin. An http→https upgrade or apex→www redirect therefore loses `Authorization` too: configure the final URL. Verify `TestSSRFClientStripsCredentialHeadersAcrossOrigins`.
- Spotlight, Canary and Structure are retired, including request-local Guardian construction. Legacy options cannot attach those guards or manufacture trusted prompt envelopes. Never infer trust or skip a scan from input tags or copied system text; preserve the original human request.
- Optional content scans and the PromptSec judge accept only completed, strictly parsed verdicts. Scan every contiguous UTF-8-safe 4 KiB section with 512-byte overlap, at most eight; excess content is incomplete quarantine. Apply the judge once to the full original after local window scans, preserving its escalation mode without upstream truncation or normalized cache keys. Ingresses with an explicit content scan use local-only prechecks. Missing scanners, transport errors, unfinished responses and unknown verdicts never allow content, independently of the tool-execution fail-safe. Cache only complete valid verdicts.
- `QuarantineNotice` provides fixed local reason categories and an isolated source/reference, never original payloads, subjects or model-written reasons. Callers deliver it only through the original target's authorized route, preserving acknowledgment/deduplication and never running the original payload callback. A notice grants no quarantine bypass.
- Shell, Python and `run_tool` output always use `IsolateExternalData`. Only Game Maker source tools may use the copyable source form. Scan original source before rewriting role markers; cap raw input before escaping and preserve isolation form during compression and archival.
- Imported backup metadata cannot grant agent readability. Retain existing local grants only under the Vault's matching-value rules; new imported secrets default hidden.
- Token files use synced private temporary files and `fileutil` replacement. Cast media tickets expire, bind one path and invalidate after process restart; they grant GET/HEAD media reads only.
- Only the stored scope set `["cyd"]`, compared exactly like `Validate`, gets the short on-glass token (`scopesExactlyCYD`); any other set, including `cyd` mixed with another scope, gets a full-entropy token. A short token's stored prefix is `aura_...` and reveals no body characters. Verify `TestCreateMixedScopesNeverUsesShortToken` and `TestCYDTokenPrefixHidesBody`.

## Work Guidance

Register secrets for scrubbing; never print credentials in diagnostics or fixtures.

## Verification

`go test ./internal/security ./internal/fileutil` and server backup/logout/Cast tests.

## Child DOX Index

None.
