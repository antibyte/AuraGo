# WebAuthn, biometric login and FIDO2 implementation plan

Status: planned; no authentication behavior has been changed.
Source baseline: `1a6646906` on `main`, inspected on 2026-10-22.

## Goal and selected scope

Add an optional WebAuthn login to AuraGo's existing owner account. Support platform authenticators such as Windows Hello, Touch ID, Face ID and Android screen unlock, and external FIDO2 security keys through the browser's native credential dialog. Actual availability depends on the browser, operating system and authenticator.

Support both login variants through a dropdown backed by `auth.webauthn.login_mode`. WebAuthn defaults off; enabling it requires an explicit mode selection and at least one registered credential. Keep the owner password configured. Both modes use the same credential registry, verifier and session lifecycle.

| Mode | Successful browser login | Existing password/TOTP route |
| --- | --- | --- |
| `passkey` - Passkey sign-in | A WebAuthn assertion with required local user verification; no password or TOTP on this path. | Password plus active TOTP remains the fallback; password alone is allowed only when TOTP is not enabled. |
| `second_factor` - Password plus WebAuthn | Verify the password first, then require a WebAuthn assertion before issuing a session. | Password plus TOTP remains an alternative second-factor path when TOTP is enrolled. Without TOTP, password alone must never issue a session. |

TOTP and WebAuthn are alternative second-factor methods in `second_factor` mode; do not require a third TOTP step after successful password plus WebAuthn. A browser cancellation, unsupported authenticator or service error must never downgrade the chosen policy. Switching modes requires a valid owner session plus credential step-up, cancels pending ceremonies and explains the changed login/recovery behavior. No third mode or additional fallback toggle is needed; the existing TOTP setting determines TOTP availability.

Biometric recognition happens locally. AuraGo receives a signed authentication result, never fingerprints, face images or an authenticator PIN. Require local user verification, while allowing the operating system to choose biometrics or PIN; do not promise to enforce biometrics exclusively. See the [WebAuthn specification](https://www.w3.org/TR/webauthn-3/#sctn-biometric-privacy).

Use one WebAuthn implementation for both platform and external authenticators. The first version targets FIDO2 keys with local user verification in both modes. Keep older touch-only U2F keys outside this version; selecting `second_factor` does not silently lower the required verification or change credential eligibility.

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
| Native clients | The Rust TUI posts password/TOTP to `/auth/login`. Preserve its request shape while enforcing the selected policy server-side. Without TOTP in `second_factor` mode, return a clear browser-login-required result; do not mint a session from password alone. |
| Missing full UI resources | The embedded recovery login currently accepts password/TOTP. It must also support the minimal password-plus-WebAuthn flow when that is the only permitted login, using the same small, tested client helper and backend routes. Asset repair must not require weakening the selected policy. |

Graph analysis found the shared step-up dependency chain through password change, TOTP enrollment/replacement and TOTP removal, plus their tests. GitNexus reported 11 affected symbols for that path and 13 for `handleAuthLogin`, both as lower bounds with unknown index freshness and no complete process coverage. Login callers include the normal UI and embedded recovery mounts, so both must enforce the same mode. This is a scope aid, not security clearance; actual source and focused tests decide.

## Trust, configuration and deployment prerequisites

- Add `auth.webauthn.enabled` (default `false`), `auth.webauthn.login_mode` (enum `passkey` or `second_factor`), `auth.webauthn.rp_id`, and an explicit list of complete `auth.webauthn.origins`. Require a valid mode whenever enabled; reject missing/unknown modes rather than guessing a weaker policy. No public request, forwarded host or model input may select the mode or relying-party identity.
- Recommend a stable DNS name and a trusted HTTPS certificate, including for LAN access. Reuse AuraGo's proxy trust policy. A self-signed certificate warning that a user clicked through is not proof of a usable secure context.
- The RP ID contains the domain only; origins include scheme, host and effective port. Only explicitly listed origins valid for that RP ID are admitted. No wildcard origins, paths, userinfo or request-derived additions. Use one origin per ceremony and bind it to that ceremony.
- LAN IP access, a LAN hostname and an unrelated Tailscale hostname do not automatically share credentials. The settings page shows the configured login URL. A stable RP ID can support multiple valid, explicitly allowed origins, including different ports. See [RP ID and origin rules](https://www.w3.org/TR/webauthn-3/#rp-id).
- An explicit `http://localhost` development configuration is permitted where the browser supports it. Do not create an insecure remote development bypass or promise WebAuthn over arbitrary IP addresses.
- Require configured owner authentication, a readable Vault and valid relying-party settings before enrollment. Permit enrollment while WebAuthn is off, so the first credential can be registered and verified before activation. Enabling WebAuthn requires at least one validated credential. Invalid WebAuthn configuration must never open owner setup, disable authentication or allow a password-only login that the selected mode would reject.
- RP changes never silently rewrite stored credentials. Require step-up and an explicit reset/re-enrollment flow with a tested fallback permitted by the selected mode. In `second_factor` mode without TOTP, first enroll TOTP or explicitly disable WebAuthn through the protected settings handler before changing the RP ID; do not create an automatic password-only fallback. Origin-only changes retain credentials only when valid for the unchanged RP ID. Cancel pending ceremonies on any policy change.
- Enable/disable, mode and RP/origin changes use dedicated session-and-step-up-protected auth settings handlers. Generic config writes, config import and bearer-only requests must not bypass this policy. Preserve legitimate offline operator configuration and the existing authenticated backup import boundary.

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

Add a bounded, server-owned in-memory ceremony store on the server lifecycle. Store library session data, a random transaction ID, the originating browser binding, operation, mode, RP/origin and authentication-configuration generation. Registration additionally binds the current owner session and its successful credential step-up. In `second_factor` mode, login begin verifies the password through the shared password/backoff path before creating a pending ceremony. Store only the successful first-factor proof bound to this ceremony, never the supplied password; this proof is not a login session or bearer token. Password, TOTP and mode changes invalidate it.

- Generate challenges with server-side cryptographic randomness. Use a two-minute lifetime and small per-browser/global limits, with expiry cleanup and shutdown cancellation.
- Bind unauthenticated login ceremonies to a separate opaque, HttpOnly, SameSite=Strict browser cookie; use Secure on trusted HTTPS. Authenticated registration also binds its originating session. A transaction ID alone grants nothing.
- Consume transactions atomically on finish, including failed finishes. Expired, wrong-browser, wrong-operation, cross-origin, configuration-stale and replayed responses fail. A fresh attempt obtains a fresh challenge.
- Require same-origin JSON POSTs with exact origin validation, bounded request bodies and non-cacheable responses on both public login ceremony endpoints. Verify client-data origin, RP binding, signature, expected owner/credential, user presence and required user verification through the library.
- Reuse the existing IP/account verification backoff and concurrency policy without introducing a global owner lockout. Rate-limit begin requests as well as verification; browser cancellation without an assertion is not an invalid-credential guess.
- Do not log assertions, challenges, raw credential IDs, cookies or authenticator details. Use fixed error codes and a scrubbed event category; retain accessible browser messages.

### Proposed HTTP surface

| Method and route | Protection and result |
| --- | --- |
| `POST /auth/webauthn/login/begin` | Exact public exemption; origin, availability and rate checks; additionally verifies the password in `second_factor` mode; returns options and a browser-bound transaction. |
| `POST /auth/webauthn/login/finish` | Exact public exemption; single-use transaction, mode-matched first-factor proof when required, and full WebAuthn verification; persists state and only then issues the existing AuraGo session cookie. |
| `GET /api/auth/webauthn/credentials` | Browser owner session; returns labels and administrative metadata, not raw Vault contents. |
| `POST /api/auth/webauthn/register/begin` | Browser owner session plus current password and active TOTP; returns enrollment options. |
| `POST /api/auth/webauthn/register/finish` | Same valid owner session and the step-up-bound enrollment transaction; publishes only a validated credential. |
| `PATCH /api/auth/webauthn/credentials/{id}` | Same-origin browser owner session plus step-up; rename a registered credential. |
| `DELETE /api/auth/webauthn/credentials/{id}` | Same-origin browser owner session plus step-up; prevents future authentication using that credential and cancels pending transactions. |
| `PUT /api/auth/webauthn/settings` | Same-origin browser owner session plus step-up; validates and serializes enablement, mode and relying-party policy. |

Only the two public login routes join `authBypassExactPaths`. They must remain blocked during the missing-password bootstrap lockdown, and must not enter `noPasswordExactPaths`. No prefix-wide `/auth/webauthn` exemption. Public auth status exposes only availability, the effective mode and whether password/TOTP fallback is usable; never labels or credential counts. The mode is descriptive, not client-controlled authority.

Both modes reuse `SetSessionCookie`, `sanitizeRedirectTarget`, the authenticated-session wait in the browser and existing logout/activity behavior. Enforce the mode before every login cookie issuance, including `/auth/login`, the TUI's legacy request and embedded recovery; sharing cookie helpers alone does not enforce two factors. Keep password/TOTP step-up unchanged for password, TOTP and passkey administration in the first version. A stolen unlocked browser session therefore cannot enroll a replacement key by itself.

For missing full UI resources, include only the minimal shared WebAuthn adapter/interaction in the embedded recovery page, within its existing size budget. Serve its ceremony routes in recovery mode as well. Do not embed the full UI, depend on the missing external bundle or create an emergency password-only bypass. Native clients without WebAuthn can use password/TOTP when enrolled; otherwise they direct the owner to browser login. No new native biometric integration is required.

Credential removal revokes subsequent uses of that key, not sessions that have already been issued. Existing sessions retain the current logout/revocation/expiry behavior. State this in the removal dialog; do not claim that credential removal signs a stolen browser out. In `second_factor` mode without TOTP, reject removal of the final credential until another key or TOTP is verified, or WebAuthn is explicitly disabled through step-up. Similarly reject TOTP removal if it would eliminate the last usable second-factor method. Global or credential-linked session revocation is a separate change if required.

## Browser experience

- In `passkey` mode, show "Sign in with a passkey or security key" beside the existing password path. In `second_factor` mode, collect the password, then show "Continue with a passkey or security key"; offer password/TOTP as the alternative only when TOTP is enrolled. The browser handles the authenticator chooser and any supported phone-assisted flow.
- Check `window.isSecureContext`, `PublicKeyCredential` and `navigator.credentials` before offering the action. Platform-biometric availability must not hide external-key support. Unsupported contexts show a clear reason and only the fallback methods permitted by the selected mode.
- Use native browser APIs with a small, tested base64url/ArrayBuffer adapter; feature-detect newer JSON conversion APIs. Keep current asset packaging, entry-page layout, theme behavior and content security policy.
- Show a pending state and allow cancellation. Handle cancellation, timeout, missing credential, unsupported authenticator, server rejection and lost network responses distinctly without automatically starting another ceremony. Prevent concurrent button submissions and cancel/ignore work on navigation.
- Under existing security settings, add explicit enablement, a two-option login-mode dropdown, RP/origin validation, a list of named credentials with last-use information, add/rename/remove actions, and a local capability diagnostic. Explain TOTP fallback and native-client limitations for the selected mode. Enrollment success is the connection test; do not prompt for biometric access during passive status reads.
- Preserve the normal config draft until the protected settings save succeeds. Use existing modals and accessible focus/status handling. Translate login, settings, help and errors in all 16 locales. Keep config search mapped to the actual new auth paths.
- Recommend registering a second authenticator and optionally enrolling TOTP for recovery/native clients before relying on `second_factor` mode. After protected activation or a mode change, test a complete login in a second browser tab while keeping the original owner session available for a step-up-protected correction. Do not enable public login ceremonies while the feature is off just to perform this check. The password cannot be removed in this version. No new unauthenticated reset endpoint or SMS recovery is introduced.

## Execution sequence and acceptance

1. **Configuration, trust and library**
   - [ ] Implement both configured modes and the password/WebAuthn/TOTP acceptance table above, including missing/invalid mode handling.
   - [ ] Add the pinned dependency, config defaults/template/validation and protected settings handling.
   - [ ] Test HTTPS/origin/RP validation, trusted versus forged proxy headers and rejection of generic auth-policy bypasses.
2. **Registry and ceremony lifecycle**
   - [ ] Implement focused server-owned registry and ceremony files; reuse Vault primitives.
   - [ ] Test restart/backup stability, duplicate/limit handling, corrupt records, Vault errors, concurrent registration/login/removal, stale generations and single-use expiry.
3. **Enrollment and login endpoints**
   - [ ] Wire the routes in the normal server, exact auth exemption table and relevant asset-recovery boundaries.
   - [ ] Test successful registration/assertion in both modes, required UV, wrong signatures/RP/origin/user/credential, cross-browser replay, disabled/removed keys, throttling and locked Vault/bootstrap behavior.
   - [ ] Test missing/wrong passwords, a finish without a verified first factor, mode spoofing, mid-flow password/mode changes, no cookie after the first factor alone, and every password/TOTP fallback combination. Exercise `/auth/login`, recovery and the TUI request directly to prove that the UI cannot bypass the mode.
   - [ ] Preserve sensitive-change step-up, logout, activity renewal and redirects; test last-factor removal guards and explicit browser-required results for unsupported native login.
4. **Login and settings UI**
   - [ ] Implement the native credential flow and small encoding helper, credential management and all locales.
   - [ ] Add browser regression coverage for both login modes and the settings dropdown, using a virtual WebAuthn authenticator with and without UV, plus cancellation, missing API, insecure context and failed persistence paths.
   - [ ] Verify session confirmation, focus/keyboard behavior, responsive layout and config save failures.
5. **Documentation, packaging and real devices**
   - [ ] Update owning server/UI/security contracts, operator security/HTTPS documentation and resource-packaging guidance as needed.
   - [ ] Run focused tests first, then appropriate package checks and the UI bundle integrity check. Build the version-bound asset set and matching binary for deployment validation; a package archive alone does not activate the new login.
   - [ ] Manually verify Windows Hello with Edge/Chrome, Apple platform authentication with Safari, Android with Chrome and a UV-capable USB/NFC FIDO2 key on supported clients.
   - [ ] Exercise both modes over direct trusted HTTPS, a configured trusted reverse proxy and allowed localhost development; test wrong hostname/untrusted certificate and missing-resource recovery with and without TOTP.
   - [ ] Register two authenticators, log out, authenticate with each in both modes, remove one and confirm its next assertion fails while the permitted fallback still works. Verify that disabling TOTP cannot introduce password-only access in `second_factor` mode.

Representative checks after implementation, using the repository cache wrapper if ignored scratch files are present:

```powershell
./scripts/go-test-with-cache.ps1 ./internal/config ./internal/security ./internal/server ./ui ./internal/webassets ./cmd/assetpack
node scripts/build-ui-bundles.js --check
git diff --check
```

Use focused `WebAuthn|AuthStepUp|AuthSession|AuthBypass|AuthLockdown|ExternalAssetsRecovery` test selections while iterating. Run dependency vulnerability checks and the CGO-free build with the pinned production compiler. Passing virtual-authenticator tests is not evidence that a physical key, platform biometrics or a deployed proxy works; record those acceptance results separately.

## Deliberate limits

No multi-user redesign, separate biometric database, browser framework, identity-provider sidecar, mandatory cloud account, automatic attestation service, passkey-only recovery scheme, U2F compatibility mode or native TUI biometric flow. These are not needed for the selected scope. Where password/TOTP remains an allowed fallback, the deployment as a whole retains that alternative's phishing exposure even though the WebAuthn ceremony is origin-bound. Without TOTP in `second_factor` mode, possession of the password alone grants no access, including through legacy clients and recovery.
