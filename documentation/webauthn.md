# Passkeys and FIDO2 security keys

AuraGo can authenticate its single owner with WebAuthn. Your browser and
authenticator verify you locally using biometrics or a device PIN. AuraGo
receives a signed proof and stores the public credential in the encrypted
Vault; it never receives biometric templates or a private key.

WebAuthn defaults off. Configure it under **Config → Web Configuration &
Security → Passkeys and security keys**. The password remains necessary for
sensitive account changes.

## Choose a login mode

| Mode | Browser login | Existing password login, recovery and native TUI |
| --- | --- | --- |
| Passkey sign-in (`passkey`) | A verified passkey or FIDO2 key completes login. | Password plus the active TOTP code; password alone if TOTP is off. |
| Password + WebAuthn (`second_factor`) | Correct password, then a verified passkey or FIDO2 key. | Password plus an enrolled TOTP code. Without TOTP, use the browser; a password alone is rejected. |

User verification is required for enrollment and login. A touch-only U2F key
without user verification is insufficient. The authenticator chooses a
supported biometric or PIN method; AuraGo cannot force a fingerprint instead
of a PIN. Phone-assisted and synced passkeys depend on browser/platform support.
No server USB, Bluetooth or biometric service is necessary.

## Configure the login address

Use a stable DNS name and trusted HTTPS. Clicking through a self-signed
certificate warning does not establish WebAuthn capability. See
[HTTPS configuration](https.md) for certificate and reverse-proxy setup.

For `https://aurago.example.net:8443`:

```yaml
auth:
  webauthn:
    enabled: false
    login_mode: second_factor
    rp_id: aurago.example.net
    origins:
      - https://aurago.example.net:8443
```

The RP ID is a lowercase domain without a scheme, port or trailing dot.
Each origin contains a scheme, hostname and optional port, with no path,
trailing slash, query, fragment, user information or wildcard. An origin must
use the RP domain or one of its subdomains. Configure up to eight origins.
Credentials are scoped to the RP ID. An unrelated LAN and Tailscale hostname
do not share credentials. Never infer allowed hosts from proxy headers.

For local development only, explicitly configure RP ID `localhost` and an
origin such as `http://localhost:8088`. Remote HTTP and IP-based RP IDs are
rejected. The settings page checks local browser capability without starting
a biometric prompt. Keep a trusted HTTPS route when changing HTTP/TLS settings.

## Enroll and activate

1. Sign in with the existing owner password and active TOTP code.
2. Keep WebAuthn disabled, select the mode and enter the RP ID and origins.
   Enter your current password and active TOTP code, then use the section's
   **Save** button. Its inputs are independent of the general Config save.
3. Enter a key name and current credentials, then choose **Add a passkey or
   security key**. Complete the browser prompt. Only a verified registration
   enters the key list. Register a second authenticator for recovery.
4. Enable WebAuthn using the section's **Save** button and fresh step-up
   credentials. Activation requires at least one enrolled key for that RP ID.
5. Test a complete login in another tab while retaining the original owner
   session. In second-factor mode, enroll TOTP if you also need native TUI
   login or a separate recovery method.

Up to 16 credentials are supported, with names of 1–64 characters. Rename and
remove actions require the current password and active TOTP code. Removing a
key blocks its subsequent logins and invalidates pending ceremonies; sessions
already issued remain signed in until ordinary logout, revocation or expiry.
Removing the last key in second-factor mode is blocked without TOTP. TOTP
removal is blocked when no matching WebAuthn key remains.

To change RP ID, first explicitly disable WebAuthn through this protected
section, then save the new domain with the reset confirmation. Reset removes
the old registry and requires enrollment again. Origin-only changes for the
same RP retain keys. Generic Config/setup writes and bearer-only requests
cannot change WebAuthn policy. Offline operator edits and the existing
authenticated encrypted-backup import retain their administrative authority.

## Recovery, backups and failures

The embedded recovery page supports both modes even when the full resource
set is missing. It includes the same small WebAuthn client; its authentication
does not depend on external scripts. Repairing UI assets does not relax the
selected login mode. Unsupported browsers show only the fallback allowed by
that mode. There is no unauthenticated key-reset endpoint.

Encrypted Vault backups include the versioned credential registry, stable
owner handle, RP binding, counter and backup flags. Restoring an older backup
can restore keys you previously removed: review the key list after restoration.
Corrupt or unknown registry versions are rejected without silently erasing them.
The registry is unavailable to agent secret access, Python export and generic
secret prompts. Configuration YAML and public auth status contain no key list.

Requests have a two-minute expiry and are single-use, browser-bound and
origin-bound. Account changes, registry writes and config/Vault replacement
invalidate pending work. Concurrent logins can require one fresh retry after
the first succeeds. Nothing retries assertions or writes automatically.

A cancelled browser prompt does not create a session. If a network response
is lost after a write, the UI reports an uncertain result: check the saved key
list/settings or session before retrying. A Vault write failure issues no
login cookie. A positive authenticator counter regression rejects that login
and leaves the key intact; zero-counter synced passkeys remain supported.

## Verification and delivery

Focused checks:

```powershell
go test ./internal/config ./internal/server -run 'WebAuthn|AuthStepUp|AuthSession|AuthBypass|AuthLockdown|ExternalAssetsRecovery'
go test ./internal/security ./internal/vaultprompt ./internal/webassets ./cmd/assetpack ./ui
node scripts/webauthn-client.test.mjs
node scripts/build-webauthn-locales.mjs --check
node scripts/build-ui-bundles.js --check
$env:AURAGO_RUN_BROWSER_SMOKE='1'
go test ./internal/server -run TestWebAuthnBrowserLoginAndRecovery -count=1
```

Use the repository cache wrapper when scratch packages are present. The browser
test uses an isolated virtual CTAP2 authenticator and synthetic account data.
It verifies enrollment, both login modes, recovery, cancellation, capability
failure and preservation of unrelated configuration drafts. Backend tests cover
signature/UV/counter checks, stale transactions, origin/bootstrap boundaries,
last-factor guards, backup/restart stability and failed Vault persistence.

Build the [version-bound UI and matching binary](web-assets.md) together. Local
tests and a matching resource check do not activate a running installation.
Physical Windows Hello, Apple/Android authentication and USB/NFC keys still
need acceptance on the intended device and trusted HTTPS/proxy deployment.
