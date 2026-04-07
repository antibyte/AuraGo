# Firewall (`firewall`, `firewall_rules`, `iptables`)

Manage and inspect local Linux firewall rules using iptables or ufw. Supports read-only inspection and active firewall guard mode that monitors for changes.

## Operations

| Operation | Description |
|-----------|-------------|
| `get_rules` | Retrieve current active firewall rules |
| `modify_rule` | Add, remove, or modify firewall rules (requires sudo) |

## Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `operation` | string | yes | `get_rules` or `modify_rule` |
| `command` | string | for modify_rule | Firewall command: `iptables ...` or `ufw ...` |

## Examples

**Get current firewall rules:**
```json
{"action": "firewall", "operation": "get_rules"}
```

**Allow incoming SSH (iptables):**
```json
{"action": "firewall", "operation": "modify_rule", "command": "iptables -A INPUT -p tcp --dport 22 -j ACCEPT"}
```

**Allow incoming HTTPS (ufw):**
```json
{"action": "firewall", "operation": "modify_rule", "command": "ufw allow 443/tcp"}
```

**Block specific IP with iptables:**
```json
{"action": "firewall", "operation": "modify_rule", "command": "iptables -A INPUT -s 192.168.1.100 -j DROP"}
```

**Delete a rule (list rules first, then delete by specification):**
```json
{"action": "firewall", "operation": "modify_rule", "command": "iptables -D INPUT -p tcp --dport 80 -j ACCEPT"}
```

**Enable ufw:**
```json
{"action": "firewall", "operation": "modify_rule", "command": "ufw enable"}
```

## Configuration

```yaml
firewall:
  enabled: true
  mode: "readonly"  # "readonly" = inspect only, "guard" = monitor for changes
  poll_interval_seconds: 60  # How often to check for changes in guard mode
```

## Requirements

- **Linux only**: This tool only works on Linux systems with iptables or ufw installed
- **Root access**: Modification commands require either:
  - Running as root directly
  - NOPASSWD sudo configured for the aurago user
  - sudo password stored in vault under `sudo_password`
- **Docker restrictions**: In Docker containers, firewall modification may be blocked due to `--no-new-privileges` flag

## Security Notes

- Modification commands are blocked when `firewall.mode: "readonly"` is set
- Commands must start with `iptables` or `ufw` (command injection protection)
- The agent should only modify firewall rules when explicitly requested by the user
- In guard mode, the agent will be woken up if firewall rules change

## Notes

- **iptables vs ufw**: iptables is the lower-level Linux kernel netfilter interface; ufw (Uncomplicated Firewall) is a simpler frontend on top of iptables
- **List rules first**: Use `get_rules` to see current rules before modifying
- **Rule persistence**: Changes made with iptables/ufw commands are not persistent across reboots by default; use `iptables-save` or enable ufw boot startup to persist
