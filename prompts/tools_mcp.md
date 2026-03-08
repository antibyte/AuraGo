---
id: "tools_mcp"
tags: ["conditional"]
priority: 33
conditions: ["mcp_enabled"]
---
### MCP (Model Context Protocol)
| Tool | Purpose |
|---|---|
| `mcp_call` | Interact with external MCP servers — list servers, discover tools, call tools |

**Operations:**
| Operation | Required Fields | Description |
|---|---|---|
| `list_servers` | — | Lists all connected MCP servers and their status |
| `list_tools` | `server` (optional) | Lists available tools; filter by server name or leave empty for all |
| `call_tool` | `server`, `tool_name` | Calls a specific tool on an MCP server; pass arguments via `mcp_args` |

**Notes:**
- Always call `list_servers` first to discover which MCP servers are available
- Then use `list_tools` with a server name to see what tools the server offers
- Tool arguments (`mcp_args`) depend on the specific tool — check `input_schema` from `list_tools`
- MCP servers are external processes — responses may be slow depending on the server
