# WebAuthn, biometric login and FIDO2 implementation plan

Status: planned; no authentication behavior has been changed.
Source baseline: `1a6646906` on `main`, inspected on 2026-10-22.

## Goal and recommended scope

Add an optional WebAuthn login to AuraGo's existing owner account. Support platform authenticators such as Windows Hello, Touch ID, Face ID and Android screen unlock, and external FIDO2 security keys through the browser's native credential dialog. Actual availability depends on the browser, operating system and authenticator.

Recommended first version: a verified WebAuthn assertion is a complete alternative login. The existing password login continues to require TOTP whenever TOTP is enabled. Enabling WebAuthn explicitly explains this alternative; it does not disable TOTP or silently change the password route. Keep the owner password configured for recovery and native clients. WebAuthn defaults off.

The login-policy preference was requested during planning. This document uses the recommended passwordless alternative unless the operator selects password plus WebAuthn or configurable modes before implementation. Do not implement multiple modes speculatively.

Biometric recognition happens locally. AuraGo receives a signed authentication result, never fingerprints, face images or an authenticator PIN. Require local user verification, while allowing the operating system to choose biometrics or PIN; do not promise to enforce biometrics exclusively. See the [WebAuthn specification](https://www.w3.org/TR/webauthn-3/#sctn-biometric-privacy).

Use one WebAuthn implementation for both platform and external authenticators. The first version targets FIDO2 keys with user verification. Older touch-only U2F keys cannot provide the required verification for this passwordless policy; supporting them requires the separate password-plus-key policy, not a weaker passwordless assertion.

## Existing integration points

| Concern | Current source and behavior |
| --- | --- |
| Owner authentication | `internal/server/auth_handlers.go`: `handleAuthLogin` verifies password and optional TOTP, then uses `SetSessionCookie`. There is one owner account, not a user directory. |
| Sensitive changes | `internal/server/auth_step_up.go`: `verifyAdminCredentials` / `requireAdminStepUp` verify the current password and active TOTP. Password changes, TOTP confirmation and TOTP removal call this path. |
| HTTP boundaries | `internal/server/auth.go`: exact authentication exemptions, password-bootstrap lockdown, browser-origin checks, login backoff and cookie helpers. |
| Session lifecycle | `internal/server/auth_session.go` and `auth_session_revocation.go`: explicit activity can extend a still-valid session to at least ten minutes; status polling does not renew it. Logout revocation is persistent. |
| Route registration | `internal/server/server_routes.go`, `server_routes_ui.go`, and the recovery mounts in `assets_handlers.go`. |
| Configuration | `internal/config/config_types.go` contains the inline `Auth` struct. `patchAuthConfig` serializes auth configuration and Vault publication under `CfgSaveMu`. |
| Browser login | `ui/login.html`, `ui/js/login/main.js`, `ui/css/login.css`; retain the session-confirmation wait and redirect validation. |
| Owner settings | `ui/cfg/auth.js`; the config search catalog must resolve the actual `auth` paths. |
| Durable state | Existing encrypted Vault, including batch writes and atomic file replacement. Generic Vault routes already reserve `auth_` keys. |
| Native clients | The Rust TUI posts password/TOTP to `/auth/login`. Preserve this contract. |
| Missing full UI resources | The embedded recovery login already accepts password/TOTP. Preserve this route and asset repair without adding another WebAuthn implementation there. |

Graph analysis found the shared step-up dependency chain through password change, TOTP enrollment/replacement and TOTP removal, plus their tests. GitNexus reported 11 affected symbols as a lower bound, with unknown index freshness and no complete process coverage. This is a scope aid, not security clearance; actual source and focused tests decide.

## Trust, configuration and deployment prerequisites

- Add `auth.webauthn.enabled` (default `false`), `auth.webauthn.rp_id`, and an explicit list of complete `auth.webauthn.origins`. Validate them on save and load. No public request, forwarded host or model input may select the relying-party identity.
- Recommend a stable DNS name and a trusted HTTPS certificate, including for LAN access. Reuse AuraGo's proxy trust policy. A self-signed certificate warning that a user clicked through is not proof of a usable secure context.
- The RP ID contains the domain only; origins include scheme, host and effective port. Only explicitly listed origins valid for that RP ID are admitted. No wildcard origins, paths, userinfo or request-derived additions. Use one origin per ceremony and bind it to that ceremony.
- LAN IP access, a LAN hostname and an unrelated Tailscale hostname do not automatically share credentials. The settings page shows the configured login URL. A stable RP ID can support multiple valid, explicitly allowed origins, including different ports. See [RP ID and origin rules](https://www.w3.org/TR/webauthn-3/#rp-id).
- An explicit `http://localhost` development configuration is permitted where the browser supports it. Do not create an insecure remote development bypass or promise WebAuthn over arbitrary IP addresses.
- Require configured owner authentication, a readable Vault and valid relying-party settings before enrollment. Login requires at least one registered credential as well. Invalid WebAuthn configuration must never open the owner setup or disable existing authentication.
- RP changes never silently rewrite stored credentials. Require step-up and an explicit reset/re-enrollment flow with a working password fallback; origin-only changes retain credentials only when valid for the unchanged RP ID. Cancel pending ceremonies on any policy change.
- Enable/disable and RP/origin changes use dedicated session-and-step-up-protected auth settings handlers. Generic config writes, config import and bearer-only requests must not bypass this policy. Preserve legitimate offline operator configuration and the existing authenticated backup import boundary.

## Implementation design

### Protocol library

Use `github.com/go-webauthn/webauthn` for credential parsing and verification. Pin a reviewed release and its checksums; do not write a CBOR, COSE, attestation or signature verifier. At planning time, the latest release is [v0.18.2](https://github.com/go-webauthn/webauthn/releases/tag/v0.18.2); its [module](https://github.com/go-webauthn/webauthn/blob/v0.18.2/go.mod) declares Go 1.26 and toolchain 1.27.1, compatible in version requirements with AuraGo's required Go 1.27.2. Recheck advisories and the CGO-free build during implementation.

Registration uses `userVerification: required`, `residentKey: preferred`, and `attestation: none`, with no mandatory authenticator attachment. Prefer discoverable passkeys while allowing UV-capable FIDO2 credentials that are not discoverable. The existing single owner allows explicit login with `allowCredentials`; a discoverable-only account lookup is unnecessary. Do not add a server-side Bluetooth/USB stack, external identity provider, attestation metadata service or frontend framework.

### Credential persistence

Keep a small, versioned credential registry under the reserved Vault key `auth_webauthn_state`. Reuse Vault read/write and atomic replacement instead of adding an authentication database or schema migration.

- Persist a stable random 32-byte owner handle, the RP ID and the library's credential record, plus a user-selected label, creation time and last successful use. Preserve the credential public key, transport hints, authenticator counter and backup-related flags needed by the verifier. Store no private keys or biometric templates.
- Bound registry size to 16 credentials and labels to a documented small length. Reject duplicate credential IDs and unknown record versions rather than replacing data. Never overwrite a malformed existing registry with an empty one.
- Serialize registry mutations and revalidate the current credential and configuration generation before committing an authentication result. Deletion, disablement and backup import invalidate pending work and stale in-memory state. Define lock order with `CfgSaveMu` before introducing a registry mutex.
- Persist accepted authenticator state before issuing a login cookie. A failed Vault write produces an explicit failure with no successful login, no partially published state and no automatic replay of the finished ceremony.
- Counter-zero synced credentials remain usable. Treat a positive counter regression as an anomaly: reject that ceremony, emit a scrubbed diagnostic and offer password/TOTP login. Do not permanently delete or label a credential as cloned based on that signal alone. Test concurrency and backup-related flags using the pinned library.
- Keep this registry inaccessible to agent tools, Python/skill export, generic secret prompts and public status. Verify both Vault reservations and export denylists; a public key is not a secret, but replacing it grants account access.
- Existing encrypted backups must carry the registry with the stable owner handle and matching RP identity. Verify import publication and corrupt/old-format handling. Restoring an older backup can restore previously removed keys; document that consequence. A registry-format upgrade requires a backup and a tested conversion before publication.

### Challenges and request boundaries

Add a bounded, server-owned in-memory ceremony store on the server lifecycle. Store library session data, a random transaction ID, the originating browser binding, operation, RP/origin and configuration generation. Registration additionally binds the current owner session and its successful credential step-up.

- Generate challenges with server-side cryptographic randomness. Use a two-minute lifetime and small per-browser/global limits, with expiry cleanup and shutdown cancellation.
- Bind unauthenticated login ceremonies to a separate opaque, HttpOnly, SameSite=Strict browser cookie; use Secure on trusted HTTPS. Authenticated registration also binds its originating session. A transaction ID alone grants nothing.
- Consume transactions atomically on finish, including failed finishes. Expired, wrong-browser, wrong-operation, cross-origin, configuration-stale and replayed responses fail. A fresh attempt obtains a fresh challenge.
- Require same-origin JSON POSTs with exact origin validation, bounded request bodies and non-cacheable responses on both public login ceremony endpoints. Verify client-data origin, RP binding, signature, expected owner/credential, user presence and required user verification through the library.
- Reuse the existing IP/account verification backoff and concurrency policy without introducing a global owner lockout. Rate-limit begin requests as well as verification; browser cancellation without an assertion is not an invalid-credential guess.
- Do not log assertions, challenges, raw credential IDs, cookies or authenticator details. Use fixed error codes and a scrubbed event category; retain accessible browser messages.

### Proposed HTTP surface

| Method and route | Protection and result |
| --- | --- |
| `POST /auth/webauthn/login/begin` | Exact public exemption; origin, availability and rate checks; returns options and a browser-bound transaction. |
| `POST /auth/webauthn/login/finish` | Exact public exemption; single-use transaction and full verification; persists state and issues the existing AuraGo session cookie. |
| `GET /api/auth/webauthn/credentials` | Browser owner session; returns labels and administrative metadata, not raw Vault contents. |
| `POST /api/auth/webauthn/register/begin` | Browser owner session plus current password and active TOTP; returns enrollment options. |
| `POST /api/auth/webauthn/register/finish` | Same valid owner session and the step-up-bound enrollment transaction; publishes only a validated credential. |
| `PATCH /api/auth/webauthn/credentials/{id}` | Same-origin browser owner session plus step-up; rename a registered credential. |
| `DELETE /api/auth/webauthn/credentials/{id}` | Same-origin browser owner session plus step-up; prevents future authentication using that credential and cancels pending transactions. |
| `PUT /api/auth/webauthn/settings` | Same-origin browser owner session plus step-up; validates and serializes the feature policy. |

Only the two public login routes join `authBypassExactPaths`. They must remain blocked during the missing-password bootstrap lockdown, and must not enter `noPasswordExactPaths`. No prefix-wide `/auth/webauthn` exemption. Add only a minimal availability flag to public auth status; never expose labels or credential counts there.

Passkey login reuses `SetSessionCookie`, `sanitizeRedirectTarget`, the authenticated-session wait in the browser and existing logout/activity behavior. Keep password/TOTP step-up unchanged for password, TOTP and passkey administration in the first version. A stolen unlocked browser session therefore cannot enroll a replacement key by itself.

Credential removal revokes subsequent uses of that key, not sessions that have already been issued. Existing sessions retain the current logout/revocation/expiry behavior. State this in the removal dialog; do not claim that credential removal signs a stolen browser out. Global or credential-linked session revocation is a separate change if required.

## Browser experience

- Add one primary action, "Sign in with a passkey or security key", beside the existing password path. The browser handles the authenticator chooser and any supported phone-assisted flow.
- Check `window.isSecureContext`, `PublicKeyCredential` and `navigator.credentials` before offering the action. Platform-biometric availability must not hide external-key support. Unsupported contexts show a clear reason and retain password/TOTP.
- Use native browser APIs with a small, tested base64url/ArrayBuffer adapter; feature-detect newer JSON conversion APIs. Keep current asset packaging, entry-page layout, theme behavior and content security policy.
- Show a pending state and allow cancellation. Handle cancellation, timeout, missing credential, unsupported authenticator, server rejection and lost network responses distinctly without automatically starting another ceremony. Prevent concurrent button submissions and cancel/ignore work on navigation.
- Under existing security settings, add explicit enablement, RP/origin validation, a list of named credentials with last-use information, add/rename/remove actions, and a local capability diagnostic. Enrollment success is the connection test; do not prompt for biometric access during passive status reads.
- Preserve the normal config draft until the protected settings save succeeds. Use existing modals and accessible focus/status handling. Translate login, settings, help and errors in all 16 locales. Keep config search mapped to the actual new auth paths.
- Recommend registering a second authenticator and keeping password/TOTP recovery material before relying on the new login. The password cannot be removed in this version. No new unauthenticated reset endpoint or SMS recovery is introduced.

## Execution sequence and acceptance

1. **Configuration, trust and library**
   - [ ] Validate the selected login policy and document its TOTP semantics before implementation.
   - [ ] Add the pinned dependency, config defaults/template/validation and protected settings handling.
   - [ ] Test HTTPS/origin/RP validation, trusted versus forged proxy headers and rejection of generic auth-policy bypasses.
2. **Registry and ceremony lifecycle**
   - [ ] Implement focused server-owned registry and ceremony files; reuse Vault primitives.
   - [ ] Test restart/backup stability, duplicate/limit handling, corrupt records, Vault errors, concurrent registration/login/removal, stale generations and single-use expiry.
3. **Enrollment and login endpoints**
   - [ ] Wire the routes in the normal server, exact auth exemption table and relevant asset-recovery boundaries.
   - [ ] Test successful registration/assertion, required UV, wrong signatures/RP/origin/user/credential, cross-browser replay, disabled/removed keys, throttling and locked Vault/bootstrap behavior.
   - [ ] Preserve existing password/TOTP, sensitive-change step-up, logout, activity renewal, redirects and native TUI request contracts.
4. **Login and settings UI**
   - [ ] Implement the native credential flow and small encoding helper, credential management and all locales.
   - [ ] Add browser regression coverage using a virtual WebAuthn authenticator with and without UV, plus cancellation, missing API, insecure context and failed persistence paths.
   - [ ] Verify session confirmation, focus/keyboard behavior, responsive layout and config save failures.
5. **Documentation, packaging and real devices**
   - [ ] Update owning server/UI/security contracts, operator security/HTTPS documentation and resource-packaging guidance as needed.
   - [ ] Run focused tests first, then appropriate package checks and the UI bundle integrity check. Build the version-bound asset set and matching binary for deployment validation; a package archive alone does not activate the new login.
   - [ ] Manually verify Windows Hello with Edge/Chrome, Apple platform authentication with Safari, Android with Chrome and a UV-capable USB/NFC FIDO2 key on supported clients.
   - [ ] Exercise direct trusted HTTPS, a configured trusted reverse proxy, allowed localhost development, wrong hostname/untrusted certificate and missing-resource recovery.
   - [ ] Register two authenticators, log out, authenticate with each, remove one and confirm its next assertion fails while password/TOTP still works.

Representative checks after implementation, using the repository cache wrapper if ignored scratch files are present:

```powershell
./scripts/go-test-with-cache.ps1 ./internal/config ./internal/security ./internal/server ./ui ./internal/webassets ./cmd/assetpack
node scripts/build-ui-bundles.js --check
git diff --check
```

Use focused `WebAuthn|AuthStepUp|AuthSession|AuthBypass|AuthLockdown|ExternalAssetsRecovery` test selections while iterating. Run dependency vulnerability checks and the CGO-free build with the pinned production compiler. Passing virtual-authenticator tests is not evidence that a physical key, platform biometrics or a deployed proxy works; record those acceptance results separately.

## Deliberate limits

No multi-user redesign, separate biometric database, browser framework, identity-provider sidecar, mandatory cloud account, automatic attestation service, passkey-only recovery scheme, U2F compatibility mode or native TUI biometric flow. These are not needed for the recommended first version. Password/TOTP remains a usable alternative, so the deployment as a whole retains that alternative's phishing exposure even though the WebAuthn ceremony is origin-bound.
