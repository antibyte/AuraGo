---
tool: whois_lookup
version: 1
tags: ["always"]
---

# WHOIS Lookup Tool

Query domain registration information from WHOIS servers.

## Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `domain` | string | **Required.** Domain name to look up (e.g. `example.com`) |
| `include_raw` | boolean | Include the raw WHOIS response text (default: false) |

## Response Fields

| Field | Description |
|-------|-------------|
| `registrar` | Domain registrar name |
| `created` | Registration/creation date |
| `expires` | Expiration date |
| `updated` | Last modification date |
| `domain_status` | Array of EPP status codes |
| `name_servers` | Array of authoritative name servers |
| `dnssec` | DNSSEC status |
| `extra` | Additional fields (registrant, country if available) |

## Supported TLDs

Automatic WHOIS server selection for 30+ TLDs including: com, net, org, io, dev, de, uk, fr, nl, eu, ch, at, se, no, dk, it, es, pl, cz, us, ca, au, jp, cn, ru, br, in, and more.

## Examples

Basic lookup:
```json
{"domain": "example.com"}
```

With raw response:
```json
{"domain": "example.de", "include_raw": true}
```

## Notes

- Complements the `dns_lookup` tool which queries DNS records
- Some registrars redact WHOIS data for privacy (GDPR)
- Rate limits may apply for bulk lookups
- URLs are auto-stripped to extract the domain name
