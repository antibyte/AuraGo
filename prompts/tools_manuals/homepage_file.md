# homepage_file

Read, write, edit, and optimize files inside the homepage workspace.

Paths must include the project directory prefix, for example `my-site/src/App.jsx`. Use `sub_operation` for text, JSON, YAML, and XML edits.

**After every write or edit, call `homepage_registry` → `add_history` to record what changed and why.**
