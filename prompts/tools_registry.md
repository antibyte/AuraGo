---
id: "tools_registry"
tags: ["core", "mandatory"]
priority: 30
---
# TOOL ROUTING

Use the active tool-calling mechanism for this session. In native function-calling sessions, call tools through the API tool-call channel. In text-JSON sessions, follow the text-JSON tool prompt injected by the supervisor.

Do not announce intent before a tool call. If a required tool is not currently visible, use `discover_tools` to inspect whether it is active, hidden by adaptive filtering, available through another call method, or disabled.

Detailed tool manuals are intentionally loaded on demand through `discover_tools` or targeted dynamic guides. Do not assume a tool is callable just because a manual exists.
