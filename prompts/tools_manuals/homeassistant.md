# Home Assistant (`home_assistant`, `homeassistant`, `ha`)

Control your smart home via Home Assistant. Get entity states, call services, and monitor your home.

## Operations

| Operation | Aliases | Description |
|-----------|---------|-------------|
| `get_states` | `list_states`, `states` | List all entities or filter by domain |
| `get_state` | `state` | Get a specific entity's state |
| `call_service` | `service` | Call a Home Assistant service |
| `list_services` | `services` | List available services |

## Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `operation` | string | yes | One of: get_states, get_state, call_service, list_services |
| `domain` | string | for get_states, list_services | Filter by domain (e.g. `light`, `switch`, `sensor`, `climate`) |
| `entity_id` | string | for get_state, call_service | Full entity ID (e.g. `light.living_room`) |
| `service` | string | for call_service | Service to call (e.g. `turn_on`, `turn_off`, `toggle`) |
| `service_data` | object | for call_service | Optional data to pass with the service call |

## Examples

**List all entities:**
```json
{"action": "homeassistant", "operation": "get_states"}
```

**List all lights:**
```json
{"action": "homeassistant", "operation": "get_states", "domain": "light"}
```

**Get specific entity state:**
```json
{"action": "homeassistant", "operation": "get_state", "entity_id": "climate.thermostat"}
```

**Turn on a light:**
```json
{"action": "homeassistant", "operation": "call_service", "domain": "light", "service": "turn_on", "entity_id": "light.living_room"}
```

**Turn off all switches:**
```json
{"action": "homeassistant", "operation": "call_service", "domain": "switch", "service": "turn_off"}
```

**List available services in a domain:**
```json
{"action": "homeassistant", "operation": "list_services", "domain": "climate"}
```

## Configuration

```yaml
home_assistant:
  enabled: true
  url: "http://homeassistant.local:8123"  # Home Assistant URL
  access_token: "your_long_lived_access_token"  # From Home Assistant profile
  read_only: false  # Set true to block service calls
```

## Notes

- **Entity IDs**: Format is `domain.name` (e.g. `light.living_room`, `sensor.temperature`).
- **Domains**: Common domains include `light`, `switch`, `sensor`, `climate`, `cover`, `automation`, `script`.
- **Read-only mode**: When `home_assistant.read_only: true`, service calls are blocked.
- **Service data**: Some services accept additional data (e.g. `{"brightness": 255}` for `turn_on`).
- **Long-lived tokens**: Create in Home Assistant UI → Profile → Long-Lived Access Tokens.