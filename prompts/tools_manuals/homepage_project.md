# homepage_project

Manage homepage project lifecycle: initialize, start/stop, build, install dependencies, run dev servers, publish locally, tunnel, and destroy with explicit `force=true`.

Use homepage workspace paths and project directories. Do not use generic filesystem tools for homepage project files.

**Before planning changes, read `homepage_registry` → `list_history`. After init, build, or publish, call `homepage_registry` → `add_history`.**
