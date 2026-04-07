# Exit Lifeboat (`exit_lifeboat`) — Maintenance Only

Signal that maintenance is complete and attempt to return control to the main supervisor.

## Parameters

None — this tool takes no arguments.

## Example

```json
{"action": "exit_lifeboat"}
```

## Notes

- **Maintenance mode only**: This tool is only available during maintenance mode
- **Purpose**: Signals completion of maintenance tasks and returns control to the main supervisor