# virtual_desktop

Use `virtual_desktop` to control AuraGo's first-party browser desktop. It can inspect the desktop state, read and write files inside the desktop workspace, install generated JavaScript apps, pin widgets, open apps, and show desktop notifications.

The desktop workspace is jailed to `virtual_desktop.workspace_dir`. Never place credentials or vault values in generated app files. If an app needs sensitive data, build a small backend or agent-mediated flow that retrieves only the minimum safe result.

## Operations

- `status` / `bootstrap`: return desktop settings, built-in apps, installed apps, widgets, and workspace folders.
- `list_files`: list a workspace directory. Use `path`, for example `Documents`.
- `read_file`: read one text file. Use `path`.
- `write_file`: write one text file. Use `path` and `content`.
- `install_app`: install a generated app under `Apps/<id>/`. Provide `manifest` and `files`.
- `upsert_widget`: create or update a pinned widget. Provide `widget`.
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
    "icon": "note",
    "entry": "index.html",
    "description": "A compact notes app."
  },
  "files": {
    "index.html": "<main id=\"app\"></main><script src=\"app.js\"></script>",
    "app.js": "document.getElementById('app').textContent = 'Ready';"
  }
}
```

Keep generated apps self-contained, fast, and accessible. Prefer vanilla JavaScript unless a future desktop app runtime explicitly provides shared libraries.
