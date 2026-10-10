# Layerling on the Virtual Desktop

Layerling is an optional, locally shipped browser CAD editor. Enable it in
**Configuration → Virtual Desktop → Layerling**. No Docker container, Node
server, model provider or separately installed skill is needed at runtime.
Node 24.15.0 is used only when rebuilding the pinned vendor assets.

The Desktop frame follows AuraGo's language and theme. The CAD editor uses
German for a German Desktop and English for all other Desktop languages.
Open it from the Creative category. New windows start maximized, are loaded
on demand and participate in Desktop session restoration.

## Files

Save projects as `.lyl`; this is the unchanged Layerling project format.
The suggested folder is `Documents/Layerling`, created on the first save.
STL, OBJ, 3MF, STEP/STP and SVG can be opened with Layerling from the file
manager. Their existing default applications are preserved. Export STL, OBJ,
3MF, STEP or PNG to a Desktop folder. **Browser download** is a separate action.

Saving checks the version read when the project was opened. If another window
changed the file, choose Replace, Save a copy or Cancel. Saving is atomic;
Desktop size/path/readonly restrictions and Notes protection still apply.
Unsaved edits keep a browser-local recovery draft. Closing waits for saving or
asks whether to retain a recoverable draft. A stale recovery is saved to a new
file. Use Recover to choose drafts from closed windows; each window keeps its
own draft. Keep the browser's site data if you need an unsaved recovery draft.
CAD archives are checked before reaching the editor: at most 10,000 entries
and 256 MiB expanded data, including actual decompression/CRC checks. The
Desktop file-size limit (50 MiB by default) also applies. Archive paths and
symlinks are rejected. Read-only access cannot modify or save the model.

## Agent access

The default is **Off**, independently of the integration's activation switch.
**Read** permits inspection and previews. **Read and write** also permits model
edits and Desktop file writes. Desktop agent access and readonly remain upper
bounds. Window AI chat supplies the exact connected editor ID. General Desktop
chat can list eligible editors; it must choose explicitly when ambiguous.

Connections and commands belong to the authenticated Desktop session and a
single editor document. Navigation/closing revokes its channel. Disconnection,
timeout or permission revocation can leave a command's outcome uncertain, so
the agent never automatically repeats it. Reopen, inspect and reconcile first.
Previews and large scene results remain outside the model context. Their
artifact links expire after five minutes and require the owning session.

Version 1 requires an open browser editor. It does not provide headless CAD,
collaborative simultaneous editing or direct printer control.

## Packaging and licensing

Upstream is Layerling **1.57.0**, commit
`25f12513802e322cdccecaf138f874ff526f331d`, licensed **AGPL-3.0-only**.
The editor's Source code and AGPL links provide the corresponding source archive
including the integration changes, dependency lockfile, license and build guide.
Build details and upstream archive checksum: `scripts/layerling/README.md`.
Browser assets live under `ui/js/vendor/layerling` and are included in AuraGo's
verified external web resource set. Asset packaging and deployment follow
`documentation/web-assets.md`; a plain backend build is recovery-only.

The fixed editor route is `/api/desktop/layerling/ui/`. All fonts, workers,
WASM and guide files are local. AuraGo disables Layerling's service worker and
HTTP MCP registration and uses the existing document-bound MessageChannel.
