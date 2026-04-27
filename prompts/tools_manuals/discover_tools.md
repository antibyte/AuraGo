# discover_tools

Browse and search the full tool catalog, including active, hidden, and disabled tools.

## When to use
- You need a tool that is **not in your current tool list** (hidden by adaptive filtering)
- You want to find the right tool for a task but aren't sure which one exists
- You need to see the **exact parameters** and the correct call method before calling a hidden tool

## Output

`discover_tools` returns a JSON envelope under `Tool Output:`. Read the machine-readable fields first:

- `status`: request status
- `results[]` or `tool`: catalog entries
- `kind`: `native`, `skill`, `custom`, or `mcp`
- `status`: `active`, `hidden`, or `disabled`
- `call_method`: `direct`, `invoke_tool`, `execute_skill`, or `run_tool`
- `callable_now`: whether the tool can be used immediately
- `schema_available`: whether a parameter schema is included
- `instruction`: concise call guidance

## Operations

### list_categories
Browse tools organized by category. Shows active, hidden, and disabled catalog entries.

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
Get the full parameter schema, usage guide, status, and call method for a specific tool. **Use this before calling a hidden tool** — it shows the exact parameters and whether to use `invoke_tool`, `execute_skill`, `run_tool`, or a direct native call.

```json
{"action": "discover_tools", "operation": "get_tool_info", "tool_name": "docker"}
```

## Workflow
1. Use `list_categories` or `search` to find the tool you need
2. Use `get_tool_info` to see its full parameter schema
3. Follow the returned `call_method`:
   - `direct`: call the native tool normally
   - `invoke_tool`: call `invoke_tool` with `tool_name` and `arguments`; the system will re-inject the real native schema for follow-up calls
   - `execute_skill`: use `execute_skill` with `skill` and `skill_args`
   - `run_tool`: use `run_tool`

## Important
- Hidden native tools are enabled but absent from your current schema due to adaptive filtering. Use `invoke_tool` only as the recovery path after `discover_tools` tells you to.
- Disabled tools (✗) cannot be called — they must be enabled in the config first
- Native AuraGo tools are not skills. Never use `execute_skill` for a `kind: native` result.
