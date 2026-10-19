# AuraGo Architecture

AuraGo runs a portable Go backend with embedded SQLite. The complete Web UI is a
verified, version-bound local resource set served by the backend; only the small
recovery/login page stays embedded. Optional integrations and execution runtimes
have their own dependencies.

## System overview

[![AuraGo architecture: clients and message or automation inputs reach the Go backend, which connects models, local data, gated tools, optional runtimes and home-lab or cloud integrations](../assets/readme/system-wiring.svg)](../assets/readme/system-wiring.svg)

[Open the SVG](../assets/readme/system-wiring.svg) ·
[Editable, offline HTML source](architecture.html)

The figure shows primary calls and dependencies. Results return along the same
paths. The dashed rectangle marks the main Go process, not a shared permission
scope. The inputs group names entry points: channel adapters and scheduling run
inside the backend. Local persistence is drawn separately to distinguish stored
data from the code that reads it. Server-side apps and services also use these
stores; the figure omits those secondary connections.

## Components

| Component | Responsibility | Main source |
| --- | --- | --- |
| Web UI and AgoDesk | Chat, dashboard, desktop and native-client access | [`ui/`](../ui/), [AgoDesk](https://github.com/antibyte/agodesk) |
| HTTP server | Authentication, REST API, SSE, WebSocket bridges and verified UI serving | [`internal/server/`](../internal/server/), [`internal/webassets/`](../internal/webassets/) |
| Messages and jobs | Channel input, missions, scheduled tasks and webhooks; each retains its admission rules | [`internal/server/`](../internal/server/), [`internal/telegram/`](../internal/telegram/), [`internal/discord/`](../internal/discord/), [`internal/planner/`](../internal/planner/), [`internal/webhooks/`](../internal/webhooks/) |
| Agent runtime | Context assembly, model/tool loop, memory retrieval, personality, co-agents and budgets | [`internal/agent/`](../internal/agent/), [`internal/prompts/`](../internal/prompts/), [`internal/budget/`](../internal/budget/) |
| Model access | Configured providers, local or hosted models, failover and optional task routing | [`internal/llm/`](../internal/llm/), [`internal/localllm/`](../internal/localllm/) |
| Tools and services | Allowed tool execution, integration clients, Guardian checks and runtime gates | [`internal/agent/agent_loop_tools.go`](../internal/agent/agent_loop_tools.go), [`internal/tools/`](../internal/tools/), [`internal/security/`](../internal/security/) |
| Local persistence | SQLite conversation/core memory and knowledge graph, chromem-go vectors, app databases and files | [`internal/memory/`](../internal/memory/), [`cmd/aurago/main.go`](../cmd/aurago/main.go) |
| Optional runtimes | Managed services, browser VMs, speech engines and native media workers | [`internal/virtualcomputers/`](../internal/virtualcomputers/), [`internal/speechlab/`](../internal/speechlab/), [`internal/videostudio/`](../internal/videostudio/) |
| Integrations | Enabled home-lab, device, messaging and cloud connections | [Integration manual](manuals/en/08-integrations.md) |

## A turn through the system

1. A client request, channel message or authorized automation run enters through
   its own authentication, trust and scheduling rules.
2. The agent builds context from the request, relevant history, core facts,
   retrieved memories, personality guidance and available tool definitions.
3. The configured model returns an answer or native tool calls. Provider
   failover and budget accounting accompany model requests.
4. Tool calls pass their runtime and integration gates. Service clients retrieve
   needed credentials from the encrypted vault, then operate on the configured
   local or remote target. Tool results return to the loop for the next round.
5. Progress and the final answer return through the originating channel. The
   runtime persists conversation state and applies its memory lifecycle.

The browser can also use app APIs directly: opening a file or changing a setting
does not require an LLM turn. Co-agents and background services have their own
limits and lifecycle. Optional task routing and the Decisions service extend the
runtime when enabled; both default off.

## Boundaries that matter

- **Browser resources:** the full UI is served from a verified local resource set
  pinned to the executable. It is neither a new CDN dependency nor an embedded
  full SPA. See [external web resources](web-assets.md).
- **Memory:** conversation history, core facts, semantic retrieval and the
  knowledge graph have different roles. The diagram groups their storage; it
  does not imply every record enters every prompt. See the
  [memory manual](manuals/en/09-memory.md).
- **Credentials and permissions:** service clients use the vault for secrets.
  Personality does not grant permissions. Channels, integrations and workers
  retain their scopes; shell, Python, writes, network and remote execution have
  separate gates. See the [security manual](manuals/en/14-security.md).
- **Deployment:** the Go backend is CGO-free. Python skills, Docker services,
  local inference, browser VMs and media features can require additional
  runtimes, downloads or hardware. See [dependencies](dependencies.md) and
  [Docker installation](docker_installation.md).

## Diagram maintenance

The README and English/German manuals share
[`assets/readme/system-wiring.svg`](../assets/readme/system-wiring.svg).
Edit [`architecture.html`](architecture.html), then export its inline SVG with
the Diagram Design skill's `scripts/export_svg.py` and `--system-fonts`.
Keep the HTML and SVG in sync. Check labels, connections and contrast at full
size, at a 900px README width, and at 360px. Essential explanations stay in
Markdown for narrow screens and assistive technology.

Startup wiring is composed in [`cmd/aurago/main.go`](../cmd/aurago/main.go).
The figure is a component overview rather than a deployment topology or an
exhaustive list of integrations.
