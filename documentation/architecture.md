# AuraGo Architecture

> **Note:** This document describes AuraGo v2.x architecture. Diagrams are rendered automatically on GitHub.

## System Overview

```mermaid
blockdiag {
   # === STYLING ===
   default_fontsize = 11
   default_shape = roundedbox
   span_width = 2

   # === CORE LAYER ===
   web_ui [label = "Web UI\n(embedded SPA)"];
   server [label = "Server\n(REST + SSE)"];
   agent_loop [label = "Agent Loop\n(Tool Dispatch)"];
   llm_client [label = "LLM Client\n(OpenAI-compatible)"];

   # === MEMORY LAYER ===
   stm [label = "STM\n(SQLite sliding window)"];
   ltm [label = "LTM\n(Chromem-go vector DB)"];
   knowledge_graph [label = "Knowledge Graph\n(SQLite + FTS5)"];
   vault [label = "Vault\n(AES-256-GCM)"];
   core_memory [label = "Core Memory\n(Permanent facts)"];

   # === TOOLS ===
   tool_dispatcher [label = "Tool Dispatcher\n(90+ built-in tools)"];

   # === INTEGRATIONS ===
   telegram [label = "Telegram"];
   discord [label = "Discord"];
   fritzbox [label = "Fritz!Box"];
   home_assistant [label = "Home Assistant"];
   mqtt [label = "MQTT"];
   proxmox [label = "Proxmox"];
   jellyfin [label = "Jellyfin"];
   tailscale [label = "Tailscale"];
   docker [label = "Docker"];

   # === LAYOUT ===
   web_ui -> server -> agent_loop -> llm_client;

   agent_loop -> tool_dispatcher;
   agent_loop -> { stm, ltm, knowledge_graph, vault, core_memory };

   tool_dispatcher -> { telegram, discord, fritzbox, home_assistant, mqtt, proxmox, jellyfin, tailscale, docker };

   # === GROUPING ===
   group core [label = "Core", color = "#e1e4e8"];
   group memory [label = "Memory & Security", color = "#e1e4e8"];
   group tools [label = "Tool Layer", color = "#e1e4e8"];
   group integrations [label = "Integrations", color = "#e1e4e8"];
}
```

## Component Description

### Core Layer
- **Web UI** — Embedded single-page application served via `go:embed`
- **Server** — HTTP/HTTPS server with REST API and SSE streaming
- **Agent Loop** — Core orchestration: message handling, LLM calls, tool dispatch, co-agents
- **LLM Client** — OpenAI-compatible client with failover, retry, and pricing

### Memory & Security
- **STM** — Short-term memory: SQLite sliding-window conversation context
- **LTM** — Long-term memory: Chromem-go vector database for semantic search
- **Knowledge Graph** — Entity-relationship store for structured facts
- **Core Memory** — Permanent facts always included in context
- **Vault** — AES-256-GCM encrypted secret storage

### Tool Layer
- **Tool Dispatcher** — Routes tool calls to 90+ built-in implementations (Shell, Python, Filesystem, HTTP, Docker, SSH, Cron, and more)

### Integrations
- Telegram, Discord, Fritz!Box, Home Assistant, MQTT, Proxmox, Jellyfin, Tailscale, Docker

## Notes

- AuraGo ships as a single Go binary — no external runtime dependencies
- All integrations are optional and configured via `config.yaml`
- See `prompts/tools_manuals/` for detailed tool documentation