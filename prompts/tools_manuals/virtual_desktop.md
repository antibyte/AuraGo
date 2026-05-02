# virtual_desktop

Use `virtual_desktop` to control AuraGo's first-party browser desktop. It can inspect the desktop state, read and write files inside the desktop workspace, install generated JavaScript apps, pin widgets, open apps, and show desktop notifications.

The desktop workspace is jailed to `virtual_desktop.workspace_dir`. Never place credentials or vault values in generated app files. If an app needs sensitive data, build a small backend or agent-mediated flow that retrieves only the minimum safe result.

## Operations

- `status` / `bootstrap`: return desktop settings, built-in apps, installed apps, widgets, and workspace folders.
- `list_files`: list a workspace directory. Use `path`, for example `Documents`.
- `read_file`: read one text file. Use `path`.
- `write_file`: write one text file. Use `path` and `content`.
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
    "icon": "note",
    "entry": "index.html",
    "runtime": "aura-desktop-sdk@1",
    "description": "A compact notes app."
  },
  "files": {
    "index.html": "<link rel=\"stylesheet\" href=\"/css/desktop-sdk.css\"><main id=\"app\"></main><script src=\"/js/desktop/aura-desktop-sdk.js\"></script><script src=\"app.js\"></script>",
    "app.js": "const app = AuraDesktop.app({ title: 'Quick Notes' }); app.mount(AuraDesktop.ui.panel([AuraDesktop.ui.emptyState({ icon: 'note', title: 'Ready' })]));"
  }
}
```

Every generated app must have a non-empty `icon`. Use a name from the desktop sprite sheet when possible, for example `note`, `calendar`, `terminal`, `database`, `image`, `settings`, `folder`, or `sparkles`.

Generated browser apps should use the first-party Aura Desktop SDK:

- Add `/css/desktop-sdk.css` and `/js/desktop/aura-desktop-sdk.js` to the app entry HTML.
- Set `manifest.runtime` to `aura-desktop-sdk@1` or omit it to use that default.
- Request only the permissions the app needs, for example `files:read`, `files:write`, `widgets:write`, `notifications`, or `apps:open`.
- Build controls with `AuraDesktop.ui` (`button`, `toolbar`, `panel`, `card`, `list`, `tabs`, `field`, `input`, `textarea`, `toggle`, `emptyState`) instead of custom per-app styling.
- Use `AuraDesktop.fs`, `AuraDesktop.widgets.register`, `AuraDesktop.notifications.show`, and `AuraDesktop.desktop.openApp` for desktop actions. The SDK talks to the desktop shell through a safe iframe bridge.

## Widget Registration

Register widgets with `upsert_widget`. A widget can be a simple pinned card or an SDK-backed iframe owned by an app.

```json
{
  "operation": "upsert_widget",
  "widget": {
    "id": "quick-notes-summary",
    "app_id": "quick-notes",
    "type": "summary",
    "title": "Quick Notes",
    "icon": "note",
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
