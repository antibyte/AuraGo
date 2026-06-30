# Show HN: AuraGo – An open-source, privacy-first AI agent framework for homelabs

**Target Audience:** Hacker News (Show HN)
**Suggested Title:** `Show HN: AuraGo – Open-source, privacy-first AI agent framework built for homelabs`

---

Hi HN,

I’m the creator of AuraGo (https://github.com/antibyte/AuraGo). I wanted to share a project I’ve been working on to solve a personal frustration: I wanted an autonomous AI assistant to manage my home servers, monitor Docker containers, and control Home Assistant, but I was absolutely unwilling to send my local system configurations, SSH keys, or network logs to a third-party cloud API.

AuraGo is a self-hosted, open-source (AGPLv3) agentic framework purpose-built for homelabs and self-hosters. It lets you deploy capable AI agents directly on your own infrastructure—keeping your data entirely within your local network.

### 🔌 Why a homelab-specific agent?

Most general-purpose agents (like AutoGPT or standard ChatGPT custom GPTs) are designed for general web-based tasks. They lack native, secure adapters for local hardware. Writing custom wrappers is tedious, and giving an LLM full root shell access on your primary server without safety guards is a recipe for disaster.

AuraGo is designed from the ground up for homelab orchestrations. It bridges LLMs directly to your home stack using over 100+ native secure adapters, allowing the agent to:
- **Docker**: Stop/start/restart containers, inspect logs, and scale Docker Compose stacks.
- **Proxmox**: Manage VM/LXC lifecycles, monitor node memory/CPU, and take backups or snapshots.
- **Home Assistant**: Toggle entities, poll sensors, and trigger home scenes with explicit read/write guardrails.
- **TrueNAS**: Track storage pool health, datasets, and monitor share states.
- **Local Utilities**: Perform network diagnostics (mDNS/UPnP scans, pings, local port scans) and run sandboxed Python scripts.

---

### 🛡️ How we handle Security (The "LLM Guardian")

Connecting an LLM to your home infrastructure is naturally a massive security risk. We designed AuraGo with security-first principles to keep your servers safe:

1. **The LLM Guardian**: To prevent indirect prompt-injection (e.g. your agent parses an incoming server alert email containing instructions to `rm -rf /`), we run a secondary, specialized local security model. This "Guardian" scans the outputs and inputs of high-risk tool calls in real-time, blocking malicious payloads before they hit your execution layer.
2. **AES-256 Vault**: All integration API keys and SSH credentials are encrypted locally inside an AES-256-GCM Vault. The agent never has direct read access to raw keys; it only refers to them via metadata handles.
3. **Hard Toggle "Danger Zone"**: Every high-risk feature—filesystem writes, root shell access, Python sandboxing, external network requests—can be completely disabled via environment-level flags.

---

### ⚙️ Under the Hood (No Heavy Python Footprint)

Most AI agent frameworks are written in Python and come with massive node sizes, complex dependency hells, and slow startup times. 

We chose **Go** for AuraGo:
- **Single Binary**: It compiles into a single, lightweight binary with zero external OS-level dependencies.
- **Modest Footprint**: It runs comfortably on a Raspberry Pi 4/5 or an old NUC, drawing minimal RAM.
- **Fast Local Memory**: For long-term memory and RAG, we use `chromem-go` (a vector database written in pure Go) and modernc's pure Go SQLite driver for conversation logs—completely avoiding heavy Cgo bindings.
- **Vanilla Embedded UI**: The Web UI setup wizard and chat console are written in vanilla JS, utilizing Rollup, CodeMirror, and Quill, embedded directly into the Go binary using `go:embed` for single-port delivery (port 8088).

---

### 🚀 Get Started Locally

We provide an automated, Docker-integrated one-liner installer that configures directories, sets up secure random master keys, and initializes a systemd service:

```bash
curl -fsSL https://raw.githubusercontent.com/antibyte/AuraGo/main/install.sh | bash
```

Once installed, just run `./start.sh`, navigate to **http://localhost:8088**, and follow the interactive Setup Wizard to connect your preferred local model (via Ollama/Llama.cpp) or cloud provider (OpenAI/OpenRouter).

### 🤝 Open Source & License

The core framework is fully public, reproducible, and licensed under **AGPLv3**. We are actively looking for contributors, especially homelab enthusiasts who run custom hypervisors or unique network configurations and want to write new tool integrations.

I’d love to hear your feedback, architectural suggestions, or questions on how we handle security and tool-filtering in local networks!

Check out the repo here: [https://github.com/antibyte/AuraGo](https://github.com/antibyte/AuraGo)
