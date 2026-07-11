# remote_control_files

Read, write, edit, and search files on connected remote-control devices.

Use `root_id` for AgoDesk file-access roots when paths are relative to an approved root.

Prefer `file_patch` for precise AgoDesk edits. First read the file and keep its `sha256`, then call `file_patch` with `expected_sha256`, `patches`, and the default `dry_run:true`. Apply only with explicit `dry_run:false` after the dry-run result is acceptable.

Each patch is `{old_text,new_text,expected_occurrences}` and must be exact. If the result preserves `FILE_PATCH_MISMATCH` or `FILE_HASH_MISMATCH`, read the file again and base the next patch on the fresh content instead of guessing or using fuzzy replacement.
