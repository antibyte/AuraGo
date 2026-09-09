# AuraGo FAQ

Back to the [handbook hub](../README.md) · [English overview](README.md)

<p align="center">
  <a href="../images/manual-hero.webp"><img src="../images/manual-hero.webp" width="480" alt="AuraGo gopher with a handbook"></a>
</p>

## Start

### Fastest way in?
Linux: the one-liner in [Chapter 2](02-installation.md), then [Quick start](03-quickstart.md). Docker: clone the repo and `docker compose up -d` — do not curl `config.yaml` from GitHub; that file is not in the repository.

### Do I need Docker?
No. The core is a binary. Docker is the cleaner isolation and brings sidecars. See the [Docker guide](../../docker_installation.md).

### Why do I only get a recovery/login page?
The full UI is a **versioned resource set**, not the whole binary. An unpinned `go build` or a missing `aurago-web-assets-*.tar.gz` lands in recovery. Run `./aurago --check-assets`, `--install-assets` if needed, or rebuild with the asset flags. [Web assets](../../web-assets.md).

### Which URL?
Default is **http://127.0.0.1:8088** / **http://localhost:8088**. Docker and LAN often bind `0.0.0.0:8088`.

## Security

### Where do API keys go?
The vault. Not markdown, git, or plaintext exports. [Chapter 14](14-security.md).

### Can I put it on the internet?
Only with HTTPS, a login, and ideally 2FA. A VPN is the calmer option.

### Why can’t the agent read `config.yaml`?
On purpose. File tools live in `agent_workspace`. `../../config.yaml` and `data/` are jailed. Ask for system information or put a file in the workspace.

### Where are the logs?
`log/aurago.log` and `log/web_access.log`. There is no `supervisor.log`.

## Toys

### How many tools are there really?
The catalog is large and **feature-gated**. The manuals say “100+” because that is the documented native-tool ballpark. Your install only shows what Config and integrations allow. [Chapter 6](06-tools.md) · [Chapter 22](22-internal-tools.md).

### Telegram, Discord, MeshCore, SIP?
All in [Chapter 8](08-integrations.md). Telegram extra: [telegram_setup.md](../../telegram_setup.md). MeshCore: [meshcore-en.md](../../meshcore-en.md).

### Is there a desktop app?
Yes. [AgoDesk](https://github.com/antibyte/agodesk) for Windows and Linux. That is not the web UI.

### Where did Eggs and Nests go?
[Invasion Control](12-invasion.md) — not under Mission Control. Missions are scheduled chats; Invasion is remote deploy.

### Does AuraGo support MCP?
Yes, client and server, behind `agent.allow_mcp`. [Chapter 8](08-integrations.md).

## When it creaks

### UI loads, actions die?
Logs, Danger Zone toggles, provider key. Then [Chapter 16](16-troubleshooting.md).

### What is authoritative?
The code and `config_template.yaml`. This handbook follows them, not the other way around.
