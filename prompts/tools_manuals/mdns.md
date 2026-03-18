---
description: mdns_scan: Scan the local network for MDNS (ZeroConf/Bonjour) devices and services.
---

# `mdns_scan` Tool

The **mdns_scan** tool allows you to scan the local area network (LAN) for devices broadcasting services via MDNS (Multicast DNS / Bonjour / ZeroConf).

This is highly useful for discovering IoT devices, printers, smart home hubs, or other agent instances running on the local network.

## Parameters

- **`service_type`** *(string, optional)*
  The specific MDNS service type to scan for. 
  - Standard format is `_service._protocol.local.`
  - Example for Google Cast: `_googlecast._tcp`
  - Example for Apple AirPlay: `_airplay._tcp`
  - Example for HTTP servers: `_http._tcp`
  - If omitted, the tool attempts a generic scan for all broadcasting services (`_services._dns-sd._udp`).

- **`timeout`** *(integer, optional)*
  The duration in seconds to listen for responses. Default is `5` seconds. Setting this too high keeps you waiting; setting it too low might miss slow devices. `5` to `10` is recommended.

- **`auto_register`** *(boolean, optional)*
  If `true`, all discovered devices are automatically saved to the device inventory in a **single tool call**. This avoids the token cost of calling `manage_inventory` individually for each device. Default: `false`.

- **`register_type`** *(string, optional)*
  Device type label to assign to auto-registered devices (e.g. `"iot"`, `"printer"`, `"server"`). Defaults to `"mdns-device"` when not set.

- **`register_tags`** *(array of strings, optional)*
  Tags to attach to each auto-registered device (e.g. `["mdns", "home-lab"]`).

- **`overwrite_existing`** *(boolean, optional)*
  If `true`, update the inventory record when a device with the same name already exists. Default: `false` (skip duplicates silently).

## Usage Example (JSON format)

To find all Chromecasts and register them in the device inventory in one step:
```json
{
  "action": "mdns_scan",
  "service_type": "_googlecast._tcp",
  "timeout": 5,
  "auto_register": true,
  "register_type": "chromecast",
  "register_tags": ["mdns", "chromecast"]
}
```

To perform a generic device discovery scan:
```json
{
  "action": "mdns_scan"
}
```

## Returns

A JSON string containing:
- `status`: "success" or "error".
- `count`: The number of devices found.
- `devices`: An array of objects, containing:
  - `name`: the human-readable network name of the device.
  - `host`: the hostname.
  - `ips`: a list of IPs (IPv4 / IPv6).
  - `port`: the port the service is running on.
  - `info`: additional TXT record info provided by the device (e.g., model name, status).
- `auto_register` *(only when `auto_register: true`)*: registration summary:
  - `created`: number of new devices added to the inventory.
  - `updated`: number of existing devices updated (only when `overwrite_existing: true`).
  - `skipped`: number of devices skipped (already exist and `overwrite_existing` is false, or errors).

## Notes

- Scanning takes time. You will not receive a response until the `timeout` expires.
- Not all devices respond to generic scans. If you are looking for a specific type of device, it is much more reliable to search for its exact `service_type`.
