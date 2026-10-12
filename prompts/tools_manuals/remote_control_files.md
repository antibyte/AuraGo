# remote_control_files

Read, write, edit, and search files on connected remote-control devices.

Classic clients enforce configured file-size limits on reads, inputs and final
edits through rooted handles. Edits publish atomically and preserve the original
on cancellation or write failure. Empty device paths inherit global paths;
both empty deny file access. AgoDesk roots remain locally managed. Respect
`UNSUPPORTED_CAPABILITY`, `CONFIG_PENDING`, `CLIENT_BUSY`, and `RESULT_EVICTED`;
do not blindly replay a mutation with a new command ID after a timeout.

Use `root_id` for AgoDesk file-access roots when paths are relative to an approved root.

Prefer `file_patch` for precise AgoDesk edits. First read the file and keep its `sha256`, then call `file_patch` with `expected_sha256`, `patches`, and the default `dry_run:true`. Apply only with explicit `dry_run:false` after the dry-run result is acceptable.

Each patch is `{old_text,new_text,expected_occurrences}` and must be exact. If the result preserves `FILE_PATCH_MISMATCH` or `FILE_HASH_MISMATCH`, read the file again and base the next patch on the fresh content instead of guessing or using fuzzy replacement.
