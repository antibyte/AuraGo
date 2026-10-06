# CommandCode workspace and preview

CommandCode runs in its own managed Store container. Project files in
`/workspace` are shared with AuraGo Files at `Shared/CommandCode`.

The app opens CommandCode beside a browser preview. **Terminal** shows or hides
the shell drawer underneath; **+** adds a shell. Each shell starts in
`/workspace`. Change into your project directory there before running commands.
Hiding the drawer keeps its sessions and foreground processes connected. Closing
a shell tab, restarting it or closing the app window closes the corresponding
terminal connection. Use the restart control in the appropriate pane after a
disconnection. Copy and paste controls belong to their own pane.

Start a web project's development server, for example:

```sh
cd /workspace/my-project
npm run dev -- --host 0.0.0.0 --port 5173
```

The preview gateway occupies container port 80 and checks port 5173 by default.
It can discover servers on 5173, 4173, 3000, 3001, 5174, 8080 and 8000. Run
`preview-port 3000` in another shell to select that port explicitly. The command
selects a destination; the project server must already be running. HTTP and
WebSocket traffic pass through the existing authenticated preview gateway.

The container automatically imports a short environment guide from
`/usr/local/share/aurago/commandcode-preview.md` into
`~/.commandcode/AGENTS.md`. Existing personal and project instructions are kept.
CommandCode receives the guide through its normal memory mechanism, including
when opened inside a project subdirectory. Existing installations need the new
published CommandCode image and a Store update for this guide; the drawer ships
with AuraGo's versioned Web UI resources.

Local checks: `go test ./internal/desktopstore -run TestCommandCode` (Bash needed
for the memory initialization check), `go test ./ui -run TestDesktopStoreTerminal`
and `npm run check:ui`. Set `AURAGO_RUN_BROWSER_SMOKE=1` for the browser checks;
they use real xterm with simulated container sockets and preview readiness.
Live CommandCode/provider and container acceptance require the installed image.
