---
id: "tools_homepage_local_server"
tags: ["conditional"]
priority: 31
conditions: ["homepage_allow_local_server"]
---
## Homepage — Python HTTP Server Fallback (Danger Zone)

Local Python execution is **enabled** (`homepage.allow_local_server: true`). When Docker is unavailable, the agent can start a Python HTTP server directly on the host.

### Security Risks
- Local file system exposed via HTTP server
- Process runs with agent's privileges (no container isolation)
- Only enable in trusted environments; ensure firewall blocks external access to the server port

### Fallback vs Docker Mode

| Feature | Docker mode | Python fallback |
|---|---|---|
| Node.js / npm | ✅ | ❌ |
| Lighthouse / Playwright | ✅ | ❌ |
| Automatic HTTPS (Caddy) | ✅ | ❌ |
| File read / write / list | ✅ | ✅ |
| Static file hosting | ✅ | ✅ |

### Fallback Behavior

When Docker is unavailable the `init` operation starts a Python HTTP server instead of the dev container:
```json
{"action": "homepage", "operation": "init"}
```

`webserver_start` also falls back to Python HTTP server automatically when Docker is unavailable.

### Troubleshooting Fallback Mode

**"Python fallback failed" Error**

Neither Docker nor Python is available. Install Python 3:
- Debian/Ubuntu: `sudo apt-get install python3`
- RHEL/CentOS: `sudo yum install python3`

**Limited Functionality**

No container builds, no Lighthouse testing, no automatic HTTPS. Workarounds:
- For static sites: use `webserver_start` directly (serves files from workspace)
- For builds: run `npm run build` locally, then use `webserver_start` or `deploy`

**Status Shows `"running": false`**

Server stopped. Run `webserver_start` again — the Python server does not auto-restart.
