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

## Usage Example (JSON format)

To find all Chromecasts on the network:
```json
{
  "action": "mdns_scan",
  "service_type": "_googlecast._tcp",
  "timeout": 5
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

## Notes

- Scanning takes time. You will not receive a response until the `timeout` expires.
- Not all devices respond to generic scans. If you are looking for a specific type of device, it is much more reliable to search for its exact `service_type`.
