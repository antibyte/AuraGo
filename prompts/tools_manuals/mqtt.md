# MQTT Tools (`mqtt_publish`, `mqtt_subscribe`, `mqtt_unsubscribe`, `mqtt_get_messages`)

Publish and subscribe to MQTT topics for IoT device communication.

## Operations

| Operation | Description | Parameters |
|-----------|-------------|------------|
| `mqtt_publish` | Publish a message to a topic | `topic`, `payload`, `qos`, `retain` |
| `mqtt_subscribe` | Subscribe to a topic | `topic`, `qos` |
| `mqtt_unsubscribe` | Unsubscribe from a topic | `topic` |
| `mqtt_get_messages` | Retrieve recently received messages | `topic`, `limit` |

## Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `topic` | string | yes | MQTT topic (e.g. `home/living_room/light`) |
| `payload` | string | for publish | Message content (string or JSON) |
| `qos` | integer | no | Quality of Service: 0 (at most once), 1 (at least once), 2 (exactly once). Default: 0 |
| `retain` | boolean | no | Whether broker should retain the message. Default: false |
| `limit` | integer | for get_messages | Max messages to return (default: 20) |

## Examples

**Publish a JSON payload:**
```json
{"action": "mqtt_publish", "topic": "home/sensors/temperature", "payload": "{\"value\": 22.5, \"unit\": \"celsius\"}"}
```

**Publish with QoS 1:**
```json
{"action": "mqtt_publish", "topic": "home/alarm", "payload": "TRIGGERED", "qos": 1}
```

**Subscribe to all topics in home:**
```json
{"action": "mqtt_subscribe", "topic": "home/#"}
```

**Subscribe to specific pattern:**
```json
{"action": "mqtt_subscribe", "topic": "home/+/light"}
```

**Get recent messages:**
```json
{"action": "mqtt_get_messages", "topic": "#", "limit": 50}
```

**Unsubscribe:**
```json
{"action": "mqtt_unsubscribe", "topic": "home/temp_sensor"}
```

## Configuration

```yaml
mqtt:
  enabled: true
  broker: "tcp://localhost:1883"  # Or "mqtts://" for TLS
  username: ""  # Optional
  password: ""  # Optional
  client_id: "aurago"  # MQTT client identifier
  clean_session: true
```

## Notes

- **Topics**: Use `/` to separate levels (e.g. `home/living_room/light`).
- **Wildcards**: `+` matches one level (e.g. `home/+/light`), `#` matches all remaining (e.g. `home/#`).
- **QoS levels**: 0 = fire-and-forget, 1 = at least once, 2 = exactly once.
- **Retained messages**: Useful for sensor state (e.g. last temperature reading).
- **Message buffer**: Messages are buffered for retrieval via `mqtt_get_messages`.
