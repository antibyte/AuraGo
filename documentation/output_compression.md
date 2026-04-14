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
| `go test` | Failure extraction | Extracts failing tests + summary line |
| `pytest` | Failure extraction | Extracts failing tests + short summary |
| `cargo test` | Failure extraction | Extracts failing tests + summary |
| `grep` | Directory grouping | Groups matches by directory with counts |
| `find` | Directory grouping | Groups results by directory |
| `ls`/`tree` | Directory grouping | Groups entries by directory |

### Python Filters (`python_compression`)

| Filter | What it does |
|--------|-------------|
| Traceback frame filtering | Keeps user-code frames, omits `site-packages/`, `/usr/lib/python`, etc. |
| Output dedup | Collapses consecutive identical lines |

### API Filters (`api_compression`)

| Filter | What it does |
|--------|-------------|
| JSON compaction | Removes `null`, `""`, `[]`, `{}` fields from multi-line JSON |
| Generic pipeline | ANSI strip → whitespace collapse → dedup → tail focus |

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
├── compressor.go    # Config, Compress(), DefaultConfig(), routing logic
├── dedup.go         # Generic pipeline: dedup, whitespace, tail-focus, ANSI strip
├── shell.go         # Shell-specific filters (git, docker, test, grep, find, ls)
├── analytics.go     # Thread-safe stats aggregator for dashboard
└── compressor_test.go # 51+ tests
```

Integration point: `internal/agent/tool_execution_policy.go` → `finalizeToolExecution()`
