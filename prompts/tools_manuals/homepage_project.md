# homepage_project

Manage homepage project lifecycle: initialize, start/stop, build, install dependencies, run dev servers, publish locally, tunnel, and destroy with explicit `force=true`.

Use homepage workspace paths and project directories. Do not use generic filesystem tools for homepage project files.

Project-scoped operations must use workspace-relative `project_dir` or `name` values, for example `my-site`. Use the actual project directory from the homepage workspace or registry; do not use guessed folder names, absolute host folders, or `/workspace/...` paths.

For `status`, inspect `local_preview.available`, `http_status`, and `project_dir`
as well as container state. `running: true` does not prove that the website is
available; an empty HTTP 404 or invalid document is a failed preview. A successful
status query is not a successful publication. Do not change deployment providers
to work around a failed local preview without the user's instruction.


## Managed website identity and change reasons

Query homepage_registry overview/get_site before changing an existing page. Every mutation requires reason, a short statement of purpose. managed_site_id is the internal identity, distinct from provider IDs. Exec requires project_dir and starts there. Changes and outcomes are recorded automatically; avoid duplicate routine notes. Use list_events for what changed and why, and discover_workspace/adopt for unregistered projects. Historical unknown reasons remain unknown.
