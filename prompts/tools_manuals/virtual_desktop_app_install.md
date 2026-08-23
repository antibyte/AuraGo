# virtual_desktop_app_install

Atomically install or replace one generated virtual desktop app from a complete manifest and file set.

- `manifest.id`, `manifest.name`, and `manifest.entry` are required.
- `files` must be a non-empty object keyed by app-relative paths.
- Use `index.html`, not `Apps/<app_id>/index.html`, for both `manifest.entry` and the matching `files` key.
- The exact `manifest.entry` path must exist in `files` with non-empty content.
- Include every generated app file in the same call. Existing workspace files are never reused implicitly.
- Omit `manifest.icon` to let AuraGo infer one, or use a semantic name returned by the desktop icon catalog.

After a successful install, use `virtual_desktop_apps` with `diagnose_app`, then open the app only when diagnosis is healthy.
