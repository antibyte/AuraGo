# virtual_desktop

Use `virtual_desktop` to control AuraGo's first-party browser desktop. It can inspect the desktop state, read and write files inside the desktop workspace, create and edit basic Office documents/workbooks, install generated JavaScript apps, pin widgets, open apps, and show desktop notifications.

The desktop workspace is jailed to `virtual_desktop.workspace_dir`. Never place credentials or vault values in generated app files. If an app needs sensitive data, build a small backend or agent-mediated flow that retrieves only the minimum safe result.

## Operations

- `status` / `bootstrap`: return desktop settings, built-in apps, installed apps, widgets, workspace folders, and `icon_catalog` for generated app icon selection.
- `list_files`: list a workspace directory. Use `path`, for example `Documents`.
- `read_file`: read one text file. Use `path`.
- `write_file`: write one text file. Use `path` and `content`.
- `read_document`: read `.docx`, `.html`, `.md`, or `.txt` through AuraGo's Office backend. Use `path`; returns `document` with `title`, `text`, `html`, and `delta`.
- `write_document`: create/update `.docx`, `.html`, `.md`, or `.txt`. Use `path`, plus either `content`/`title` or a `document` object.
- `read_workbook`: read `.xlsx`, `.xlsm`, or `.csv` through the Office backend. Use `path`; returns workbook JSON `{sheets:[{name, rows:[[ {value, formula} ]]}]}`.
- `write_workbook`: create/update `.xlsx`, `.xlsm`, or `.csv`. Use `path` and `workbook`.
- `set_cell`: update one workbook cell. Use `path`, `sheet`, `cell` (A1 style), and either `value` or `formula`.
- `export_file`: export an Office file to another workspace file. Use `path`, `output_path`, and `format` (`docx`, `html`, `md`, `txt`, `xlsx`, or `csv`).
- `install_app`: register a generated app and install its files under `Apps/<id>/`. Provide `manifest` and `files`.
- `upsert_widget`: register or update a pinned widget. Provide `widget`.
- `open_app`: ask the browser desktop to open an app. Provide `app_id`.
- `show_notification`: show a desktop notification. Provide `title` and `content`.

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

Prefer AuraGo's semantic Papirus icon names from `status.icon_catalog.preferred`: `apps`, `archive`, `audio`, `browser`, `calendar`, `calculator`, `code`, `css`, `database`, `desktop`, `documents`, `downloads`, `editor`, `folder`, `go`, `html`, `image`, `javascript`, `json`, `markdown`, `network`, `notes`, `pdf`, `python`, `settings`, `spreadsheet`, `terminal`, `text`, `trash`, `video`, `weather`, `xml`, or `yaml`. The desktop and SDK resolve these to Papirus SVGs and fall back to the built-in sprite sheet when needed. `status.icon_catalog.aliases` lists friendly aliases such as `sparkles -> apps`, `edit -> editor`, `note -> notes`, `todo -> notes`, and `music-player -> audio`. App and widget icons are normalized against this catalog; emoji icons and unknown custom names are rejected. If `icon` is omitted, AuraGo infers a catalog icon from app/widget id, name/title, type, entry, or description. Use `sprite:<name>` only when you deliberately need a legacy sprite icon.
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

Generated browser apps should use the first-party Aura Desktop SDK:

- Add `/css/desktop-sdk.css` and `/js/desktop/aura-desktop-sdk.js` to the app entry HTML.
- Set `manifest.runtime` to `aura-desktop-sdk@1` or omit it to use that default.
- Request only the permissions the app needs, for example `files:read`, `files:write`, `widgets:write`, `notifications`, or `apps:open`.
- Build controls with `AuraDesktop.ui` (`icon`, `button`, `toolbar`, `panel`, `card`, `list`, `tabs`, `field`, `input`, `textarea`, `toggle`, `emptyState`) instead of custom per-app styling. Pass semantic icon names to `icon`, `button`, `card`, and `emptyState`; do not use emoji as app or tool icons.
- Use `await AuraDesktop.icons.catalog()` when an app needs to choose icons dynamically. It returns the same `icon_catalog` object from desktop bootstrap, including the active theme, preferred semantic names, aliases, and the legacy `sprite:` prefix.
- Use `AuraDesktop.fs`, `AuraDesktop.widgets.register`, `AuraDesktop.notifications.show`, and `AuraDesktop.desktop.openApp` for desktop actions. The SDK talks to the desktop shell through a safe iframe bridge.

## Widget Registration

For a simple standalone widget, write complete non-empty HTML directly to `Widgets/<widget_id>.html` with `write_file`. The desktop automatically registers and pins that file as a widget. Do not write empty placeholder HTML. If `write_file` returns an empty-content error, retry by sending the full HTML in `content` before claiming completion.

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

If `entry` is set, it must be a file inside the owning app directory (`Apps/<app_id>/`). Widget entries should also load the SDK stylesheet and script.
Register an iframe widget only after the owning app has been installed and the widget entry file exists with non-empty HTML content.
