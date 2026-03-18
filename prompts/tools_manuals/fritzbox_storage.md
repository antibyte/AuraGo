# Fritz!Box Storage Tool (`fritzbox_storage`)

Query and configure the Fritz!Box NAS (FRITZ!NAS) storage features: USB drives, FTP server, and DLNA/UPnP media server.

**Requires**: `fritzbox.storage.enabled: true` in config.
Write operations additionally require `fritzbox.storage.readonly: false`.

## Key Operations

| Operation | Description | Parameters |
|-----------|-------------|------------|
| `get_storage_info` | USB storage volumes, total/free capacity | — |
| `get_ftp_status` | Whether the built-in FTP server is enabled | — |
| `set_ftp` | Enable or disable the FTP server | `enabled` |
| `get_media_server_status` | Whether the DLNA/UPnP media server is enabled | — |
| `set_media_server` | Enable or disable the DLNA/UPnP media server | `enabled` |

## Examples

```json
{"action": "fritzbox_storage", "operation": "get_storage_info"}
```

```json
{"action": "fritzbox_storage", "operation": "get_ftp_status"}
```

```json
{"action": "fritzbox_storage", "operation": "set_ftp", "enabled": false}
```

```json
{"action": "fritzbox_storage", "operation": "get_media_server_status"}
```

```json
{"action": "fritzbox_storage", "operation": "set_media_server", "enabled": true}
```

## Notes

- Storage info is only available if a USB drive is attached to the Fritz!Box.
- FTP and DLNA/UPnP media server features depend on the Fritz!OS version and device model; older models may not support them.
- Disabling the FTP server will disconnect any active FTP sessions immediately.
- The DLNA media server scans attached USB storage; large libraries may cause a short delay after enabling.
