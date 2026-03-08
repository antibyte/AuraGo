---
id: api_request
tags: [core]
priority: 100
conditions: ["allow_network_requests"]
---

## Tool: API Client (`api_request`)

Make HTTP requests to any API endpoint.

**Supported methods:** GET, POST, PUT, DELETE, PATCH
**Response cap:** 16 KB | **Timeout:** 30 seconds

For authenticated APIs, pass an `Authorization` header or retrieve the key via `get_secret` first.

### Examples

```json
{"action": "api_request", "method": "GET", "url": "https://api.example.com/data"}
```

```json
{"action": "api_request", "method": "POST", "url": "https://api.example.com/submit", "headers": {"Authorization": "Bearer sk-xxx"}, "content": "{\"key\": \"value\"}"}
```