# Fritz!Box Smart Home Tool (`fritzbox_smarthome`)

Control Fritz!DECT smart home devices: smart plugs (switches), thermostats (FRITZ!DECT 301/302), colour lamps (FRITZ!DECT 500), and automation templates.

**Requires**: `fritzbox.smarthome.enabled: true` in config.
Write operations additionally require `fritzbox.smarthome.readonly: false`.

## Key Operations

| Operation | Description | Parameters |
|-----------|-------------|------------|
| `get_devices` | List all paired smart home devices with state and measurements | — |
| `set_switch` | Turn a smart plug on or off | `ain`, `enabled` |
| `set_heating` | Set target temperature on a thermostat | `ain`, `temp_c` (float, °C) |
| `set_brightness` | Set lamp brightness | `ain`, `brightness` (int 0–100, percent) |
| `get_templates` | List saved automation templates | — |
| `apply_template` | Apply an automation template | `ain` (template ID) |

## Key Parameter: `ain`

The **AIN** (Actor Identification Number) uniquely identifies each Fritz!DECT device or template.
Format examples: `"087610123456"`, `"12345 6789012"`. Retrieve AIns from `get_devices` or `get_templates`.

## Examples

```json
{"action": "fritzbox_smarthome", "operation": "get_devices"}
```

```json
{"action": "fritzbox_smarthome", "operation": "set_switch", "ain": "087610123456", "enabled": true}
```

```json
{"action": "fritzbox_smarthome", "operation": "set_heating", "ain": "087610123456", "temp_c": 21.5}
```

```json
{"action": "fritzbox_smarthome", "operation": "set_brightness", "ain": "087610123456", "brightness": 75}
```

```json
{"action": "fritzbox_smarthome", "operation": "get_templates"}
```

```json
{"action": "fritzbox_smarthome", "operation": "apply_template", "ain": "tmp0B7610-1234ABCD"}
```

## Notes

- `get_devices` response includes power consumption (watts), energy totals (kWh), and thermostat readings where available.
- `set_heating`: valid range is typically 8–28 °C. Use 0 for "off" (frost-protection mode) and 100 for "on" (continuous heating).
- `brightness` is a percentage (0–100). The Fritz!Box converts it to the device-specific range internally.
- Template AIns begin with `"tmp"` and can be listed via `get_templates`.
