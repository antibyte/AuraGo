# log_analyzer — Advanced Log Parsing and Search

Search, filter, and analyze large log files quickly without loading them entirely into memory.

## Operations

| Operation | Description | Required Params |
|-----------|-------------|-----------------|
| `tail` | Get the last N lines of a log file | `file_path` (optional: `lines` defaults to 100) |
| `search` | Find lines matching a keyword or regex | `file_path`, `query` |
| `errors` | Filter for common error keywords (ERROR, FATAL, Exception) | `file_path` |
| `time_range`| Extract logs between two timestamps | `file_path`, `start_time`, `end_time` |

## Key Behaviors

- **search**: Supports plain text and basic regex.
- **errors**: Automatically scans for standard error shapes so you don't have to guess the format.
- Optimized for huge files (GBs) using streaming underlying readers.

## Examples

```
# Get the most recent logs
log_analyzer(operation="tail", file_path="/var/log/syslog", lines=50)

# Find specific user actions
log_analyzer(operation="search", file_path="app.log", query="user_login: 1042")

# Show all errors in the docker log
log_analyzer(operation="errors", file_path="/var/log/docker.log")
```

## Tips
- Avoid using `read_file` for logs; always use `log_analyzer` to avoid context overflow.
- If `errors` returns too much noise, use `search` with a more specific query.