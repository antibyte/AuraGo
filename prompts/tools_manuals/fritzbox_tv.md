# Fritz!Box TV Tool (`fritzbox_tv`)

List DVB-C/DVB-T TV channels configured on a Fritz!Box with a built-in TV tuner (e.g., FRITZ!Box 6490 Cable).

**Requires**: `fritzbox.tv.enabled: true` in config.

## Key Operations

| Operation | Description | Parameters |
|-----------|-------------|------------|
| `get_channels` | List all configured TV channels (name, number, type) | — |

## Examples

```json
{"action": "fritzbox_tv", "operation": "get_channels"}
```

## Notes

- Only available on Fritz!Box models with a built-in TV tuner (e.g., FRITZ!Box 6490 Cable, 6590 Cable).
- Channels are returned as a list of objects with at minimum `name` and `channel_number`.
- This tool is read-only; channel configuration must be done via the Fritz!Box web UI.
- If the Fritz!Box model has no TV tuner, the operation will return an error from the device.
