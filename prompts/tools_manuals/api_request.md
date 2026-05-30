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

### Local/Internal Safety

`api_request` blocks private, link-local, and loopback addresses by default (SSRF protection). The only local exception is the configured Ollama API base URL (`ollama.url`), and only for Ollama API paths under `/api/` or `/v1/`. Use the configured Ollama URL, normally `http://localhost:11434`, for local Ollama chat/completion calls. Do not use AuraGo's internal loopback port (for example `127.0.0.1:18080`) as an Ollama endpoint.

### Examples

```json
{"action": "api_request", "method": "GET", "url": "https://api.example.com/data"}
```

```json
{"action": "api_request", "method": "POST", "url": "https://api.example.com/submit", "headers": {"Authorization": "Bearer sk-xxx"}, "content": "{\"key\": \"value\"}"}
```
