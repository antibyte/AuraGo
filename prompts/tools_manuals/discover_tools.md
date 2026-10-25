# discover_tools

Browse and search the full tool catalog, including active, hidden, and disabled tools.

## When to use
- You need a tool that is **not in your current tool list** (hidden by adaptive filtering)
- You want to find the right tool for a task but aren't sure which one exists
- You need to see the **exact parameters** and the correct call method before calling a hidden tool

## Output

`discover_tools` returns a JSON envelope under `Tool Output:`. Read the machine-readable fields first:

- `status`: request status
- `results[]` or `tool`: catalog entries; `categories[]` for category lists, `manual` for a guide
- `kind`: catalog tool kind; `status`: `active`, `hidden`, or `disabled`
- `call_method`: `direct`, `invoke_tool`, `execute_skill`, `run_tool`, `activate_agent_skill`, `composio_call`, `needs_setup`, or `disabled`
- `callable_now`: whether the tool can be used immediately
- `schema_available`: whether a parameter schema is included
- `instruction`: concise call guidance
- `hidden_reason`: why an unavailable entry cannot be called; discovery never grants permission
- `next_cursor`: pass this cursor to continue a paged result or manual; omit `cursor` on page 1
- `cursor_reset` and `notice`: your cursor was invalid or the catalog changed, so the first page is shown again
- `no_results`: the search found nothing (a normal answer, not an error); try `list_categories`, a shorter English keyword, or one of `suggestions`
- `suggestions`: close tool names for a search with no hits or an unknown name
- `valid_categories`: the category names to use when `list_family` rejects an unknown `category`

## Operations

### list_categories
Browse tool categories. Returns category names with their counts and a short purpose, not the tools themselves; pass a `category` (or use `list_family`) to list the tools of one category.

```json
{"action": "discover_tools", "operation": "list_categories"}
{"action": "discover_tools", "operation": "list_categories", "category": "network"}
{"action": "discover_tools", "operation": "list_family", "category": "network"}
```

Categories, e.g.: `system`, `memory`, `files`, `network`, `media`, `smart_home`, `infrastructure`, `data_apis`, `communication`, plus the runtime categories `skills`, `custom`, `mcp`, `agent_skills`. Call `list_categories` for the categories this run actually has.

### search
Find tools by keyword across all categories. Matches tool names and descriptions.

```json
{"action": "discover_tools", "operation": "search", "query": "docker"}
{"action": "discover_tools", "operation": "search", "query": "email"}
```

### get_tool_info and get_manual
`get_tool_info` returns the full parameter schema, status, and call method for a specific tool. **Use this before calling a hidden tool** — it shows the exact parameters and whether to use `invoke_tool`, `execute_skill`, `run_tool`, or a direct native call. `get_manual` returns the tool's guide text in pages (`manual`, `next_cursor`); keep passing `next_cursor` until it is absent.

```json
{"action": "discover_tools", "operation": "get_tool_info", "tool_name": "docker"}
{"action": "discover_tools", "operation": "get_manual", "tool_name": "docker"}
```

## Workflow
1. Use `list_categories` or `search` to find the tool you need
2. Use `get_tool_info` to see its full parameter schema; use `get_manual` for workflow guidance
3. Follow the returned `call_method`:
   - `direct`: call the tool itself with its own arguments. This includes a hidden tool whose schema is queued for your next request; if it is not in your tool list then, call `invoke_tool` with the same `tool_name` and arguments
   - `invoke_tool`: call `invoke_tool` with `tool_name` and `arguments`; the system will re-inject the real native schema for follow-up calls
   - `execute_skill`: use `execute_skill` with `skill` and `skill_args`
   - `run_tool`: use `run_tool`
   - `activate_agent_skill`: load the package with `activate_agent_skill`
   - `composio_call`: use the service instructions in the catalog result
   - `needs_setup`: resolve `hidden_reason` before retrying
   - `disabled`: stop and tell the user the tool must be enabled first

## Important
- Hidden native tools are enabled but absent from your current schema due to adaptive filtering. Follow the `call_method` that `discover_tools` returns: `direct` means call the tool itself, `invoke_tool` means go through `invoke_tool`. `invoke_tool` is also the fallback when a direct call is unavailable or an enabled active tool cannot be called directly by the current model/runtime.
- `get_tool_info`, `search`, `list_categories`, `list_family` and `get_manual` are operations of `discover_tools`, not tools of their own; always call `discover_tools` and name the operation.
- Disabled tools (✗) cannot be called — they must be enabled in the config first
- Native AuraGo tools are not skills. Never use `execute_skill` for a `kind: native` result.
