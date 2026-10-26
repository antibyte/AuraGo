# homepage_file

Read, write, edit, and optimize files inside the homepage workspace.

Paths must include the project directory prefix, for example `my-site/src/App.jsx`. Supply `reason` for every mutation. Use `sub_operation` for text, JSON, YAML, and XML edits.

File paths support Unicode letters, spaces and Next.js route segments such as `my-site/app/(marketing)/[slug]/page.tsx` or `my-site/app/[[...slug]]/page.tsx`. Pass the literal path without shell escaping. Keep project identifiers simple; absolute paths, traversal segments, surrounding segment whitespace, control characters and shell operators are rejected.

For `operation: "edit_file"`, choose one of these exact `sub_operation` values:

| sub_operation | Required arguments | Effect |
| --- | --- | --- |
| `str_replace` | `old`, `new` | Replace one unique text match. `new: ""` deletes it. |
| `str_replace_all` | `old`, `new` | Replace every exact text match. |
| `insert_after`, `insert_before` | `marker`, `content` | Add lines after/before the one line containing `marker`. |
| `append`, `prepend` | `content` | Add text at the end/start. |
| `delete_lines` | `start_line`, `end_line` | Delete the inclusive, 1-based line range. |

`replace` and `insert` are not supported values. Duplicate marker lines are an error: inspect the file and choose a unique anchor. Do not guess which occurrence to change.

Example: `homepage_file(operation="edit_file", path="my-site/index.html", sub_operation="insert_before", marker="<!-- NEWS-START -->", content="<article>New item</article>", reason="Add the latest news item")`.

Prefer targeted edits for existing large files. `write_file` requires the complete file content and refuses empty writes. Docker reads/writes/edits transfer complete files up to 2 MiB; an oversized or unreadable file fails before an edit is written.

## Managed website identity and change reasons

Query homepage_registry overview/get_site before changing an existing page. Every mutation requires reason, a short statement of purpose. managed_site_id is the internal identity, distinct from provider IDs. Exec requires project_dir and starts there. Changes and outcomes are recorded automatically; avoid duplicate routine notes. Use list_events for what changed and why, and discover_workspace/adopt for unregistered projects. Historical unknown reasons remain unknown.

`ledger_updated: true` means the attempt was recorded, including failed attempts. Check `status` for execution success and verify the changed file; the ledger flag never overrides an error. Do not deploy after a failed edit.
