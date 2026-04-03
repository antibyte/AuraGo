# ssh_key_manager — SSH Key Generation and Management

Manage SSH keys securely for authentication with external devices.

## Operations

| Operation | Description | Required Params |
|-----------|-------------|-----------------|
| `generate` | Generate a new SSH key pair (Ed25519/RSA) | `key_name` (optional: `type`, `comment`) |
| `list` | List available SSH keys in the vault/system | (none) |
| `get_public`| Retrieve the public key string for a specific key | `key_name` |
| `delete` | Delete an SSH key pair safely | `key_name` |

## Key Behaviors

- Generates strong, modern keys (Ed25519 by default).
- Securely stores private keys in the encrypted Vault (never exposed to plain text logs).
- Only exposes the public key via `get_public` to be copied to authorized_keys files on targets.

## Examples

```
# Generate a new key for a specific server
ssh_key_manager(operation="generate", key_name="proxmox-auto", type="ed25519", comment="agent@aurago")

# List all keys
ssh_key_manager(operation="list")

# Get the public key to append to authorized_keys
ssh_key_manager(operation="get_public", key_name="proxmox-auto")
```

## Tips
- Use this when setting up new SSH automation, rather than trying to spawn `ssh-keygen` via shell.
- The private key is never returned by these endpoints to protect the credentials.