# AuraGo development environment

You are running inside AuraGo's managed CommandCode container. The persistent
working directory is `/workspace`, shown in AuraGo Files as `Shared/CommandCode`.
Keep project files there; project-specific instructions still apply.

AuraGo already provides a browser preview beside this conversation. To preview
a web app, start its development server inside this container and keep it running
in the background. For example, from the project's directory:

```sh
npm run dev -- --host 0.0.0.0 --port 5173
```

The preview gateway uses container port 80; leave that port free. Its default
target is `http://127.0.0.1:5173`. It checks that target first, then looks for a
server on 5173, 4173, 3000, 3001, 5174, 8080 or 8000. If the project uses a
different port, run `preview-port <port>` (for example `preview-port 3000`).
This selects the target; it does not start the server. With multiple servers,
select the intended one explicitly. Both HTTP and WebSocket traffic are proxied.
The user opens the app through AuraGo's preview, not their computer's localhost.
Do not ask them to publish a container port or open port 80 on the host.

For static files, you can run `python3 -m http.server 8000 --bind 0.0.0.0`
from the directory containing the page, then `preview-port 8000`.
Check that the server responds before telling the user the preview is ready.
AuraGo detects readiness automatically; its preview button can hide/show the pane.

The Terminal button opens a separate shell below CommandCode, initially in
`/workspace`. Shells and CommandCode share files, but not their current directory;
use `cd /workspace/<project>` in a shell when needed. Hiding the terminal leaves
its session running. Closing a shell tab, restarting it or closing the app window
ends that shell connection, so keep that in mind for foreground dev servers.
