# SPDX-License-Identifier: AGPL-3.0-or-later
# AuraGo Store launcher for Velxio's MCP server; contract: "Velxio" in internal/desktopstore/AGENTS.md.
import hmac
import os
import sys
import time
import urllib.request

sys.path.insert(0, "/app")
os.chdir("/app")

token = os.environ.get("AURAGO_VELXIO_MCP_TOKEN", "")
port = int(os.environ.get("AURAGO_VELXIO_MCP_PORT", "8002"))
if not token:
    print("aurago-velxio-mcp: no token, MCP server not started", file=sys.stderr)
    while True:
        time.sleep(3600)

# Importing app.mcp.server initialises arduino-cli (config + core install).
# Wait until the image's own entrypoint has finished that and started the
# backend, so the two never install cores at the same time.
deadline = time.monotonic() + 1800
while True:
    try:
        with urllib.request.urlopen("http://127.0.0.1:8001/health", timeout=3) as r:
            if r.status == 200:
                break
    except Exception:
        pass
    if time.monotonic() > deadline:
        print("aurago-velxio-mcp: backend not healthy, giving up", file=sys.stderr)
        sys.exit(1)
    time.sleep(2)

from app.mcp.server import mcp  # noqa: E402
from mcp.server.transport_security import TransportSecuritySettings  # noqa: E402

# The bearer token replaces FastMCP's localhost-only Host allowlist, which
# would reject AuraGo reaching the server by container name.
mcp.settings.transport_security = TransportSecuritySettings(
    enable_dns_rebinding_protection=False
)
mcp.settings.stateless_http = True
mcp.settings.json_response = True
inner = mcp.streamable_http_app()
expected = ("Bearer " + token).encode()


async def guarded(scope, receive, send):
    if scope["type"] == "lifespan":
        await inner(scope, receive, send)
        return
    if scope["type"] != "http":
        if scope["type"] == "websocket":
            await send({"type": "websocket.close", "code": 1008})
        return
    if scope["type"] == "http":
        auth = b""
        for key, value in scope.get("headers", []):
            if key == b"authorization":
                auth = value
                break
        if not hmac.compare_digest(auth, expected):
            await send(
                {
                    "type": "http.response.start",
                    "status": 401,
                    "headers": [(b"content-type", b"text/plain")],
                }
            )
            await send({"type": "http.response.body", "body": b"unauthorized"})
            return
    await inner(scope, receive, send)


import uvicorn  # noqa: E402

print("aurago-velxio-mcp: listening on port %d" % port, file=sys.stderr)
uvicorn.run(guarded, host="0.0.0.0", port=port, log_level="warning")
