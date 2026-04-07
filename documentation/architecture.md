# AuraGo Architecture

> **Note:** This document describes AuraGo v2.x architecture. Diagrams are rendered automatically on GitHub.

## System Overview

```mermaid
flowchart TD
    subgraph Core["🔧 Core"]
        WEB[("Web UI<br/>embedded SPA")]
        SERVER[("Server<br/>REST + SSE")]
        AGENT[("Agent Loop<br/>Tool Dispatch")]
        LLM[("LLM Client<br/>OpenAI-compatible")]
    end

    subgraph Memory["🧠 Memory & Security"]
        STM[("STM<br/>SQLite sliding window")]
        LTM[("LTM<br/>Chromem-go vector DB")]
        KG[("Knowledge Graph<br/>SQLite + FTS5")]
        VAULT[("Vault<br/>AES-256-GCM")]
        CORE[("Core Memory<br/>Permanent facts")]
    end

    subgraph Tools["🔧 Tool Layer"]
        TD[("Tool Dispatcher<br/>90+ built-in tools")]
    end

    subgraph Integrations["🔌 Integrations"]
        TG[("Telegram")]
        DC[("Discord")]
        FB[("Fritz!Box")]
        HA[("Home Assistant")]
        MQ[("MQTT")]
        PX[("Proxmox")]
        JF[("Jellyfin")]
        TS[("Tailscale")]
        DOC[("Docker")]
    end

    WEB --> SERVER --> AGENT --> LLM
    AGENT --> TD
    AGENT --> STM
    AGENT --> LTM
    AGENT --> KG
    AGENT --> VAULT
    AGENT --> CORE
    TD --> TG
    TD --> DC
    TD --> FB
    TD --> HA
    TD --> MQ
    TD --> PX
    TD --> JF
    TD --> TS
    TD --> DOC
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