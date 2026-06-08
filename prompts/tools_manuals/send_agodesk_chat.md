## Tool: AgoDesk Chat (`send_agodesk_chat`)

Send a proactive text message to a connected AgoChat/AgoDesk desktop companion.

Use this when an autonomous run, mission, heartbeat, or other non-AgoChat session needs to notify a user who currently has an AgoDesk desktop client connected.

### Parameters

| Field | Required | Description |
|---|---:|---|
| `message` | yes | Text to show in AgoChat |
| `device_id` | no | Connected AgoDesk RemoteHub device ID |
| `device_name` | no | Connected AgoDesk device name when `device_id` is omitted |
| `conversation_id` | no | AuraGo chat conversation ID (`sess-...`) to attach the proactive response to a specific shared chat |

If exactly one AgoChat device is connected, `device_id` can be omitted. If multiple devices are connected, provide `device_id`. The current system prompt includes connected AgoChat targets in `REACHABLE CHAT CHANNELS` when available.

### Example

```json
{"action": "send_agodesk_chat", "device_id": "device-123", "conversation_id": "sess-abc", "message": "Mission finished successfully."}
```

### Notes

- The client must be connected and paired.
- The backend sends this as a server-initiated AgoDesk `chat.response` with `metadata.server_push=true`.
- Include `conversation_id` when the notification belongs to a known shared AgoDesk/Web Chat conversation.
- Use normal chat replies for the current AgoChat request; use this tool for proactive outbound notifications from other sessions.
