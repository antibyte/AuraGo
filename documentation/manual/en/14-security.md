# Chapter 14: Security

This chapter covers all security aspects of AuraGo – from the encrypted vault to two-factor authentication. These settings are essential for production environments.

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

> 💡 **Tip:** The vault is stored in `data/vault.dat`. The master password (64 hex characters) is read from the `AURAGO_MASTER_KEY` environment variable.

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

## Web UI Authentication

AuraGo supports password protection with optional TOTP two-factor authentication.

### Password Setup

**Via Web UI:**
1. Go to Configuration → Auth
2. Enable "Authentication"
3. Enter password
4. Save and restart

**Via CLI (for hashing):**
```bash
# Generate bcrypt hash (cost 10)
python3 -c "import bcrypt; print(bcrypt.hashpw(b'your-password', bcrypt.gensalt(10)).decode())"
```

**Config:**
```yaml
auth:
  enabled: true
  password_hash: "$2a$10$..."  # bcrypt hash
  session_timeout_hours: 24
```

### Two-Factor Authentication (2FA)

**Setup:**
1. Enable TOTP in config:
```yaml
auth:
  totp_enabled: true
  totp_secret: ""  # Will be generated on first start
```

2. Start AuraGo – secret will be generated
3. Check logs or use API to get QR code
4. Scan with authenticator app (Google Authenticator, Authy, Aegis)
5. Enter 6-digit code to verify

> ⚠️ **Warning:** Store the TOTP secret safely! If you lose your authenticator app, you'll need the secret to recover access.

## Danger Zone – Capability Gates

The Danger Zone allows granular control over what the agent can do.

### Tool Capabilities

```yaml
tools:
  memory:
    enabled: true
    read_only: false
  execute_shell:
    enabled: false      # Disable shell execution
  docker:
    enabled: true
    read_only: true     # Only view, don't modify
```

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

## File Locks and Instance Prevention

AuraGo uses file locks to prevent multiple instances from running simultaneously:

| Lock File | Purpose |
|-----------|---------|
| `data/vault.lock` | Prevents concurrent vault access |
| `lifeboat.lock` | Prevents update conflicts |
| `agent.lock` | Prevents multiple agent instances |

**If stuck:**
```bash
# Remove stale locks
rm data/*.lock 2>/dev/null
```

## Rate Limiting

Protects against brute force and overload:

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
mv data/secrets.vault data/secrets.vault.backup.$(date +%s)

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

> 🔍 **Deep Dive: Cryptography**
> 
> The vault uses AES-256-GCM:
> - **AES-256**: Symmetric encryption with 256-bit key
> - **GCM**: Galois/Counter Mode provides authenticated encryption
> - **AEAD**: Authenticated Encryption with Associated Data prevents tampering
> - **Key derivation**: Master key is hashed with SHA-256 for subkeys
> - **Random nonces**: Each encryption uses a unique 96-bit nonce

---

> 💡 **Tip:** Security is a process, not a product. Regularly review your configuration and update credentials.
