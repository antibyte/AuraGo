# Homepage — Python HTTP Server Fallback (`homepage_local_server`)

Local Python HTTP server fallback for when Docker is unavailable. This is a **conditional tool** only enabled when `homepage.allow_local_server: true`.

> **Security Warning**: Local file system is exposed via HTTP server. Process runs with agent's privileges (no container isolation). Only enable in trusted environments.

## Fallback vs Docker Mode

| Feature | Docker mode | Python fallback |
|---------|-------------|----------------|
| Node.js / npm | Available | Not available |
| Lighthouse / Playwright | Available | Not available |
| Automatic HTTPS (Caddy) | Available | Not available |
| File read / write / list | Available | Available |
| Static file hosting | Available | Available |

## Configuration

```yaml
homepage:
  enabled: true
  allow_local_server: true  # Enable Python fallback when Docker unavailable
```

## Notes

- **Security risk**: Local file system exposed via HTTP server without container isolation
- **Limited functionality**: No container builds, no Lighthouse testing, no automatic HTTPS in fallback mode
- **Troubleshooting**: If Python fallback fails, neither Docker nor Python is available — install Python 3
- **Auto-restart**: Python server does not auto-restart — run `webserver_start` again if status shows `"running": false`
