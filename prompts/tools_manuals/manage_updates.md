---
description: "Check for and install AuraGo software updates from GitHub."
conditions: ["allow_self_update"]
---

# Tool Guide: manage_updates

Use this tool to manage AuraGo software updates. It interacts with the git repository to check for new commits or apply them using the internal update script.

## Operations

### check
- **Description**: Fetches the latest state from GitHub and compares it with your local version.
- **Usage**: Use this during daily maintenance or when the user asks if an update is available.
- **Output**: Returns whether an update is available, the number of commits ahead, and a brief changelog.

### install
- **Description**: Pulls the latest code, merges configuration, and restarts the system.
- **Usage**: **ONLY** call this after receiving explicit user permission to "install the update" or "perform the update".
- **Risk**: This operation will restart the AuraGo service. You will be temporarily disconnected.

## Usage Example

```json
{
  "action": "manage_updates",
  "operation": "check"
}
```

```json
{
  "action": "manage_updates",
  "operation": "install"
}
```
