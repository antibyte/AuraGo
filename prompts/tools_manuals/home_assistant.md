# home_assistant — Smart Home Control

Control Home Assistant devices and services via the HA REST API.

> **Requires** `home_assistant.enabled: true` and a valid Long-Lived Access Token in `config.yaml`.

## Operations

### get_states
Retrieve all entity states, optionally filtered by domain.

```json
{"action": "home_assistant", "operation": "get_states"}
{"action": "home_assistant", "operation": "get_states", "domain": "light"}
{"action": "home_assistant", "operation": "get_states", "domain": "climate"}
```

Returns: list of `{entity_id, state, friendly_name}` for each matching entity.

---

### get_state
Get the full state of a single entity.

```json
{"action": "home_assistant", "operation": "get_state", "entity_id": "light.living_room"}
{"action": "home_assistant", "operation": "get_state", "entity_id": "sensor.temperature_bedroom"}
```

Returns: complete entity object with state, attributes, last_changed, etc.

---

### call_service
Call a Home Assistant service to control a device.

```json
{"action": "home_assistant", "operation": "call_service", "domain": "light", "service": "turn_on", "entity_id": "light.living_room"}
{"action": "home_assistant", "operation": "call_service", "domain": "light", "service": "turn_on", "entity_id": "light.living_room", "service_data": {"brightness": 128, "color_name": "blue"}}
{"action": "home_assistant", "operation": "call_service", "domain": "switch", "service": "turn_off", "entity_id": "switch.heater"}
{"action": "home_assistant", "operation": "call_service", "domain": "scene", "service": "turn_on", "entity_id": "scene.movie_night"}
{"action": "home_assistant", "operation": "call_service", "domain": "climate", "service": "set_temperature", "entity_id": "climate.thermostat", "service_data": {"temperature": 21}}
```

**Common domains & services:**
| Domain | Services |
|---|---|
| `light` | `turn_on`, `turn_off`, `toggle` (+ brightness, color_name, rgb_color, color_temp) |
| `switch` | `turn_on`, `turn_off`, `toggle` |
| `climate` | `set_temperature`, `set_hvac_mode`, `turn_on`, `turn_off` |
| `scene` | `turn_on` |
| `media_player` | `play_media`, `media_pause`, `media_play`, `volume_set` |
| `cover` | `open_cover`, `close_cover`, `stop_cover` |
| `script` | `turn_on` (run a HA script) |
| `automation` | `trigger`, `turn_on`, `turn_off` |

---

### list_services
List available services, optionally filtered by domain.

```json
{"action": "home_assistant", "operation": "list_services"}
{"action": "home_assistant", "operation": "list_services", "domain": "light"}
```

Returns: list of domains with their available service names.

---

## Workflow

1. Use `get_states` with a `domain` filter to discover available entities
2. Use `get_state` to inspect a specific entity's current state
3. Use `call_service` to control devices
4. Use `list_services` if unsure what services a domain supports

## Error Handling

- Returns `{"status": "error", ...}` if HA is unreachable, the entity doesn't exist, or the service call fails.
- If the tool returns "Home Assistant integration is not enabled", the user needs to configure it in `config.yaml`.
