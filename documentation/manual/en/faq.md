# AuraGo Manual FAQ (EN)

Back to [Manual Start Page](../README.md) and [English Manual Overview](README.md).

## General

### What is the fastest way to start AuraGo?
Use the installation flow in [Chapter 2](02-installation.md) and then follow [Chapter 3 Quick Start](03-quickstart.md).

### Do I need Docker?
No. The core agent runs as a single binary. Docker is recommended for isolation and sidecars (for example Gotenberg). See [Docker Installation](../../docker_installation.md).

### How many tools are currently available?
The current platform documents 100+ built-in tools and several integration-specific capabilities. See [Chapter 6: Tools](06-tools.md).

## Security

### Where should I store API keys and passwords?
In AuraGo's encrypted vault. Do not store secrets in markdown docs, commits, or plain config exports. See [Chapter 14: Security](14-security.md).

### Can I run AuraGo internet-facing?
Yes, but only with HTTPS, login protection, and ideally 2FA enabled. See [Chapter 14: Security](14-security.md) and [Installation](02-installation.md).

## Integrations & Features

### Where can I configure Telegram and Discord?
Use [Chapter 8: Integrations](08-integrations.md) and the dedicated [Telegram setup guide](../../telegram_setup.md).

### Is there distributed orchestration support?
Yes. Invasion Control and Remote Control are covered in [Chapter 12](12-invasion.md) and [Chapter 15](15-coagents.md).

### Does AuraGo support MCP?
Yes, both client integration and server mode are available. Check [Chapter 8](08-integrations.md) and [Chapter 7](07-configuration.md).

## Troubleshooting

### The UI loads but actions fail – where should I look first?
Check the runtime logs, danger-zone toggles, and provider credentials. Start with [Chapter 16: Troubleshooting](16-troubleshooting.md).

### A manual section looks outdated – what is authoritative?
Treat the codebase and config schema as source of truth, then update docs accordingly. Start from [Chapter 7: Configuration](07-configuration.md).
