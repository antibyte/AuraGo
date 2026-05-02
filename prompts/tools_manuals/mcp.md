# MCP (Model Context Protocol) — Tool Manual

## Overview
MCP enables AuraGo to connect to external MCP-compliant servers and use their tools. This extends AuraGo's capabilities dynamically without code changes.

## Tool: `mcp_call`

### Operation: `list_servers`
Lists all connected MCP servers.

```json
{"action": "mcp_call", "operation": "list_servers"}
```

Response:
```json
{"status": "success", "servers": [{"name": "my-server", "ready": true, "tool_count": 5}]}
```

### Operation: `list_tools`
Lists available tools, optionally filtered by server name.

```json
{"action": "mcp_call", "operation": "list_tools", "server": "my-server"}
```

Response:
```json
{"status": "success", "tools": [{"server": "my-server", "name": "get_weather", "description": "Fetches weather data", "input_schema": {"type": "object", "properties": {"city": {"type": "string"}}}}]}
```

### Operation: `call_tool`
Calls a specific tool on an MCP server.

```json
{"action": "mcp_call", "operation": "call_tool", "server": "my-server", "tool_name": "get_weather", "mcp_args": {"city": "Berlin"}}
```

Response contains the tool's output text.

## Security
- MCP must be explicitly enabled in two places:
  1. **Danger Zone**: `agent.allow_mcp: true` (capability gate)
  2. **MCP Section**: `mcp.enabled: true` (feature toggle)
- MCP clients can connect to local/Docker stdio servers or to network servers via streamable HTTP, SSE, or WebSocket.
- Local and Docker stdio servers can execute arbitrary code; network servers can expose remote capabilities. Only add trusted MCP servers.

## Configuration
```yaml
agent:
  allow_mcp: true   # Danger Zone gate

mcp:
  enabled: true
  servers:
    - name: "my-server"
      transport: stdio
      command: "npx"
      args: ["-y", "@my/mcp-server"]
      env:
        API_KEY: "{{api-token}}"
      enabled: true
    - name: "remote-tools"
      transport: streamable_http
      url: "https://example.com/mcp"
      headers:
        Authorization: "Bearer {{remote-mcp-token}}"
      allowed_tools: []
      allow_destructive: false
```
