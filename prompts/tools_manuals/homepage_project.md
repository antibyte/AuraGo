# homepage_project

Manage homepage project lifecycle: initialize, start/stop, build, install dependencies, run dev servers, publish locally, tunnel, and destroy with explicit `force=true`.

Use homepage workspace paths and project directories. Do not use generic filesystem tools for homepage project files.

Project-scoped operations must use workspace-relative `project_dir` or `name` values, for example `my-site`. Use the actual project directory from the homepage workspace or registry; do not use guessed folder names, absolute host folders, or `/workspace/...` paths.

**Before planning changes, read `homepage_registry` → `list_history`. After init, build, or publish, call `homepage_registry` → `add_history`.**


## Managed website identity and change reasons

Query homepage_registry overview/get_site before changing an existing page. Every mutation requires reason, a short statement of purpose. managed_site_id is the internal identity, distinct from provider IDs. Exec requires project_dir and starts there. Changes and outcomes are recorded automatically; avoid duplicate routine notes. Use list_events for what changed and why, and discover_workspace/adopt for unregistered projects. Historical unknown reasons remain unknown.
