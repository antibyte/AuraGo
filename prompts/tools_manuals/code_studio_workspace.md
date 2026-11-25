# code_studio_workspace

Work in a saved **SSH workspace** of Code Studio: a folder on a remote host that the user opened in Code Studio. The Code Studio chat context names the `workspace_id`; use exactly that id.

Only for SSH workspaces. The local Code Studio workspace (`workspace_id` "local" or none) is the virtual desktop workspace: use `virtual_desktop_files` there. Never use `virtual_desktop_files`, `virtual_desktop_apps` `open_in_app`, `file_editor`, `filesystem` or shell tools for SSH workspace paths; they do not reach the remote folder.

Paths are relative to the remote folder (`src/main.go`); `/workspace/src/main.go` from the Code Studio context is the same file. Host paths, `~` and `..` are refused.

Operations:
- `list_files`: `path` (optional, default the folder root).
- `read_file`: `path`. Large files return the start with `truncated: true`; continue with `read_file_excerpt`.
- `read_file_excerpt`: `path`, `line_start` (1-based), `line_count` (max 240).
- `search`: `query`, optional `path`, `regex`, `case_sensitive`, `whole_word`, `include`, `exclude`, `max_matches` (max 200).
- `write_file`: `path`, complete `content`. Empty content needs `allow_empty: true`.
- `patch_file`: `path` with `find` and `replace`, or `replacements: [{"find": "...", "replace": "..."}]`. Every find text must occur exactly once; add surrounding lines when it is ambiguous. Replacements apply in order.
- `run_command`: `command` (run by `/bin/sh -c` in the folder, or in `cwd`), `timeout_seconds` (default 60, max 120). Returns `exit_code` and a capped `output`.

Desktop read-only allows list, read and search only. After writes and commands Code Studio reloads its open tabs by itself: do not open the file in an app afterwards and do not paste the changed code into the reply.

```json
{"action": "code_studio_workspace", "operation": "patch_file", "workspace_id": "ws_7f3k", "path": "cmd/api/main.go", "find": "port := 8080", "replace": "port := 9090"}
```
