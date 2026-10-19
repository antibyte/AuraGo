---
id: sandbox
tags: [core]
priority: 100
conditions: ["sandbox_enabled"]
---

# Sandbox Code Execution

`execute_sandbox` runs source in a fresh supervised container. Foreground
`execute_python`, `execute_shell`, saved tools and supported scripts use the
same isolation by default. An unavailable supervisor is a setup error; code
never automatically falls back to the host.

```json
{"action":"execute_sandbox","code":"print('Hello, World!')","sandbox_lang":"python"}
```

Python and POSIX shell use a reviewed digest-pinned official Python image.
Other languages require a configured digest-pinned `sandbox.images` entry.
Required libraries must already be present in a reviewed image: a nonempty
`libraries` request is refused. Do not request package installation or network
access from generic workers.

Workers receive no Vault credentials, host files, Docker socket or control
network. Fresh scratch files disappear with the worker. Resource and output
limits apply to each run. Use native authorized tools to read/write workspace
files or access integrations. Foreground Python can explicitly request a scoped
Tool Bridge when enabled; its supervisor brokers calls under the originating
run's current permissions, without giving the worker a token or network access.

The supervisor requires `llm-sandbox[mcp-docker]==0.3.45` (or `mcp-podman`).
An administrator provisions the package and any additional pinned images.
See `documentation/isolated-executor.md` for operator configuration and limits.

Explicit unsafe-host execution remains a compatibility opt-in. Generic
`execute_sandbox` never uses that opt-in as a fallback. Default background shell,
Python and saved-tool requests return a durable isolated task ID. Review output,
status and cancellation in System Tasks; completion notifications are enabled.
Opted-in host background process entry points retain their managed PID path.
