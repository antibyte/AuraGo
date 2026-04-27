# invoke_tool

Recovery-only invoker for enabled native tools that are hidden by adaptive filtering.

## When to use

Use this tool only when `discover_tools` returns a catalog entry with:

- `kind: "native"`
- `status: "hidden"`
- `call_method: "invoke_tool"`
- `callable_now: true`

Do not use `invoke_tool` for normal active tools, disabled tools, or Python skills.

## Parameters

| Parameter | Required | Description |
|-----------|----------|-------------|
| `tool_name` | Yes | Native tool name returned by `discover_tools` |
| `arguments` | Yes | Arguments matching the schema returned by `discover_tools` |

## Usage

Use the `invoke_tool` tool with:

- `tool_name`: the hidden native tool name returned by `discover_tools`
- `arguments`: the parameter object from that tool's schema

For example, after discovering `yepapi_instagram`, invoke `invoke_tool` with `tool_name` set to `yepapi_instagram` and put Instagram parameters such as `operation`, `username`, or `username_or_url` inside `arguments`.

After a hidden native tool is discovered or invoked, AuraGo marks it for schema re-injection so follow-up calls can use the real native tool schema directly.

## Important

- `invoke_tool` routes to the real native handler. Security, audit, and usage tracking should treat the underlying tool as the executed tool.
- If `discover_tools` returns `kind: "skill"`, use `execute_skill` instead.
- If `discover_tools` returns `status: "disabled"`, the tool must be enabled in config before it can run.
