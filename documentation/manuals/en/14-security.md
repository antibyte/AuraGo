# Chapter 14: Security

<p align="center">
  <a href="../images/manual-security.webp"><img src="../images/manual-security.webp" width="560" alt="AuraGo gopher beside a brass vault with the Danger Zone lever off"></a>
</p>

Vault closed, jail on, Danger Zone on purpose. Personality does not change that.

> ⚠️ **Critical:** AuraGo executes code on your system. Proper security configuration is not optional but mandatory.

---

## The AES-256 Vault

The vault is the heart of AuraGo's security architecture. All sensitive data – API keys, passwords, tokens – is stored encrypted here.

### Technical Details

```
┌─────────────────────────────────────────────────────────┐
│                    AES-256-GCM Vault                    │
├─────────────────────────────────────────────────────────┤
│  • Encryption: AES-256 in GCM mode                     │
│  • Key length: 256 Bit (64 hex characters)             │
│  • Nonce: 96 Bit, randomly generated per operation     │
│  • Authentication: Galois/Counter Mode (AEAD)          │
│  • File locking: Prevents parallel access              │
└─────────────────────────────────────────────────────────┘
```

### What is stored in the vault?

| Category | Examples |
|----------|----------|
| **API Keys** | OpenRouter, OpenAI, Google Workspace |
| **Credentials** | SMTP/IMAP passwords, database passwords |
| **Tokens** | OAuth refresh tokens, bot tokens |
| **Internal** | Master key derivatives, session secrets |

> 💡 **Tip:** The vault is stored in `data/vault.bin`. The master password (64 hex characters) is read from the `AURAGO_MASTER_KEY` environment variable.

### Agent Access to the Vault

> 🔒 **Important Security Note:**
>
> The agent **never** has direct access to secrets stored in the vault for internal integrations and tools. It cannot read or retrieve these secrets.
>
> **Exception:** Secrets that the agent **created itself** (e.g., via chat or API) can be retrieved and managed by it at any time.
>
> The application (not the agent) loads vault secrets at runtime and securely injects them into the appropriate tools without the agent ever seeing the plaintext values.

### Initializing the vault

```bash
# Generate master key (only once)
openssl rand -hex 32
# Output: a1b2c3d4e5f6... (64 characters)

# Set as environment variable
export AURAGO_MASTER_KEY="your-generated-key"

# Start AuraGo – vault will be created automatically
./aurago
```

## LLM Guardian

An optional second model looks at tool calls and untrusted content before dispatch. Personality never bypasses it.

**Config → Security → LLM Guardian.** Use a dedicated provider ID, not the main chat model. `fail_safe: block` is the closed default; MeshCore and other untrusted inlets still fail closed even if a global allow is set.

```yaml
llm_guardian:
  enabled: false
  provider: ""
  model: ""
  default_level: medium   # low | medium | high | strict
  fail_safe: block        # block | quarantine | allow
  scan_documents: false   # opt-in scan for incoming documents and webhooks
  scan_emails: false      # opt-in scan for fetched and relayed email
```

Document and email scans are opt-in to control cost. Once enabled, their ingress paths accept only an explicit allow; scanner errors, incomplete coverage, and size-limit overflow quarantine the original and deliver only a fixed safe notice where an authorized agent target exists. This ingress rule is independent of `fail_safe` and cannot be changed to allow by setting it to `allow` or `quarantine`.

## Sudo

`/sudopwd` stores the password in the vault. `sudo_enabled` must be on. `sudo_unrestricted` is a second gate and needs a systemd unit without `ProtectSystem=strict`. Writes outside the install directory stay denied until both are true.

The password only reaches `sudo -v`; commands run with `sudo -n` and a closed stdin. This needs sudo timestamp caching enabled (`timestamp_timeout` ≠ 0) or a NOPASSWD rule for the AuraGo user.

## Web UI Authentication

AuraGo ships a layered authentication system for the web interface.

### Overview of Auth Methods

| Method | Strength | Use case |
|--------|----------|----------|
| **Password (bcrypt)** | High | Baseline protection for the web UI |
| **TOTP 2FA** | Very high | Extra protection for external access |
| **Session cookies** | High | Stateless authentication |
| **IP rate limiting** | Medium | Brute-force protection |

### bcrypt Password Hashing

- **Algorithm:** bcrypt with cost factor 12
- **Salt:** Automatic, unique per password
- **Cost:** ~250 ms per hashing operation (protection against rainbow tables)

### TOTP (Time-based One-Time Password)

- **Standards:** RFC 6238 / RFC 4226
- **Time window:** 30 seconds
- **Tolerance:** ±1 window (90 second grace period)
- **QR code:** Compatible with Google Authenticator, Authy, Bitwarden

---

## Setting Up Password Protection

### Step 1: Initial configuration

**In the web UI:**
1. Open **Config → Server → Web Config & Login**.
2. Enable login protection, session timeout and rate limiting.
3. Set the password and optionally TOTP via `POST /api/auth/password` and `POST /api/auth/totp/setup` (or directly in the web UI).
4. Save.

### YAML Reference
```yaml
# config.yaml
auth:
    enabled: true
    session_timeout_hours: 24
    totp_enabled: false
    max_login_attempts: 5
    lockout_minutes: 15
```

The password hash, TOTP secret and session secret are stored in the **vault** (`auth_password_hash`, `auth_totp_secret`, `auth_session_secret`), not in `config.yaml`. Set them through the web UI (**Config → Server → Web Config & Login** or `/auth/login`) or the API (`POST /api/auth/password`, `/api/auth/totp/setup`).

### Step 2: Set the password in the web UI

1. Start AuraGo with `auth.enabled: true`
2. Open the web UI (`http://localhost:8088`)
3. You are redirected to the setup page automatically
4. Choose a strong password (at least 12 characters, mixed character sets)

> ⚠️ **Warning:** Without a configured password, authentication is disabled automatically to prevent a lockout!

### Hashing a password manually (CLI)

If you want to set the password directly in `config.yaml`:

```bash
# With Python
python3 -c "import bcrypt; print(bcrypt.hashpw(b'your-password', bcrypt.gensalt(12)).decode())"

# With Node.js
node -e "const bcrypt = require('bcrypt'); console.log(bcrypt.hashSync('your-password', 12))"
```

---

## Enabling Two-Factor Authentication (2FA)

### Prerequisites

- Password authentication is configured
- A TOTP app is installed (Google Authenticator, Authy, …)

### Setup

1. **Enable it in the web UI:** **Config → Server → Web Config & Login** → enable TOTP, or in the YAML reference:
   ```yaml
   auth:
       totp_enabled: true
       totp_secret: ""              # Generated automatically
   ```

2. **Restart AuraGo**

3. **Scan the QR code:**
   - Open the login page `/auth/login` and follow the TOTP setup wizard (or call `POST /api/auth/totp/setup` directly for the QR code)
   - Scan the displayed QR code with your authenticator app
   - Confirm with `POST /api/auth/totp/confirm` and the 6-digit code

### Important notes on 2FA

> ⚠️ **Save a backup code!** If you lose access to your authenticator, you cannot log in without the TOTP secret. Export the secret from the config or store it separately.

```yaml
# Backup: keep this value safe!
auth:
    totp_secret: "JBSWY3DPEHPK3PXP"    # Base32-encoded secret
```

## Danger Zone – Capability Gates

The Danger Zone allows granular control over what the agent can do.

### Agent Capability Gates

> 🖥️ **Web UI:** Open **Config → Danger Zone** for `agent.allow_*` capability gates. Per-integration **Read-only** toggles are on each integration's own Config page (e.g. **Config → Docker**, **Config → Home Assistant**).

Danger Zone toggles live under `agent.allow_*`:

### YAML Reference
```yaml
agent:
  allow_shell: false
  allow_python: false
  allow_unsafe_host_execution: false   # additionally required for Windows shell and host Python
  allow_unsandboxed_shell: false       # lets the Linux/macOS host shell run without a sandbox
  allow_filesystem_write: false
  allow_network_requests: false
  allow_remote_shell: false
  allow_self_update: false
  allow_mcp: false
  allow_package_manager: false   # also requires package_manager.enabled
  sudo_enabled: false
  sudo_unrestricted: false
```

Per-integration read-only mode uses top-level `readonly` flags, e.g.:

```yaml
docker:
  enabled: true
  readonly: true     # list/inspect only
home_assistant:
  enabled: true
  readonly: true     # get_states only
tools:
  web_scraper:
    enabled: false   # outbound page scraping into agent context; enable deliberately (replaces deprecated agent.allow_web_scraper)
```

`docker.allow_host_access` is the Docker counterpart of a Danger Zone gate: it decides whether agent `docker compose up/create/build` may use the host (binds outside the workspace, `/var/run/docker.sock`, devices, privileged mode, host namespaces). Existing configurations keep it `true`, fresh installs start with `false`; the security check warns while it is on. Without it, agent Compose also runs with a minimal environment that holds none of AuraGo's variables. Details: [Docker Integration](08-integrations.md#docker-integration).

### Integration Capabilities

| Integration | Read-Only Mode | Effect |
|-------------|---------------|--------|
| Docker | true | List containers only, no start/stop |
| Home Assistant | true | View states only, no control |
| Email | true | Read only, no sending |

### Interface

In the Web UI (Configuration → Danger Zone):
- Toggle switches for each capability
- Red indicators for dangerous features
- Confirmation dialogs for critical changes

## Agent Filesystem and Docker Isolation

Two additional default boundaries limit what the agent can reach regardless of tool permissions:

- **Workspace jail:** agent file access (filesystem and file-editor tools) is jailed to `agent_workspace`. From the working directory, `../skills` and `../tools` stay reachable; attempts such as `../../config.yaml` or `data/` fail path resolution. Defense-in-depth additionally blocks protected system paths (data directory, `config.yaml`, vault, SQLite databases, `.env`, and the master key file) case-insensitively and with symlinks resolved. Files served under `/files/` follow symlinks and junctions only while they stay inside the served directory; links leaving `agent_workspace` or a media directory are not served.
- **Docker inspect redaction:** agent `docker inspect` output redacts `AURAGO_*` environment variables and keys ending in `_PASSWORD`, `_SECRET`, `_TOKEN`, `_API_KEY`, `_ACCESS_KEY`, `_PRIVATE_KEY`, or `_MASTER_KEY`. The AuraGo app container itself (compose service `aurago`) is hidden from inspect, lifecycle, log, exec, and copy access, and the managed Garage container is invisible as well.

## Read-only vs Read-write Mode

### Comparison

| Aspect | Read-only | Read-write |
|--------|-----------|------------|
| **Security** | 🔒 Very high | 🔓 Higher risk |
| **Usefulness** | Limited | Full |
| **Typical use** | Monitoring, research | Automation, control |
| **Error message** | "Tool is in read-only mode" | – |

### Granular control

Per integration under **Config → Integrations → …** (toggle **Read-only**):

### YAML Reference
```yaml
# Example: safe configuration for monitoring
docker:
    enabled: true
    readonly: true              # Show containers, do not change them

home_assistant:
    enabled: true
    readonly: true              # Read temperature, do not control heating

email:
    enabled: true
    readonly: true              # Read mail, do not send

# The agent can still:
# - read and write files in the workspace
# - run web searches
# - execute Python code
```

---

## File Locks and Instance Prevention

AuraGo uses file locks to prevent multiple instances from running simultaneously:

| Lock File | Purpose |
|-----------|---------|
| `data/vault.lock` | Prevents concurrent vault access |
| `data/aurago.lock` | Prevents multiple AuraGo instances |
| `agent.lock` | Prevents multiple agent instances |

**If stuck:**
```bash
# Remove stale locks
rm data/*.lock 2>/dev/null
```

## Rate Limiting

Protects against brute force and overload.

> 🖥️ **Web UI:** **Config → Web Config & Login** → **Max Login Attempts** and **Lockout Minutes**.

### YAML Reference
```yaml
# Config (in auth section)
auth:
  max_login_attempts: 5
  lockout_minutes: 15
```

**Effects:**
- After 5 failed logins: 15-minute lockout
- API rate limiting per IP
- Webhook rate limiting configurable

> 💡 **Tip:** Behind a reverse proxy (nginx, Traefik), `X-Forwarded-*` headers count only when `server.https.behind_proxy: true` is set **and** the immediate peer is listed in `server.https.trusted_proxy_cidrs`. Without this, client-supplied forwarding claims are discarded. Details below under [Ingress trust](#ingress-trust-and-public-boundaries).

---

## Ingress Trust and Public Boundaries

AuraGo treats inbound headers, embed access, and WebSocket origins explicitly instead of trusting them:

- **Forwarding headers:** `X-Forwarded-Host`, `X-Forwarded-Proto`, and the client IP count only when `server.https.behind_proxy: true` **and** the immediate TCP peer is in `server.https.trusted_proxy_cidrs` (a list of proxy IPs/CIDRs). Everything else is client-controlled and ignored.

```yaml
server:
  https:
    behind_proxy: true
    trusted_proxy_cidrs: ["192.168.1.10", "10.0.0.0/24"]
```

- **Auth-off exception:** Starting AuraGo without login (`auth.enabled: false`) while any remote listener is configured requires an explicit `auth.allow_unauthenticated_remote: true` at startup and config save. The exception never opens `/speech-lab/` — that proxy requires a real AuraGo session regardless of the auth setting. Configured inbound webhook and integration authentication stays active as well.
- **Desktop embed tickets:** Media and file references in desktop embeds go through `/desktop-ticket/<ticket>/...`. The short-lived ticket carries credentials in a path segment, is stripped before routing and access logging, and is scoped to desktop file paths (`/files/desktop/`) or the printer camera streams; query tokens are rejected.
- **Speech Lab browser:** `/speech-lab/` requires an AuraGo session; writes and the WebSocket Origin must be same-origin, and the backend must be a configuration-owned private address. AuraGo credentials are stripped by the proxy; in external mode only the server-side `speech_lab.browser_backend_url` points to the s2s web backend — never a legacy URL in the browser.
- **CSP:** The main UI at `/` carries a report-only Content Security Policy without `unsafe-inline`; desktop apps keep their own stricter policy.

---

## Outbound Requests and Host Execution

### SSRF protection for outbound HTTP

`api_request`, the web scraper, and the LLM clients use an SSRF-protected client: DNS resolution selects a public IP, the dial is pinned to that IP, every redirect is revalidated (at most 10, then abort), and system proxies are disabled. Loopback, private, CGNAT, and metadata addresses are blocked; `AURAGO_SSRF_ALLOW_LOOPBACK` remains an explicit development escape hatch.

### Network MCP servers

An MCP server with network access rejects private destination addresses unless that server grants `allow_private_network: true`; DNS is pinned per connection, and cross-origin redirects and SSE message endpoints are rejected.

### Browser automation with egress sidecar

Browser automation requires a nonempty sidecar token and an attested sidecar (`egress-v1`). The managed sidecar joins only a Docker-internal network. A separate `browser-egress` container joins that network and the outbound network, accepts proxy clients only from the internal subnet, and carries the route out. Chromium uses `http://browser-egress:7332` and disables non-proxied WebRTC UDP. `cloak_proxy` stays empty. Navigation, resources, and WebSockets follow the same egress policy.

```yaml
browser_automation:
  cloak_proxy: ""                  # must stay empty; the sidecar owns the filter
  egress_network: aurago-app       # Docker network, must be internal: true
  outbound_network: bridge         # different Docker network with a route out
  allowed_private_origins: []      # exact home-lab origin URLs allowed by the policy
```

Compose profile `browser-automation` starts `browser-egress` beside the sidecar. The sidecar has no route of its own.

### Host execution without isolation

Windows shell and every host Python path — including Agent Skill scripts and background Python jobs — need `agent.allow_unsafe_host_execution: true` in addition to the existing tool gate. Every allowed execution emits an audit warning in the log. The Linux/macOS host shell without an effective sandbox needs `agent.allow_unsandboxed_shell: true` or `agent.allow_unsafe_host_execution: true`; updates via `update.sh` and Docker write `allow_unsandboxed_shell` (`true` for configurations that already had the shell enabled, `false` otherwise). Configurations that never run config-merger and never had the key written are grandfathered even if the shell is enabled later; write `agent.allow_unsandboxed_shell` explicitly to make the decision durable. Desktop Notes protection and file jails apply regardless.

```yaml
agent:
  allow_unsafe_host_execution: false   # unsafe: set only after a deliberate decision
```

> ⚠️ Both flags (`auth.allow_unauthenticated_remote`, `agent.allow_unsafe_host_execution`) follow the danger-zone philosophy: default `false`, visible in the log, and set only when you accept the consequence.

---

## Security Best Practices

### Production Checklist

- [ ] Master key backed up securely (offline)
- [ ] Web UI authentication enabled
- [ ] 2FA activated for admin accounts
- [ ] Unnecessary tools disabled
- [ ] Read-only mode for risky integrations
- [ ] Firewall configured (only localhost or VPN)
- [ ] Regular backups of `data/` directory
- [ ] Logs monitored for suspicious activity

### Network Security

```
Internet
   │
   ▼
[Firewall] ← Block direct access
   │
   ▼
[Reverse Proxy] ← SSL/TLS termination
   │ (Authentication here)
   ▼
[AuraGo] ← Bind to localhost only
   127.0.0.1:8088
```

**Recommended setup:**
- AuraGo binds to `127.0.0.1`
- Nginx/Traefik as reverse proxy
- VPN (WireGuard/Tailscale) for remote access
- Or: Use built-in auth + SSL

## Vault Management

### Status Check

```
Du: Check vault status
Agent: 🔐 Vault Status:
   Status: locked/unlocked
   Items: 12 secrets
   Last access: 2024-01-15 14:30
```

### Reset Vault

> ⚠️ **All secrets will be lost!**

```bash
# Backup old vault
mv data/vault.bin data/vault.bin.backup.$(date +%s)

# Generate new key
export AURAGO_MASTER_KEY=$(openssl rand -hex 32)

# Restart – creates empty vault
./aurago
```

### Rotate Master Key

1. Export all secrets (via chat: "Show me all vault entries")
2. Create new vault (see above)
3. Re-import secrets

## Incident Response

### Compromised API Key

1. Revoke key at provider (OpenRouter, etc.)
2. Generate new key
3. Update in AuraGo config
4. Restart AuraGo

### Forgotten Password

**With TOTP still working:**
- Use TOTP to authenticate
- Change password in settings

**Without TOTP:**
- Manual vault reset required
- All secrets lost

### Lost TOTP Device

1. Find backup of TOTP secret (shown during setup)
2. Restore to new authenticator app
3. Or: Disable TOTP via config file (requires file access)

---

## 🔍 Deep Dive: Cryptographic Implementation

### AES-256-GCM Details

```go
// Simplified view
block, _ := aes.NewCipher(key)      // 256-bit key
gcm, _ := cipher.NewGCM(block)      // GCM mode
nonce := make([]byte, gcm.NonceSize()) // 96-bit nonce
io.ReadFull(rand.Reader, nonce)     // Cryptographically secure

ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
```

**Security properties:**
- **Confidentiality:** AES-256 encryption
- **Integrity:** GCM authentication (AEAD)
- **Non-replay:** Unique nonce per operation
- **Forward secrecy:** No implicit key derivation

### Session Cookie Security

```
Cookie: aurago_session=<payload>.<hmac>

payload = base64("user|<expiry_unix>")
hmac = SHA256(secret + payload)
```

- **HttpOnly:** Prevents XSS access to the cookie
- **SameSite=Strict:** CSRF protection
- **HMAC-SHA256:** Integrity protection
- **Expiry:** Configurable (default: 24 hours)

---

## Public Access

| Protection | Recommendation |
|------------|----------------|
| Login | Enable auth before anything stops being localhost-only |
| TOTP | 2FA for every internet-facing deployment |
| Proxy trust | Set `behind_proxy` + `trusted_proxy_cidrs`, otherwise forwarding headers are ignored |
| Security Proxy | Managed Caddy for TLS termination, rate limiting, IP filtering and Basic Auth |
| Tunnel / VPN | Cloudflare Tunnel or Tailscale instead of port forwarding |
| Webhooks | Token or HMAC, narrow scopes, rate limits |
| Danger Zone | Off until you actually need the feature |
| Unsafe exceptions | Keep `allow_unauthenticated_remote`, `allow_unsafe_host_execution` and `allow_unsandboxed_shell` at `false` |
| Backups | Encrypt `.ago` backups and keep the passphrase out of the repository |

If compromise is suspected: rotate keys with the provider, invalidate webhook tokens, read the logs, and regenerate vault secrets.

## Summary

| Feature | Configuration | Recommended setting |
|---------|---------------|---------------------|
| Vault encryption | `AURAGO_MASTER_KEY` env | 64 random hex characters |
| Password auth | `auth.enabled` | `true` (production) |
| 2FA | `auth.totp_enabled` | `true` (external access) |
| Session timeout | `auth.session_timeout_hours` | `24` |
| Rate limiting | `auth.max_login_attempts` | `5` |
| Proxy trust | `server.https.trusted_proxy_cidrs` | Explicit proxy IPs/CIDRs |
| SSRF protection | `internal/security/ssrf.go` | Always on, no gate |
| Host execution | `agent.allow_unsafe_host_execution` | `false` |
| Host shell without sandbox | `agent.allow_unsandboxed_shell` | `false` |
| Docker gate | `docker.readonly` | `true` (monitoring) |
| Vault gate | `tools.secrets_vault.readonly` | `true` |

---

> 💡 **Next steps:**
> - **[Co-agents](15-coagents.md)** – Use parallel agents safely
> - **[Configuration](07-configuration.md)** – Fine-tune every parameter
> - **[Docker deployment](../../docker_installation.md)** – Isolated production environment

---

**Previous Chapter:** [Chapter 13: Dashboard](./13-dashboard.md)
**Next Chapter:** [Chapter 15: Co-agents](./15-coagents.md)
