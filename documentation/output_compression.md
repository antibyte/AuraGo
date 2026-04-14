# Output Compression

AuraGo can automatically compress verbose tool outputs before they enter the LLM context window. This reduces token consumption by 40–70 % for common shell, test, and API commands without losing semantic content.

## How It Works

Compression runs **before** the `tool_output_limit` truncation in the tool execution pipeline:

```
Tool Output → Output Compression → tool_output_limit truncation → LLM Context
```

This means compression can intelligently filter and deduplicate content, while truncation is a blunt last-resort cutoff.

## Configuration

All settings live under `agent.output_compression` in `config.yaml`:

```yaml
agent:
    output_compression:
        enabled: true                # master toggle (default: true)
        min_chars: 500               # only compress outputs exceeding this size
        preserve_errors: true        # never compress error outputs
        shell_compression: true      # shell-specific filters
        python_compression: true     # Python traceback filtering
        api_compression: true        # JSON compaction for API responses
```

### Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Master toggle. Set to `false` to disable all compression. |
| `min_chars` | int | `500` | Only compress outputs exceeding this character count. Short outputs pass through unchanged. |
| `preserve_errors` | bool | `true` | When `true`, outputs containing error markers (`[EXECUTION ERROR]`, `fatal:`, `panic:`, etc.) are never compressed. |
| `shell_compression` | bool | `true` | Enable domain-specific filters for shell commands (git, docker, kubectl, go test, pytest, grep, find, ls/tree). |
| `python_compression` | bool | `true` | Enable Python traceback frame filtering (keeps user-code frames, omits library frames) and output deduplication. |
| `api_compression` | bool | `true` | Enable JSON compaction: removes null/empty fields from API responses. |

### Relationship to `tool_output_limit`

- `tool_output_limit` (default: 50000) is a hard truncation — anything beyond it is cut off.
- Output compression runs **before** truncation and is semantic — it filters, deduplicates, and summarises.
- Recommended: keep `tool_output_limit` as a safety net, let compression handle the intelligence.

## Supported Filters

### Shell Filters (`shell_compression`)

| Command | Filter | What it does |
|---------|--------|-------------|
| `git status` | Status grouping | Groups files by state (staged/unstaged/untracked) with counts |
| `git log` | Oneline conversion | Converts verbose log to `--oneline` format, caps at 50 entries |
| `git diff` | Summary + hunks | Shows file-level summary + first 3 hunks |
| `docker ps` | Hash stripping | Strips container ID hashes, keeps names and status |
| `docker logs` | Timestamp + dedup | Strips timestamps, deduplicates, keeps tail |
| `docker compose` | Per-subcommand | `ps` (hash strip), `config` (collapse), `events` (dedup) |
| `go test` | Failure extraction | Extracts failing tests + summary line |
| `pytest` | Failure extraction | Extracts failing tests + short summary |
| `cargo test` | Failure extraction | Extracts failing tests + summary |
| `npm`/`yarn`/`pnpm` test | Failure extraction | Extracts failing tests + summary |
| `eslint`/`tsc`/`ruff`/`golangci-lint` | Lint summary | Groups by severity, shows first N issues per group |
| `grep`/`rg`/`ag` | Directory grouping | Groups matches by directory with counts |
| `find` | Directory grouping | Groups results by directory |
| `ls`/`tree`/`dir` | Directory grouping | Groups entries by directory |
| `curl`/`wget` | Content-aware | JSON compact, HTML strip tags, verbose header dedup |
| `systemctl` | Status/list | `status` (key-value extract), `list-units` (tabular) |
| `kubectl`/`k3s`/`k9s` | K8s-aware | `get` (tabular), `describe` (section dedup), `logs` (dedup+tail) |
| `helm` | Per-subcommand | `list` (tabular), `status` (key-value), `history` (tabular) |
| `terraform`/`tf` | Plan/apply/show | Plan summary, apply result, state list grouping |
| `df` | Disk summary | Tabular with usage bars |
| `du` | Directory grouping | Groups by directory with sizes |
| `ps` | Process table | Strips header, keeps key columns |
| `ss`/`netstat` | Connection table | Strips header, groups by state |
| `ping`/`ping6` | Ping summary | Shows first/last + statistics |
| `dig` | DNS summary | Shows answer section + query time |
| `nslookup`/`host` | DNS summary | Shows answer section |
| `cat`/`less`/`more` | Log-aware | Log content: dedup+tail; non-log: tail-focus |
| `tail`/`head` | Log-aware | Log content: dedup+tail; non-log: tail-focus |
| `stat` | Multi-file grouping | Groups by file, shows key fields |
| `tar`/`zip`/`unzip` | Archive listing | Groups by directory, truncates long listings |
| `rsync` | Transfer summary | Shows stats, groups transferred files by dir |
| **Text pipelines** | | |
| `sort` | Text pipeline | Dedup consecutive lines, tail-focus for large output |
| `uniq` | Text pipeline | Collapse whitespace, tail-focus for large output |
| `cut` | Text pipeline | Collapse whitespace, tail-focus for columnar data |
| `sed` | Text pipeline | Dedup + tail-focus for large transformed output |
| `awk`/`gawk`/`mawk` | Text pipeline | Dedup + tail-focus for large output |
| `xargs` | Text pipeline | Dedup + tail-focus for large output |
| `jq` | JSON minify | Minifies JSON via `json.Compact`, then dedup + tail-focus |
| `tr` | Text pipeline | Collapse whitespace, dedup |
| `column` | Text pipeline | Collapse whitespace, tail-focus |
| `diff` | Diff summary | Reuses git-diff compression logic |
| `comm` | Text pipeline | Collapse whitespace, dedup |
| `paste` | Text pipeline | Collapse whitespace, dedup |

### Python Filters (`python_compression`)

| Filter | What it does |
|--------|-------------|
| Traceback frame filtering | Keeps user-code frames, omits `site-packages/`, `/usr/lib/python`, etc. |
| Output dedup | Collapses consecutive identical lines |

### API Filters (`api_compression`)

| Tool | Filter | What it does |
|------|--------|-------------|
| Home Assistant | State list | Groups by domain, shows entity count per domain |
| Home Assistant | Service list | Groups by domain, shows service count per domain |
| GitHub | Repos/issues/PRs/commits | Tabular with key fields, truncates long lists |
| SQL query | Result table | Shows column headers + rows, truncates large results |
| `filesystem` | list_dir | Groups dirs first, shows file sizes, truncates at 50 entries |
| `filesystem` | read_file | Preserves content, compacts wrapper metadata |
| `filesystem` | batch | Summarizes succeeded items, details failed items |
| `file_reader_advanced` | content | Preserves content, compacts line range wrapper |
| `file_reader_advanced` | search_context | Shows match count, truncated line ranges, limit 15 matches |
| `file_reader_advanced` | count_lines | Compacts to single line count |
| `smart_file_read` | analyze | Compacts to essential metadata (path, size, mime, recommendation) |
| `smart_file_read` | structure | Shows format, root type, top-level keys |
| `smart_file_read` | sample/summarize | Preserves content, compacts wrapper |
| `list_processes` | PID list | Compacts to count + comma-separated PIDs |
| `read_process_logs` | Log body | Dedup + tail-focus on log content, shows PID header |
| `manage_daemon` | Daemon list | Compact per-daemon status line (skill, status, uptime) |
| `manage_daemon` | Daemon status | Single daemon compact format |
| `manage_plan` | Plan list | Per-plan summary with status bracket, title, task progress |
| `manage_plan` | Plan get | Tasks with status markers, priority, timestamps |
| Generic API | JSON compaction | Removes `null`, `""`, `[]`, `{}` fields from multi-line JSON |
| Generic API | Generic pipeline | ANSI strip → whitespace collapse → dedup → tail focus |

### Generic Fallback

When no domain-specific filter matches, the generic pipeline runs:
1. ANSI escape sequence stripping
2. Consecutive blank line collapse
3. Consecutive duplicate line dedup (>3 repeats → count marker)
4. Tail focus for outputs >300 lines (keep head 50 + tail 100)

## Disabling Compression

### Disable entirely

```yaml
agent:
    output_compression:
        enabled: false
```

### Disable only shell filters

```yaml
agent:
    output_compression:
        shell_compression: false
```

### Disable only for specific tools

There is no per-tool toggle. Use the category toggles (`shell_compression`, `python_compression`, `api_compression`) to control groups of filters.

## Dashboard Monitoring

The Dashboard (Overview tab) shows an **Output Compression** card with:

- **Saved Characters** — total characters saved across all compressions
- **Savings Ratio** — average compression ratio (higher = more savings)
- **Compressed** — number of outputs that were compressed
- **Skipped** — number of outputs that were too short, errors, or disabled
- **Top Tools** — which tools benefit most from compression
- **Top Filters** — which filters are most active

API endpoint: `GET /api/dashboard/compression`

## Troubleshooting

### "My tool output is being modified incorrectly"

1. Check the Dashboard compression card to see which filter is being applied
2. Try disabling the specific category:
   ```yaml
   agent:
       output_compression:
           shell_compression: false  # if shell output is wrong
   ```
3. If the issue persists, disable compression entirely and file a bug report

### "Compression is not running"

1. Verify `enabled: true` in your config
2. Check that the output exceeds `min_chars` (default: 500 characters)
3. Error outputs are preserved by default (`preserve_errors: true`)

### "I want to see the original output"

Set `agent.debug_mode: true` in your config. The debug log will show:
```
DBG output compressed tool=execute_shell filter=git-status raw_chars=15000 compressed_chars=4200 ratio=0.28
```

## Architecture

```
internal/tools/outputcompress/
├── compressor.go      # Config, Compress(), DefaultConfig(), routing logic
├── dedup.go           # Generic pipeline: dedup, whitespace, tail-focus, ANSI strip
├── shell.go           # Shell-specific filters (git, docker, k8s, test, grep, find, ls, curl, ping, etc.)
├── text_pipeline.go   # Text processing pipeline (sort, uniq, cut, sed, awk, jq, xargs, tr, column, diff)
├── filesystem.go      # Filesystem tool compressors (list_dir, read_file, batch)
├── file_reader.go     # file_reader_advanced compressors (content, search_context, count_lines)
├── smart_file.go      # smart_file_read compressors (analyze, structure, sample/summarize)
├── process.go         # Process tool compressors (list_processes, read_process_logs)
├── agent_status.go    # Agent status compressors (manage_daemon, manage_plan)
├── github.go          # GitHub API compressors (repos, issues, PRs, commits, workflows)
├── sql.go             # SQL query result compressors
├── homeassistant.go   # Home Assistant compressors (states, services)
├── analytics.go       # Thread-safe stats aggregator for dashboard
└── compressor_test.go # 180+ tests
```

Integration point: `internal/agent/tool_execution_policy.go` → `finalizeToolExecution()`
