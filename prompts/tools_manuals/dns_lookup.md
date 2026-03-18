# Tool: `dns_lookup`

## Purpose
Perform DNS record lookups for a hostname using the system's DNS resolver.

## When to Use
- Verify DNS configuration for a domain or subdomain.
- Check A/AAAA records before connecting to a service.
- Look up MX records to debug email delivery issues.
- Discover NS, TXT (SPF/DKIM), or CNAME records.
- Reverse-lookup an IP address (PTR records).

## Parameters
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `host` | string | ✅ | Hostname or domain to look up (e.g. `example.com`) |
| `record_type` | string | ❌ | Record type: `all` (default), `A`, `AAAA`, `MX`, `NS`, `TXT`, `CNAME`, `PTR` |

## Output
JSON with: `status`, `host`, `record_type`, `records` (array of `{type, value, priority?}`), `message`.

## Example Calls
```json
{ "host": "example.com" }
{ "host": "example.com", "record_type": "MX" }
{ "host": "8.8.8.8", "record_type": "PTR" }
{ "host": "mail.example.com", "record_type": "A" }
```
