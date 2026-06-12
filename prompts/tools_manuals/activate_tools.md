# activate_tools

Use `activate_tools` after `discover_tools` finds enabled native tools that are hidden by adaptive filtering and the next LLM request should call them directly.

## Parameters

- `names` (required): Array of exact tool names returned by `discover_tools`. Maximum 8 names.
- `reason` (optional): Short reason for the activation.

## Behavior

The tool returns `activated`, `already_active`, `disabled`, `unknown`, and `next_request: true`. Activated tools are added to the next LLM request in the same agent run and then the activation request is consumed.
