# activate_tools

Use `activate_tools` only when `discover_tools` explicitly returns `call_method: "activate_tools"`. Treat `call_method` as binding: when it returns `invoke_tool`, call `invoke_tool` immediately instead.

## Parameters

- `names` (required): Array of exact tool names for which `discover_tools` returned `call_method: "activate_tools"`. Maximum 8 names.
- `reason` (optional): Short reason for the activation.

## Behavior

The tool returns `activated`, `already_active`, `disabled`, `unknown`, `required_call_methods`, and `next_request`. A mismatched request is rejected and reports the binding method, such as `invoke_tool` or `direct`. Activated tools are added to the next LLM request in the same agent run and then the activation request is consumed.
