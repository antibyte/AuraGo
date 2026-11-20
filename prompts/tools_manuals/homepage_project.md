# homepage_project

Manage homepage project lifecycle: initialize, start/stop, build, install dependencies, run dev servers, publish locally, tunnel, and destroy with explicit `force=true`.

Use homepage workspace paths and project directories. Do not use generic filesystem tools for homepage project files.

## Local browser libraries

Use `install_library` with `library`, `project_dir` (or `managed_site_id`), and `reason`. Supported IDs are `bootstrap`, `alpinejs`, `htmx`, `chartjs`, `swiper`, `three`, and `tailwindcss`. The tool installs the latest stable release into the selected project's public assets and returns the version, project-relative files, and browser paths. Use `stylesheet` in a stylesheet link, and `script` with the returned `script_mode` (`defer` or `module`). Resolve these site-relative browser paths against the project's existing base URL. In framework projects, reference public assets at runtime and use the project's asset URL helper where available.

Three.js provides its browser ES module and the matching relative core module. Import the returned module URL in browser code or map `three` to it with an import map. For Chart.js date scales, use the framework's ordinary dependency flow when a date adapter is required.

Tailwind CSS uses the official CLI in the managed Homepage development container. Add the returned stylesheet link, then call `compile_tailwind` with the same project, exact installed `version`, and `reason` after changing Tailwind classes. Compile before previewing or publishing those changes. Prefer the project's existing Tailwind integration when the site already uses one. Downloads need `agent.allow_network_requests`. Tailwind also needs Docker write access and Homepage container management.

Keep current references until the new version is installed. Verify a fresh preview using the local assets. For ordinary framework packages, use `install_deps` and the project's existing build workflow.

Project-scoped operations must use workspace-relative `project_dir` or `name` values, for example `my-site`. Use the actual project directory from the homepage workspace or registry; do not use guessed folder names, absolute host folders, or `/workspace/...` paths.

For `status`, inspect `local_preview.available`, `http_status`, and `project_dir`
as well as container state. `running: true` does not prove that the website is
available; an empty HTTP 404 or invalid document is a failed preview. A successful
status query is not a successful publication. Do not change deployment providers
to work around a failed local preview without the user's instruction.


## Managed website identity and change reasons

Query homepage_registry overview/get_site before changing an existing page. Every mutation requires reason, a short statement of purpose. managed_site_id is the internal identity, distinct from provider IDs. Exec requires project_dir and starts there. Changes and outcomes are recorded automatically; avoid duplicate routine notes. Use list_events for what changed and why, and discover_workspace/adopt for unregistered projects. Historical unknown reasons remain unknown.
