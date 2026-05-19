# run_tool

Run a saved custom Python tool from `agent_workspace/tools`.

## When to use

- Use for user-created Python tools that are registered in the custom tool manifest.
- Use `discover_tools` or `list_tools` first if you do not know the exact tool name.
- Do not use this for built-in AuraGo tools or registered skills. Use the direct tool call, `invoke_tool`, or `execute_skill` as indicated by `discover_tools`.

## Parameters

- `name` (string, required): Custom tool filename or manifest name.
- `args` (array of strings, optional): Positional command-line arguments.
- `params` (object, optional): Structured parameters. AuraGo forwards this as one JSON argument.
- `background` (boolean, optional): Run in the background.
- `vault_keys` (array, optional): Vault keys to inject into the Python process when allowed.
- `credential_ids` (array, optional): Credential IDs to inject when allowed.

## Examples

```json
{"name":"cleanup_reports.py","args":["--days","30"]}
```

```json
{"name":"weather_helper.py","params":{"city":"Berlin","units":"metric"}}
```

## Notes

`run_tool` requires `agent.allow_python`. If Python execution is disabled, the call returns a permission error.
