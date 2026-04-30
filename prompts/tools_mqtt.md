---
id: "tools_mqtt"
tags: ["conditional"]
priority: 32
conditions: ["mqtt_enabled"]
---
### MQTT
| Tool | Purpose |
|---|---|
| `mqtt_publish` | Publish a message to an MQTT topic |
| `mqtt_subscribe` | Subscribe to an MQTT topic to receive messages |
| `mqtt_unsubscribe` | Unsubscribe from an MQTT topic |
| `mqtt_get_messages` | Retrieve recently received MQTT messages from the buffer |

**Notes:**
- Topics use `/` as separator (e.g. `home/sensors/temperature`)
- Wildcards: `+` matches one level, `#` matches all remaining levels
- Publish topics must be concrete topic names; wildcards are only valid for subscribe/unsubscribe and message retrieval filters
- QoS levels: 0 = at most once, 1 = at least once, 2 = exactly once
- Retained messages are stored by the broker and sent to new subscribers
- Very large payloads are rejected or truncated according to `mqtt.buffer.max_payload_bytes`
- Use `mqtt_get_messages` with an empty topic or `#` to see all buffered messages
