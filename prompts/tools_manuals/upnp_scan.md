---
description: upnp_scan: Discover UPnP/SSDP devices on the local network.
---

# `upnp_scan` Tool

The **upnp_scan** tool discovers UPnP (Universal Plug and Play) and SSDP (Simple Service Discovery Protocol) devices on the local network. This covers a broad range of home-lab and smart-home devices: routers, NAS boxes, Smart TVs, media renderers, printers, IP cameras, and many IoT devices.

**Requires:** `tools.upnp_scan.enabled: true` in config.

## Parameters

- **`search_target`** *(string, optional)*: The UPnP search target. Default: `ssdp:all` (finds every UPnP device). Common values:
  - `ssdp:all` — discover all devices
  - `upnp:rootdevice` — root devices only (one response per device)
  - `urn:schemas-upnp-org:device:MediaRenderer:1` — media renderers (Kodi, TVs, etc.)
  - `urn:schemas-upnp-org:device:InternetGatewayDevice:1` — routers / gateways
  - `urn:schemas-upnp-org:device:MediaServer:1` — media servers (Plex, Jellyfin, etc.)

- **`timeout_secs`** *(integer, optional)*: How long to wait for responses in seconds (1–30, default: 5). Increase to 10–15 on a large or slow network.

## Usage Examples

Discover all UPnP devices:
```json
{
  "action": "upnp_scan"
}
```

Find only media renderers with a longer timeout:
```json
{
  "action": "upnp_scan",
  "search_target": "urn:schemas-upnp-org:device:MediaRenderer:1",
  "timeout_secs": 10
}
```

Find internet gateway devices (routers):
```json
{
  "action": "upnp_scan",
  "search_target": "urn:schemas-upnp-org:device:InternetGatewayDevice:1"
}
```

## Returns

A JSON object with:
- `status`: `"success"` or `"error"`
- `count`: number of unique devices found
- `devices`: array of device objects, each with:
  - `usn`: Unique Service Name (device identifier)
  - `location`: URL of the device description XML
  - `friendly_name`: human-readable device name
  - `device_type`: short device type (e.g. `MediaRenderer`, `InternetGatewayDevice`)
  - `manufacturer`: device manufacturer
  - `model_name`: model name
  - `model_description`: model description
  - `serial_number`: device serial number
  - `services`: list of services with `service_type` and `service_id`
- `message`: present on empty results or errors

## Notes

- Results only contain devices that are reachable and respond within the timeout window.
- Duplicate devices (same UDN appearing via multiple interfaces) are deduplicated automatically.
- This tool is disabled by default. Enable it in config: `tools.upnp_scan.enabled: true`
- Unlike `mdns_scan`, UPnP does not use mDNS — it uses SSDP multicast on 239.255.255.250:1900.
