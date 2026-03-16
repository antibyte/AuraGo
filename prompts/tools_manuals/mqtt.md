# MQTT Tools

Publish and subscribe to MQTT topics for IoT device communication.

## Tools

### `mqtt_publish` — Publish a message
```json
{"action": "mqtt_publish", "topic": "home/living_room/light", "payload": "{\"state\": \"on\"}", "qos": 1, "retain": true}
```

### `mqtt_subscribe` — Subscribe to a topic
```json
{"action": "mqtt_subscribe", "topic": "home/sensors/#", "qos": 1}
```

### `mqtt_unsubscribe` — Unsubscribe from a topic
```json
{"action": "mqtt_unsubscribe", "topic": "home/sensors/#"}
```

### `mqtt_get_messages` — Get received messages
```json
{"action": "mqtt_get_messages", "topic": "home/sensors/temperature", "limit": 10}
```

## Notes
- QoS levels: 0 (at most once), 1 (at least once), 2 (exactly once)
- Use `#` wildcard for multi-level, `+` for single-level topic matching
- `retain`: broker keeps last message for new subscribers
