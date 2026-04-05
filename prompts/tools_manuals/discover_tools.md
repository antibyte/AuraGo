# discover_tools

Browse and search all available tools, including those hidden by adaptive filtering.

## When to use
- You need a tool that is **not in your current tool list** (hidden by adaptive filtering)
- You want to find the right tool for a task but aren't sure which one exists
- You need to see the **exact parameters** of a hidden tool before calling it

## Operations

### list_categories
Browse tools organized by category. Shows which tools are active (●) vs hidden (○).

```json
{"action": "discover_tools", "operation": "list_categories"}
{"action": "discover_tools", "operation": "list_categories", "category": "network"}
```

Categories: `system`, `files`, `network`, `media`, `smart_home`, `infrastructure`, `communication`

### search
Find tools by keyword across all categories. Matches tool names and descriptions.

```json
{"action": "discover_tools", "operation": "search", "query": "docker"}
{"action": "discover_tools", "operation": "search", "query": "email"}
```

### get_tool_info
Get the full parameter schema and usage guide for a specific tool. **Use this before calling a hidden tool** — it shows you the exact parameters you need.

```json
{"action": "discover_tools", "operation": "get_tool_info", "tool_name": "docker"}
```

## Workflow
1. Use `list_categories` or `search` to find the tool you need
2. Use `get_tool_info` to see its full parameter schema
3. Call the tool directly — the system accepts any valid tool call regardless of adaptive filtering

## Important
- Hidden tools (○) are fully functional — they are just not in your current prompt to save tokens
- Disabled tools (✗) cannot be called — they must be enabled in the config first
- After `get_tool_info`, you have everything needed to make a correct tool call
