# homepage_project

Manage homepage project lifecycle: initialize, start/stop, build, install dependencies, run dev servers, publish locally, tunnel, and destroy with explicit `force=true`.

Use homepage workspace paths and project directories. Do not use generic filesystem tools for homepage project files.

Project-scoped operations must use workspace-relative `project_dir` or `name` values, for example `my-site`. Use the actual project directory from the homepage workspace or registry; do not use guessed folder names, absolute host folders, or `/workspace/...` paths.

**Before planning changes, read `homepage_registry` → `list_history`. After init, build, or publish, call `homepage_registry` → `add_history`.**
