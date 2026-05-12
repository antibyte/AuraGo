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

## Docker Deployment Patterns

When AuraGo runs in Docker, choose the MCP pattern based on where the MCP server can run.

### MCP server in its own container
Use `runtime: docker` for stdio MCP servers that can run from a Docker image. AuraGo starts the MCP server through the Docker proxy sidecar and communicates with it over stdio.

```yaml
mcp:
  enabled: true
  servers:
    - name: "container-mcp"
      enabled: true
      transport: stdio
      runtime: docker
      docker_image: "ghcr.io/astral-sh/uv:latest"
      docker_command: "uvx"
      args: ["mcp-server-fetch"]
      allowed_tools: []
      allow_destructive: false
```

### MCP server installed on the Docker host
Use `transport: streamable_http` with `host.docker.internal` when the MCP server must run on the host as a stdio process. Run a stdio-to-HTTP bridge such as `supergateway` on the host, then point AuraGo at the bridge URL. The Docker Compose setup maps `host.docker.internal` to the host gateway for Linux Docker Engine compatibility.

Host command:

```bash
npx -y supergateway --stdio "uvx mcp-server-fetch" --port 9100 --outputTransport streamableHttp --streamableHttpPath /mcp
```

AuraGo configuration:

```yaml
mcp:
  enabled: true
  servers:
    - name: "host-fetch"
      enabled: true
      transport: streamable_http
      url: "http://host.docker.internal:9100/mcp"
      allowed_tools: []
      allow_destructive: false
```

Do not use `localhost` or `127.0.0.1` for host services from inside the AuraGo container; those addresses refer to the container itself.

### AuraGo running outside Docker
Use `runtime: local` only when AuraGo itself runs directly on the same host as the stdio MCP server. This starts the command with the local operating system process environment.

```yaml
mcp:
  enabled: true
  servers:
    - name: "local-fetch"
      enabled: true
      transport: stdio
      runtime: local
      command: "uvx"
      args: ["mcp-server-fetch"]
      allowed_tools: []
      allow_destructive: false
```
