# AuraGo System Block Diagram — Design Spec

**Date:** 2026-04-07
**Status:** Draft
**Target:** `docs/architecture.md`

## Goal

Create a Mermaid `blockDiagram` that shows AuraGo's architecture at a glance, suitable for embedding in `docs/architecture.md`. The diagram must render on GitHub (which supports Mermaid natively in markdown).

## Design Decisions

### Style: Component Diagram
- Boxen mit Namen und kurzer Beschreibung
- Keine Flow-Charts oder Sequenzen — nur Struktur/Componenten
- Zeigt Relationships über hierarchische Gruppierung

### Detail: Full (~12-15 Komponenten)
Alle Layer inklusive Integrationen, da `docs/architecture.md` ein umfassendes Dokument ist.

### Layout: Grid/Table
Drei horizontale Zeilen: Core → Memory/Tools → Integrations

### Ansatz: Manuell geschrieben
Kein Code-Analysis-Tool. Architektur ist stabil genug.

---

## Proposed Structure

```
┌─────────────────────────────────────────────────────────────────────┐
│                           AuraGo Binary                             │
│                                                                     │
│  ┌──────────┐   ┌──────────┐   ┌──────────┐   ┌──────────────┐      │
│  │  Web UI  │───│  Server  │───│  Agent   │───│     LLM      │      │
│  │ (embedded│   │  REST/SSE│   │   Loop   │   │    Client    │      │
│  │    SPA)  │   │          │   │          │   │  (OpenAI-    │      │
│  └──────────┘   └──────────┘   └──────────┘   │   compatible)│      │
│                                                └──────────────┘      │
└─────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────┐
│                         Memory & Security                            │
│                                                                     │
│  ┌────────────┐  ┌──────────────┐  ┌──────────────┐  ┌─────────┐ │
│  │ STM Memory │  │  LTM Memory  │  │Knowledge Graph│  │  Vault  │ │
│  │  (SQLite)  │  │  (Chromem-go) │  │   (SQLite)    │  │AES-256  │ │
│  │  Context   │  │   Embeddings  │  │   Entities    │  │  GCM    │ │
│  └────────────┘  └──────────────┘  └──────────────┘  └─────────┘ │
└─────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────┐
│                       Tool Dispatcher (90+)                          │
│                                                                     │
│  ┌─────────┐ ┌─────────┐ ┌──────────┐ ┌────────┐ ┌────────┐        │
│  │ Shell   │ │ Python  │ │ Filesystem│ │ HTTP   │ │ Docker  │ ...   │
│  └─────────┘ └─────────┘ └──────────┘ └────────┘ └────────┘        │
│  ┌─────────┐ ┌─────────┐ ┌──────────┐ ┌────────┐                   │
│  │   SSH   │ │  Cron   │ │ Secrets   │ │  MCP   │    ...           │
│  └─────────┘ └─────────┘ └──────────┘ └────────┘                   │
└─────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────┐
│                           Integrations                               │
│                                                                     │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐  │
│  │ Telegram │ │ Discord  │ │ Fritzbox │ │Home Ass.  │ │   MQTT   │  │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘  │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐                │
│  │ Proxmox  │ │ Jellyfin │ │Tailscale │ │  S3/MinIO │              │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘                │
└─────────────────────────────────────────────────────────────────────┘
```

## Implementation

1. **File:** `docs/architecture.md` — neues Dokument erstellen
2. **Mermaid blockDiagram** — dreispaltiges Layout mit gruppierten Blöcken
3. **Farbschema:** GitHub-kompatibel, einfache Farben
4. **Beschriftung:** Jede Box mit Name und Kurzbeschreibung

## Mermaid Syntax Ansatz

```mermaid
blockdiag
   blockdiag {
      # Core
      web_ui [label = "Web UI\n(embedded SPA)"];
      server [label = "Server\n(REST + SSE)"];
      agent_loop [label = "Agent Loop\n(Tool Dispatch)"];
      llm_client [label = "LLM Client\n(OpenAI-compatible)"];

      # Memory
      stm [label = "STM\n(SQLite)"];
      ltm [label = "LTM\n(Chromem-go)"];
      kg [label = "Knowledge Graph\n(SQLite)"];
      vault [label = "Vault\n(AES-256-GCM)"];

      # Tools
      tool_dispatcher [label = "Tool Dispatcher\n(90+ tools)"];

      # Integrations
      telegram [label = "Telegram"];
      discord [label = "Discord"];
      fritzbox [label = "Fritzbox"];
      home_assistant [label = "Home Assistant"];
      mqtt [label = "MQTT"];
      proxmox [label = "Proxmox"];

      # Layout
      web_ui -> server -> agent_loop -> llm_client;
      agent_loop -> tool_dispatcher;
      tool_dispatcher -> { telegram, discord, fritzbox, home_assistant, mqtt, proxmox };
      agent_loop -> { stm, ltm, kg, vault };
   }
```

## GitHub Compatibility

- GitHub unterstützt Mermaid nativ in `.md` Dateien mit ```mermaid blocks
- Keine externe deps nötig
- Dark/Light mode funktioniert automatisch via GitHub

## Scope

- Nur das Diagramm erstellen, keine weitere Dokumentation
- Diagramm zeigt Architektur zum Zeitpunkt 2026-04-07
- Optional: kleines Intro vor dem Diagramm

## Notes

- Die 90+ Tools werden als eine Box "Tool Dispatcher" zusammengefasst
- Co-Agents sind Teil des Agent Loops
- Lifeboat (Self-Update) als Sub-Komponente des Agent