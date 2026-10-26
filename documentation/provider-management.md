# Provider management

Create reusable connections under **Config → Providers**, then select their IDs in
the main LLM, helper, router, media, SIP or other feature settings.

## Credentials and drafts

Enter credentials in Setup or the Provider editor. AuraGo stores them in its
encrypted Vault. Do not put API keys, OAuth client secrets or tokens in YAML.
The existing plaintext-secret import remains available for older installations.

An empty credential value in an API request removes that credential. In the
editor, use the explicit removal button to clear a saved masked field. The displayed
mask `••••••••` means keep the existing valid credential; it is never a secret.
A mask without a matching existing value is rejected. Invalid older mask credentials are reported
and their calls are blocked; AuraGo does not rotate or delete them automatically.
Changing authentication mode does not automatically revoke stored OAuth tokens.
An explicit revoke or provider deletion removes the relevant token.

A blank provider selection inherits its documented parent, including OAuth.
Selecting another provider uses only that provider's credentials, even when its
URL is identical to the parent's. Keyless local providers remain keyless.
Missing provider IDs are configuration errors and do not select a default API.

Incomplete provider drafts can be saved. Diagnostics explain missing models,
endpoints, gateway fields or credentials. URLs, when supplied, must be absolute
HTTP or HTTPS URLs without embedded usernames/passwords; local and custom hosts
are supported. Deletion is blocked until every saved reference has been changed
or removed, including references in disabled features.

## Save conflicts

The editor keeps the revision it loaded. Save buttons are disabled while saving.
If another client or configuration publication changes that revision, the save
fails and the draft stays open. Choose **Reload saved settings** explicitly;
there is no automatic retry or overwrite.

## Status, limits and tests

The diagnostics tab reads passive status: assigned references, the active route
and fallback, attributed runtime success/errors, recovery-probe state, OAuth
expiry, and effective context/output limits with their sources. An unknown
provider association remains unknown. Reading status makes no provider request.

Exact provider/model metadata takes precedence. A deterministic model-only
fallback is labeled `model_only_fallback`; pricing from another provider is an
estimate. Manual prices retain their source and estimate flags when saved.

Save the connection before testing it. **Metadata test** checks a supported
metadata endpoint and never generates text automatically. Unsupported tests are
reported as unsupported. **Model test** is an explicit generation that may cost
money. It makes one generation attempt, books usage through the shared budget
path, and respects budget policy. Tests run for at most 20 seconds, with at most
one active test per provider. They use a frozen saved configuration and accept no
endpoint or credential overrides. A result for an older revision does not
confirm newer settings. The protected first-run Setup test remains metadata-only.

## Retry and timeout settings

Retry policy belongs to each failover manager and is captured once per request;
task routes inherit that snapshot. `circuit_breaker.llm_timeout_seconds` is the total
request budget (default 600 seconds); a shorter caller deadline wins.
For streaming, this budget and the per-attempt timeout govern opening/retry only.
An accepted stream retains the separate three-minute inactivity limit and is
never requested again automatically. The configured per-attempt timeout has a
120-second minimum. Retry waits include cancellable jitter and never shorten
`Retry-After`; no operation makes more than ten attempts. Transport/attempt
timeouts can trigger retry/failover; caller cancellation remains neutral.
Displayed errors remove URL usernames/passwords, query strings and fragments.

## HTTP and AgoDesk compatibility

Provider reads remain JSON arrays. Clients must preserve the opaque quoted
`ETag` from `GET /api/providers` and submit it as `If-Match` on provider
writes, including price application. Successful writes return the new revision.
Revisions are unique to the server instance and change on configuration
publication and provider-credential changes.

| Request | Contract |
| --- | --- |
| `GET /api/providers` | Array plus `ETag`; credential masks only. |
| `PUT /api/providers` | Full provider array plus `If-Match`. |
| `GET /api/providers/pricing?id=main` | Read pricing candidates; no persistence. |
| `POST /api/providers/pricing?id=main` | Existing price-application body plus `If-Match`. |
| `GET /api/providers/status` | Authenticated passive status without probes or secrets. |
| `POST /api/providers/test` | `{"provider_id":"main","probe_kind":"metadata"}`; kind may be `generation`, defaults to `metadata`. |
| `PUT /api/config` with any `providers` field | 400, `provider_use_api_providers`, before any write. |

Missing revisions return **428**; stale revisions return **412**. A referenced
provider deletion returns **409**, code `provider_in_use`, with the complete
reference list. Newly introduced dangling references return **409**,
`provider_reference_conflict`. Existing dangling references can be repaired
without silently changing other selections.

AgoDesk provider list/get responses expose the same `revision`; upsert/delete
requests must submit it and consume the returned revision. `force` does not
bypass reference protection. Older HTTP and AgoDesk writers must be updated.
The protected initial Setup creation uses the same validation and credential
rules and needs no revision before initial configuration exists.

Provider writes share one serialized transaction: validate a deep copy, apply
allowlisted YAML and reversible Vault changes, then publish the snapshot and
update clients/budget. Storage rejection does not publish candidate state.
Rollback failures are reported explicitly and require operator investigation.
