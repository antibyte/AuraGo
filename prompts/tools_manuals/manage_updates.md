# Manage Updates (`manage_updates`)

Check for and install AuraGo software updates from GitHub. This is a **conditional tool** only enabled when `allow_self_update: true`.

## Operations

| Operation | Description |
|-----------|-------------|
| `check` | Fetch latest state from GitHub and compare with local version |
| `install` | Pull latest code, merge configuration, and restart the system |

## Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `operation` | string | yes | One of: check, install |

## Examples

**Check for updates:**
```json
{"action": "manage_updates", "operation": "check"}
```

**Install updates:**
```json
{"action": "manage_updates", "operation": "install"}
```

## Notes

- **Check usage**: Use during daily maintenance or when user asks if an update is available
- **Install requires permission**: ONLY call install after receiving explicit user permission
- **Service restart**: The install operation will restart the AuraGo service — temporary disconnection occurs
- **Conditional**: This tool is only available when `allow_self_update: true` in the Danger Zone settings
