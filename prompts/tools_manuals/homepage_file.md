# homepage_file

Read, write, edit, and optimize files inside the homepage workspace.

Paths must include the project directory prefix, for example `my-site/src/App.jsx`. Use `sub_operation` for text, JSON, YAML, and XML edits.

**After every write or edit, call `homepage_registry` → `add_history` to record what changed and why.**


## Managed website identity and change reasons

Query homepage_registry overview/get_site before changing an existing page. Every mutation requires reason, a short statement of purpose. managed_site_id is the internal identity, distinct from provider IDs. Exec requires project_dir and starts there. Changes and outcomes are recorded automatically; avoid duplicate routine notes. Use list_events for what changed and why, and discover_workspace/adopt for unregistered projects. Historical unknown reasons remain unknown.
