## Tools: Telnyx Integration (`telnyx_sms`, `telnyx_call`, `telnyx_manage`)

Telnyx provides SMS/MMS messaging and voice call control. Use these tools to send text messages, make phone calls with TTS, gather DTMF input, transfer calls, and manage Telnyx resources.

---

### Tool: `telnyx_sms`

Send SMS/MMS messages and check delivery status.

#### Parameters

| Parameter    | Required | Description |
|-------------|----------|-------------|
| `operation` | yes | `"send"`, `"send_mms"`, or `"status"` |
| `to`        | send/send_mms | Phone number in E.164 format (e.g. `"+4915123456789"`) |
| `message`   | send | Text message body |
| `media_urls`| send_mms | Array of media URLs (max 10) for MMS |
| `message_id`| status | Message ID to check delivery status |

#### Examples

**Send SMS:**
```json
{"action": "telnyx_sms", "operation": "send", "to": "+4915123456789", "message": "Hello from AuraGo!"}
```

**Send MMS with image:**
```json
{"action": "telnyx_sms", "operation": "send_mms", "to": "+4915123456789", "message": "Check this out", "media_urls": ["https://example.com/photo.jpg"]}
```

**Check message status:**
```json
{"action": "telnyx_sms", "operation": "status", "message_id": "40016a6b-8a5e-4e5f-8c6f-abc123"}
```

---

### Tool: `telnyx_call`

Initiate and control voice calls. Supports TTS speech, audio playback, DTMF input gathering, call transfer, and recording.

#### Parameters

| Parameter         | Required | Description |
|------------------|----------|-------------|
| `operation`      | yes | `"initiate"`, `"speak"`, `"play_audio"`, `"gather_dtmf"`, `"transfer"`, `"record_start"`, `"record_stop"`, `"hangup"`, `"list_active"` |
| `to`             | initiate/transfer | Phone number in E.164 format |
| `call_control_id`| speak/play_audio/gather_dtmf/transfer/record_*/hangup | Active call's control ID (returned by initiate or webhook) |
| `text`           | speak/gather_dtmf | Text to speak via TTS |
| `audio_url`      | play_audio | URL of audio file to play |
| `max_digits`     | no | Max DTMF digits to collect (default: 1) |
| `timeout_secs`   | no | Timeout for DTMF gathering (default: 10) |

#### Call Flow

1. **Initiate** a call → receive `call_control_id`
2. **Speak** text or **play audio** on the active call
3. Optionally **gather DTMF** input (e.g. "Press 1 for yes")
4. **Transfer** to another number or **hang up**
5. Use **record_start/record_stop** to record portions of the call

#### Examples

**Make a call:**
```json
{"action": "telnyx_call", "operation": "initiate", "to": "+4915123456789"}
```

**Speak on active call:**
```json
{"action": "telnyx_call", "operation": "speak", "call_control_id": "v2-ctrl-abc123", "text": "Hello, this is AuraGo calling."}
```

**Gather DTMF digits:**
```json
{"action": "telnyx_call", "operation": "gather_dtmf", "call_control_id": "v2-ctrl-abc123", "text": "Press 1 to confirm or 2 to cancel", "max_digits": 1, "timeout_secs": 15}
```

**Transfer call:**
```json
{"action": "telnyx_call", "operation": "transfer", "call_control_id": "v2-ctrl-abc123", "to": "+4915198765432"}
```

**Start recording:**
```json
{"action": "telnyx_call", "operation": "record_start", "call_control_id": "v2-ctrl-abc123"}
```

**Hang up:**
```json
{"action": "telnyx_call", "operation": "hangup", "call_control_id": "v2-ctrl-abc123"}
```

---

### Tool: `telnyx_manage`

Manage Telnyx phone resources and view history.

#### Parameters

| Parameter   | Required | Description |
|------------|----------|-------------|
| `operation`| yes | `"list_numbers"`, `"check_balance"`, `"message_history"`, `"call_history"` |
| `limit`    | no | Max results (default: 20) |
| `page`     | no | Page number (default: 1) |

#### Examples

**List phone numbers:**
```json
{"action": "telnyx_manage", "operation": "list_numbers"}
```

**Check balance:**
```json
{"action": "telnyx_manage", "operation": "check_balance"}
```

**View message history:**
```json
{"action": "telnyx_manage", "operation": "message_history", "limit": 10}
```

---

### Incoming Messages & Calls

When Telnyx is enabled, incoming SMS messages and voice calls arrive via webhook and are automatically relayed to the agent. Incoming SMS content is wrapped in `<external_data>` tags for safety.

**Phone number format:** Always use E.164 format: `+` followed by country code and number (e.g. `+4915123456789`, `+15551234567`).

### Security Notes

- Only numbers in `telnyx.allowed_numbers` can trigger incoming message processing
- Webhook signatures are verified using Ed25519
- Rate limiting prevents SMS flood attacks
- The agent cannot access the Telnyx API key directly — it is stored in the vault
