# AuraGo FAQ

Back to the [handbook hub](../README.md) · [English overview](README.md)

<p align="center">
  <a href="../images/manual-hero.webp"><img src="../images/manual-hero.webp" width="480" alt="AuraGo gopher with a handbook"></a>
</p>

## Table of Contents

1. [Start](#start)
2. [Security](#security)
3. [Toys](#toys)
4. [When It Creaks](#when-it-creaks)

---

## Start

### Fastest Way In?
Linux: the one-liner in [Chapter 2](02-installation.md), then [Quick start](03-quickstart.md). Docker: clone the repo and `docker compose up -d` — do not curl `config.yaml` from GitHub; that file is not in the repository.

### Do I Need Docker?
No. The core is a binary. Docker is the cleaner isolation and brings sidecars. See the [Docker guide](../../docker_installation.md).

### Why Do I Only Get a Recovery/Login Page?
The full UI is a **versioned resource set**, not the whole binary. An unpinned `go build` or a missing `aurago-web-assets-*.tar.gz` lands in recovery. Run `./aurago --check-assets`, `--install-assets` if needed, or rebuild with the asset flags. [Web assets](../../web-assets.md).

### Which URL?
Default is **http://127.0.0.1:8088** / **http://localhost:8088**. Docker and LAN often bind `0.0.0.0:8088`.

## Security

### Where Do API Keys Go?
The vault. Not markdown, git, or plaintext exports. [Chapter 14](14-security.md).

### Can I Put It on the Internet?
Only with HTTPS, a login, and ideally 2FA. A VPN is the calmer option.

### Why Can’t the Agent Read `config.yaml`?
On purpose. File tools live in `agent_workspace`. `../../config.yaml` and `data/` are jailed. Ask for system information or put a file in the workspace.

### Where Are the Logs?
`log/aurago.log` and `log/web_access.log`. There is no `supervisor.log`.

## Toys

### How Many Tools Are There Really?
The catalog is large and **feature-gated**. The manuals say “100+” because that is the documented native-tool ballpark. Your install only shows what Config and integrations allow. [Chapter 6](06-tools.md) · [Chapter 22](22-internal-tools.md).

### Telegram, Discord, MeshCore, SIP?
All in [Chapter 8](08-integrations.md). Telegram extra: [telegram_setup.md](../../telegram_setup.md). MeshCore: [meshcore-en.md](../../meshcore-en.md).

### Is There a Desktop App?
Yes. [AgoDesk](https://github.com/antibyte/agodesk) for Windows and Linux. That is not the web UI.

### Where Did Eggs and Nests Go?
[Invasion Control](12-invasion.md) — not under Mission Control. Missions are scheduled chats; Invasion is remote deploy.

### Does AuraGo Support MCP?
Yes, client and server, behind `agent.allow_mcp`. [Chapter 8](08-integrations.md).

## When It Creaks

### UI Loads, Actions Die?
Logs, Danger Zone toggles, provider key. Then [Chapter 16](16-troubleshooting.md).

### What Is Authoritative?
The code and `config_template.yaml`. This handbook follows them, not the other way around.
