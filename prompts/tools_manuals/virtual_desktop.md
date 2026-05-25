# virtual_desktop

Use `virtual_desktop` to control AuraGo's first-party browser desktop. It can inspect the desktop state, read and write files inside the desktop workspace, create and edit basic Office documents/workbooks, install generated JavaScript apps, pin widgets, open apps, and show desktop notifications.

The desktop workspace is jailed to `virtual_desktop.workspace_dir`. Never place credentials or vault values in generated app files. If an app needs sensitive data, build a small backend or agent-mediated flow that retrieves only the minimum safe result.

Generated app and widget iframes run under Content Security Policies. Generated apps may use common static library CDNs (`cdn.jsdelivr.net`, `cdnjs.cloudflare.com`, `unpkg.com`, `esm.sh`, `cdn.skypack.dev`) plus Google Fonts; this supports browser-game libraries such as Phaser and other frontend-only runtimes. Widgets stay stricter: their `connect-src` allows same-origin requests plus `https://api.open-meteo.com` and `https://geocoding-api.open-meteo.com` for public weather widgets; do not generate widgets that fetch arbitrary third-party APIs directly from the iframe. For other external data, use an agent-mediated or backend-mediated flow and write only the safe result into the desktop workspace.
Generated widgets are sandboxed. Do not navigate `window.top` or `window.parent`, and do not try to reload the desktop shell from widget code; use SDK bridge actions or in-widget state updates instead.

## Operations

- `status` / `bootstrap`: return desktop settings, built-in apps, installed apps, widgets, workspace folders, and `icon_catalog` for generated app icon selection.
- `list_files`: list a workspace directory. Use `path`, for example `Documents`.
- `read_file`: read one text file. Use `path`. Files larger than 8 KB are intentionally truncated; when that happens, do not ask the user for block anchors.
- `search_file`: search one desktop text/code file without returning the whole file. Use `path`, `query`, optional `context_lines`, `max_matches`, and `case_sensitive`.
- `read_file_excerpt`: read a line window from one desktop text/code file. Use `path`, `line_start`, and `line_count`.
- `write_file`: write one text file. Use `path` and `content`. For simple generated HTML apps, prefer `install_app`; if you write non-empty HTML to `Apps/<app_id>.html`, AuraGo also registers/updates a runnable generated app at `Apps/<app_id>/index.html`.
- `patch_file`: edit one desktop text/code file without rereading or replacing the whole file. Use `path` plus `replacements:[{find,replace}]`, `prepend_text`, or `append_text`. Prefer this for files larger than 8 KB or when you already know the exact text to change. Example: `"replacements":[{"find":"old text","replace":"new text"}]`; do not pass replacements as plain strings or `"old -> new"` entries.
- `delete` / `delete_file` / `delete_path`: delete a workspace file or directory. Use `path`. If `path` is `Apps/<app_id>.html`, AuraGo also removes the generated app registration for `<app_id>`.
- `delete_app`: delete a generated desktop app. Use `app_id`. Built-in apps cannot be deleted.
- `read_document`: read `.docx`, `.html`, `.md`, or `.txt` through AuraGo's Office backend. Use `path`; returns `document` with `title`, `text`, `html`, `delta`, and `office_version`.
- `write_document`: create/update `.docx`, `.html`, `.md`, or `.txt`. Use `path`, plus either `content`/`title` or a `document` object.
- `patch_document`: agent-friendly document edits. Use `path`, optional seed `content`, `prepend_text`, `append_text`, `title`, and `replacements:[{find,replace}]`.
- `read_workbook`: read `.xlsx`, `.xlsm`, or `.csv` through the Office backend. Use `path`; returns workbook JSON `{sheets:[{name, rows:[[ {value, formula} ]]}]}`.
- `write_workbook`: create/update `.xlsx`, `.xlsm`, or `.csv`. Use `path` and `workbook`.
- `set_cell`: update one workbook cell. Use `path`, `sheet`, `cell` (A1 style), and either `value` or `formula`.
- `set_range`: update a workbook range. Use `path`, `sheet`, `start_cell`, and `values` as a 2D array of strings or `{value, formula}` cells.
- `evaluate_formula`: evaluate AuraGo's safe formula subset in a workbook sheet. Use `path`, `sheet`, and `formula`.
- `export_file`: export an Office file to another workspace file. Use `path`, `output_path`, and `format` (`docx`, `html`, `md`, `txt`, `xlsx`, or `csv`).
- `install_app`: register a generated app and install its files under `Apps/<id>/`. Provide `manifest` and `files`.
- `upsert_widget`: register or update a pinned widget. Provide `widget`.
- `open_app` / `open_in_app`: ask the browser desktop to open an app. Provide `app_id` and optionally `path` to open a specific file. Available built-in apps: `editor` (plain text workspace files), `writer` (word-processing documents), `sheets` (spreadsheets), `code-studio` (code). Use the generated app id itself (for example `space-invaders`) when you want to run a generated app after editing it. `open_in_app` can also infer the generated app when `path` is its entry file, for example `Apps/space-invaders/index.html`. Code Studio mounts the virtual desktop workspace at `/workspace`, so a desktop file such as `Apps/space-invaders/game.js` opens there as `/workspace/Apps/space-invaders/game.js`; do not pass host filesystem or AuraGo repo paths.
- `show_notification`: show a desktop notification. Provide `title` and `content`.
- `list_apps`: list all desktop apps. Returns `builtin_apps`, `installed_apps`, `all_apps`, and `counts` (builtin, installed, total).
- `get_app`: retrieve full details for one built-in or installed app. Use `app_id`. Returns `app`, `found`, and `source` (`builtin` or `installed`).
- `list_widgets`: list all pinned widgets with their position, size, and owning app.
- `get_widget`: retrieve full details for one widget. Use `widget_id`.
- `diagnose_app`: run diagnostics on a desktop app. Use `app_id`; checks registration, built-in status, entry file path/readability/non-empty content, `health`/`health_reason`, and recommendations.
- `diagnose_widget`: run diagnostics on a widget. Use `widget_id`; checks registration, widget payload, entry path/readability/non-empty content, standalone/app-backed flags, and recommendations.

## Path Rules

The desktop workspace is rooted at `virtual_desktop.workspace_dir`. All `path` values are relative to this root. Top-level folders include `Documents`, `Apps`, and `Widgets`.

- **Apps path**: Generated app files live under `Apps/<app_id>/`. The manifest `entry` field names a file inside that folder, for example `index.html`. You may also write a single-file app directly to `Apps/<app_id>.html`; AuraGo auto-registers it as `Apps/<app_id>/index.html`.
- **Widgets path**: Standalone widgets live under `Widgets/<widget_id>.html` or `Widgets/<widget_id>/index.html`. App-backed widget entries live inside their owning app folder (`Apps/<app_id>/widget.html`).
- **Documents path**: Office files and general workspace files live under `Documents/` or other workspace folders returned by `status`.
- Do not use absolute filesystem paths, host repo paths, or paths outside the workspace. Code Studio resolves workspace files under `/workspace`.

## install_app vs Apps/<id>.html

Use `install_app` when you need a full generated app with a manifest, multiple files, SDK runtime, and icon registration. This is the recommended path for most generated apps.

Use `write_file` to `Apps/<id>.html` only for quick single-file HTML apps that do not need a manifest or SDK features. When you write non-empty HTML to `Apps/<id>.html`, AuraGo auto-registers a generated app with an inferred manifest. If you later need to add files or set an explicit icon, switch to `install_app` and use the same `id`.

Deleting `Apps/<id>.html` with `delete` or `delete_file` also removes the auto-registered app. Use `delete_app` with `app_id` to remove an app installed via `install_app`.

## Agent Workflows

### Quick Start: Generated App

1. Call `status` to check the desktop state and `icon_catalog`.
2. Choose an `id`, `name`, and `icon` for the app.
3. Build the `manifest` and `files` map.
4. Call `install_app` with the manifest and files.
5. Call `diagnose_app` with `app_id` to verify the installation.
6. Call `open_app` with `app_id` to launch it.

Example flow:

```json
{"operation": "install_app", "manifest": {"id": "todo-app", "name": "Todo List", "version": "1.0.0", "icon": "notes", "entry": "index.html", "runtime": "aura-desktop-sdk@1", "description": "A simple todo list."}, "files": {"index.html": "...", "app.js": "..."}}
{"operation": "diagnose_app", "app_id": "todo-app"}
{"operation": "open_app", "app_id": "todo-app"}
```

### Quick Start: Standalone Widget

1. Call `status` to check available space and `icon_catalog`.
2. Choose a `widget_id`, `title`, and `icon`.
3. Write complete non-empty HTML to `Widgets/<widget_id>.html` or `Widgets/<widget_id>/index.html` using `write_file`.
4. The desktop auto-registers and pins the widget.
5. Call `diagnose_widget` with `widget_id` to verify it.

Standalone widgets should not use the SDK bridge unless they explicitly need desktop actions. Keep them simple and self-contained.

### Quick Start: App-Backed Widget

1. Install the owning app first with `install_app`.
2. Create the widget entry HTML inside the app folder (for example `Apps/<app_id>/widget.html`).
3. Call `upsert_widget` with `widget.id`, `widget.app_id`, `widget.entry`, and layout fields (`x`, `y`, `w`, `h`).
4. Call `diagnose_widget` with `widget_id` to verify it.

App-backed widgets share the owning app's permissions and can use the full SDK bridge.

## Permissions Catalog

Generated apps and widgets declare permissions in their `manifest.permissions` or `widget.permissions` arrays. The desktop enforces these at runtime through the SDK bridge.

| Permission | Scope |
|---|---|
| `apps:open` | Open other apps via `AuraDesktop.desktop.openApp` |
| `files:read` | Read workspace files via `AuraDesktop.fs.read` |
| `files:write` | Write workspace files via `AuraDesktop.fs.write` |
| `filesystem:read` | Alias for `files:read`; workspace file read via SDK bridge |
| `filesystem:write` | Alias for `files:write`; workspace file write via SDK bridge |
| `notifications` | Show desktop notifications via `AuraDesktop.notifications.show` |
| `widgets:write` | Register or update widgets via `AuraDesktop.widgets.register` |

Request only the permissions the app needs.

## SDK API Reference

Generated apps and widgets using `aura-desktop-sdk@1` have these SDK signatures available in the iframe:

```js
AuraDesktop.app(options: { title?: string, root?: string | Element })
  // returns { root, context, mount(content), toolbar(items), notify(message, title) }

AuraDesktop.request(action, payload)

AuraDesktop.ui.icon(name, options?)
AuraDesktop.ui.button(options)
AuraDesktop.ui.toolbar(items)
AuraDesktop.ui.panel(children, options?)
AuraDesktop.ui.card(options)
AuraDesktop.ui.list(items, renderItem)
AuraDesktop.ui.tabs(options)
AuraDesktop.ui.field(options)
AuraDesktop.ui.input(options?)
AuraDesktop.ui.textarea(options?)
AuraDesktop.ui.select(options)
AuraDesktop.ui.toggle(options)
AuraDesktop.ui.emptyState(options)
AuraDesktop.ui.toast(message)

AuraDesktop.fs.list(path)
AuraDesktop.fs.read(path)
AuraDesktop.fs.write(path, content)

AuraDesktop.widgets.register(definition)
AuraDesktop.widgets.resize(options?)

AuraDesktop.notifications.show(options)

AuraDesktop.desktop.openApp(appID)
AuraDesktop.desktop.context()

AuraDesktop.menu.set(menus)
AuraDesktop.menu.clear()
AuraDesktop.menu.onAction(handler)

AuraDesktop.contextMenu.set(itemsOrFactory)
AuraDesktop.contextMenu.show(items, eventOrPoint)
AuraDesktop.contextMenu.clear()
AuraDesktop.contextMenu.onAction(handler)

AuraDesktop.clipboard.readText()
AuraDesktop.clipboard.writeText(text)

AuraDesktop.icons.catalog()
```

## App Manifest

```json
{
  "operation": "install_app",
  "manifest": {
    "id": "quick-notes",
    "name": "Quick Notes",
    "version": "1.0.0",
    "icon": "notes",
    "entry": "index.html",
    "runtime": "aura-desktop-sdk@1",
    "description": "A compact notes app."
  },
  "files": {
    "index.html": "<link rel=\"stylesheet\" href=\"/css/desktop-sdk.css\"><main id=\"app\"></main><script src=\"/js/desktop/aura-desktop-sdk.js\"></script><script src=\"app.js\"></script>",
    "app.js": "const app = AuraDesktop.app({ title: 'Quick Notes' }); app.mount(AuraDesktop.ui.panel([AuraDesktop.ui.emptyState({ icon: 'notes', title: 'Ready' })]));"
  }
}
```

Prefer AuraGo's semantic themed icon names from `status.icon_catalog.categories` first, then `status.icon_catalog.preferred`: categories include `games`, `office`, `productivity`, `tools`, `media`, `internet`, `system`, and `documents`. Use them as shortlists when selecting `manifest.icon` or `widget.icon`; for example games can use `run`, office can use `writer` or `spreadsheet`, productivity can use `notes` or `workflow`, and tools can use `tools` or `terminal`. The preferred catalog also includes `analytics`, `apps`, `archive`, `audio`, `audio-player`, `backup`, `book`, `browser`, `calendar`, `calculator`, `camera`, `chat`, `cloud`, `code`, `css`, `database`, `desktop`, `documents`, `downloads`, `editor`, `folder`, `forms`, `go`, `help`, `html`, `image`, `javascript`, `json`, `mail`, `markdown`, `map`, `network`, `notes`, `pdf`, `phone`, `printer`, `python`, `radio`, `run`, `settings`, `spreadsheet`, `terminal`, `text`, `tools`, `trash`, `video`, `weather`, `workflow`, `writer`, `xml`, or `yaml`. The desktop and SDK resolve these through the active Papirus or WhiteSur SVG theme and fall back to the built-in sprite sheet when needed. The Fruity UI theme pairs especially well with the WhiteSur icon theme, but app and widget icon choice stays independent from UI theme choice. `status.icon_catalog.aliases` lists friendly aliases such as `game -> run`, `space-invaders -> run`, `chart -> analytics`, `sparkles -> apps`, `edit -> editor`, `email -> mail`, `note -> notes`, `todo -> notes`, and `music-player -> audio-player`. App and widget icons are normalized against this catalog; emoji icons and unknown custom names are rejected. If `icon` is omitted, AuraGo infers a catalog icon from app/widget id, name/title, type, entry, or description. Use `sprite:<name>` only when you deliberately need a legacy sprite icon.
The app `entry` file must exist in `files` and must contain real HTML. Do not install placeholder or empty entry files.

## Office Examples

Create a DOCX document the browser Writer app can open:

```json
{
  "operation": "write_document",
  "path": "Documents/meeting-notes.docx",
  "title": "Meeting Notes",
  "content": "Agenda\nBudget review\nNext steps"
}
```

Create a workbook and add a formula:

```json
{
  "operation": "write_workbook",
  "path": "Documents/budget.xlsx",
  "workbook": {
    "sheets": [
      {
        "name": "Budget",
        "rows": [
          [{"value": "Item"}, {"value": "Amount"}],
          [{"value": "Coffee"}, {"value": "12.50"}]
        ]
      }
    ]
  }
}
```

```json
{
  "operation": "set_cell",
  "path": "Documents/budget.xlsx",
  "sheet": "Budget",
  "cell": "B3",
  "formula": "SUM(B2:B2)"
}
```

For direct Office work, prefer the dedicated `office_document` and `office_workbook` tools. Keep using `virtual_desktop` when the same task also needs desktop state, app/window events, widgets, or generated apps. Python skills should call these native tools through the Tool Bridge (`internal_tools`) instead of importing Excelize or editing OOXML directly.

Generated browser apps should use the first-party Aura Desktop SDK:

- Add `/css/desktop-sdk.css` and `/js/desktop/aura-desktop-sdk.js` to the app entry HTML.
- Set `manifest.runtime` to `aura-desktop-sdk@1` or omit it to use that default.
- Request only the permissions the app needs, for example `files:read`, `files:write`, `widgets:write`, `notifications`, or `apps:open`.
- Build controls with `AuraDesktop.ui` (`icon`, `button`, `toolbar`, `panel`, `card`, `list`, `tabs`, `field`, `input`, `textarea`, `select`, `toggle`, `emptyState`, `toast`) instead of custom per-app styling. Pass semantic icon names to `icon`, `button`, `card`, and `emptyState`; do not use emoji as app or tool icons.
- Use `await AuraDesktop.icons.catalog()` when an app needs to choose icons dynamically. It returns the same `icon_catalog` object from desktop bootstrap, including the active theme, preferred semantic names, aliases, and the legacy `sprite:` prefix.
- Use `AuraDesktop.fs`, `AuraDesktop.widgets.register`, `AuraDesktop.widgets.resize`, `AuraDesktop.notifications.show`, and `AuraDesktop.desktop.openApp` for desktop actions. The SDK talks to the desktop shell through a safe iframe bridge. Widget HTML served with `widget_id` also gets a small auto-resize bridge injected by the desktop shell; call `AuraDesktop.widgets.resize()` after large async layout changes if the widget content changes outside normal DOM resize observation.
- Use `AuraDesktop.menu.set(menus)` to register optional window menus for generated app windows, `AuraDesktop.menu.clear()` during cleanup when needed, and `AuraDesktop.menu.onAction(handler)` when menu items use string action IDs. Menu items support separators, submenus, icons, shortcuts, disabled/checked/hidden states, and direct function actions.
- Use `AuraDesktop.contextMenu.set(itemsOrFactory)` for right-click behavior inside generated apps and app-backed widgets. Return menu items when the target has meaningful actions, or return an empty list to suppress the Browser context menu on inert UI chrome. You can also call `AuraDesktop.contextMenu.show(items, eventOrPoint)` from your own `contextmenu` handler, remove the listener with `AuraDesktop.contextMenu.clear()`, and subscribe to string action IDs with `AuraDesktop.contextMenu.onAction(handler)`.
- Use `AuraDesktop.clipboard.readText()` and `AuraDesktop.clipboard.writeText(text)` for text copy/paste in sandboxed apps and widgets. Keep copy/paste on fields, editors, selected rows/cells, or useful result displays; do not add paste actions to destructive or ambiguous surfaces.

Example generated app menu:

```js
AuraDesktop.menu.set([
  {
    id: 'file',
    label: 'File',
    items: [
      { id: 'save', label: 'Save', icon: 'save', shortcut: 'Ctrl+S', action: save },
      { type: 'separator' },
      { id: 'close', label: 'Close', icon: 'x', disabled: true }
    ]
  }
]);
```

Example generated app context menu:

```js
AuraDesktop.contextMenu.set(event => {
  const row = event.target.closest('[data-note-id]');
  if (!row) return [];
  return [
    { id: 'open', label: 'Open', icon: 'folder-open', action: () => openNote(row.dataset.noteId) },
    { id: 'copy-title', label: 'Copy title', icon: 'copy', action: () => AuraDesktop.clipboard.writeText(row.textContent.trim()) }
  ];
});
```

## Widget Registration

For a simple standalone widget, write complete non-empty HTML directly to `Widgets/<widget_id>.html` or `Widgets/<widget_id>/index.html` with `write_file`. The desktop automatically registers and pins that file as a widget. Do not write empty placeholder HTML. If `write_file` returns an empty-content error, retry by sending the full HTML in `content` before claiming completion.

For 3D printer camera widgets, first call `three_d_printer` with `operation: "camera_url"` or `operation: "show_live_stream"` and use the returned same-origin `proxy_url` in the widget HTML, for example `<img src="/api/3d-printers/printer-1/camera/stream">`. Do not use the raw LAN URL inside generated HTML, and do not request a `network` permission for a simple camera widget. AuraGo normalizes known configured printer camera URLs to the same-origin proxy when desktop app/widget files are written, but agents should still prefer `proxy_url` directly.

For widgets owned by a generated app, register them with `upsert_widget`. An app-backed widget can use the SDK bridge and app permissions.

```json
{
  "operation": "upsert_widget",
  "widget": {
    "id": "quick-notes-summary",
    "app_id": "quick-notes",
    "type": "summary",
    "title": "Quick Notes",
    "icon": "notes",
    "entry": "widget.html",
    "runtime": "aura-desktop-sdk@1",
    "permissions": ["notifications"],
    "x": 0,
    "y": 0,
    "w": 2,
    "h": 2,
    "config": {
      "source": "Documents/notes.md"
    }
  }
}
```

If `entry` is set for an app-backed widget, it must be a file inside the owning app directory (`Apps/<app_id>/`). Standalone widget entries live under `Widgets/` and may be either `<widget_id>.html` or `<widget_id>/index.html`. Widget entries should also load the SDK stylesheet and script when they need SDK features.
Register an iframe widget only after the owning app has been installed and the widget entry file exists with non-empty HTML content.

## Complete Examples

### Generated App with Widget

Install a generated app and its companion widget in one flow:

```json
{
  "operation": "install_app",
  "manifest": {
    "id": "weather-dash",
    "name": "Weather Dashboard",
    "version": "1.0.0",
    "icon": "weather",
    "entry": "index.html",
    "runtime": "aura-desktop-sdk@1",
    "description": "Local weather dashboard.",
    "permissions": ["files:read", "notifications"]
  },
  "files": {
    "index.html": "<link rel=\"stylesheet\" href=\"/css/desktop-sdk.css\"><main id=\"app\"></main><script src=\"/js/desktop/aura-desktop-sdk.js\"></script><script src=\"app.js\"></script>",
    "app.js": "const app = AuraDesktop.app({ title: 'Weather Dashboard' }); app.mount(AuraDesktop.ui.panel([AuraDesktop.ui.emptyState({ icon: 'weather', title: 'Loading...' })]));",
    "widget.html": "<link rel=\"stylesheet\" href=\"/css/desktop-sdk.css\"><main id=\"widget\"></main><script src=\"/js/desktop/aura-desktop-sdk.js\"></script><script src=\"widget.js\"></script>",
    "widget.js": "document.getElementById('widget').textContent = 'Weather widget ready';"
  }
}
```

Then register the widget:

```json
{
  "operation": "upsert_widget",
  "widget": {
    "id": "weather-dash-mini",
    "app_id": "weather-dash",
    "type": "summary",
    "title": "Weather",
    "icon": "weather",
    "entry": "widget.html",
    "runtime": "aura-desktop-sdk@1",
    "permissions": ["notifications"],
    "x": 0,
    "y": 0,
    "w": 2,
    "h": 1
  }
}
```

### Standalone Widget

```json
{
  "operation": "write_file",
  "path": "Widgets/clock.html",
  "content": "<div style='padding:8px;font-family:sans-serif;'><strong>Clock</strong><div id='t'></div><script>setInterval(()=>document.getElementById('t').textContent=new Date().toLocaleTimeString(),1000)</script></div>"
}
```

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| `install_app` returns empty-content error | Entry file HTML is empty or whitespace-only | Provide real HTML in the entry file |
| `write_file` to `Apps/<id>.html` fails | Content is empty or too small | Add meaningful HTML content; do not use placeholders |
| Widget does not appear on desktop | Entry file missing or empty | Verify `Widgets/<id>.html` exists with non-empty content; call `diagnose_widget` |
| App opens but shows blank page | Missing SDK CSS/JS links or broken `entry` path | Ensure `/css/desktop-sdk.css` and `/js/desktop/aura-desktop-sdk.js` are in the entry HTML |
| `open_app` says app not found | App ID mismatch or app was deleted | Check `list_apps` for the correct `app_id`; reinstall if needed |
| Icon shows generic fallback | Unknown or emoji icon name | Use a catalog semantic name from `status.icon_catalog`; avoid emoji |
| SDK bridge calls fail silently | Missing permission in manifest | Add the required permission to `manifest.permissions` and reinstall |
| Widget fetches blocked by CSP | Widget tried to call an external API directly | Use same-origin requests or agent-mediated flows; do not fetch arbitrary third-party APIs from widgets |
| `diagnose_app` reports entry file unreadable or empty | Entry file missing, unreadable, or has no content | Reinstall or rewrite the app entry file with non-empty HTML; check `entry_path` in the diagnosis output |
