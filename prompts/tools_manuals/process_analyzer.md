---
tool: process_analyzer
version: 1
tags: ["always"]
---

# Process Analyzer Tool

Analyze running OS processes on the host system. Find resource hogs, search for specific processes, inspect details, or view parent/child trees.

Unlike `process_management` (which manages AuraGo background tasks), this tool queries ALL system processes.

## Operations

| Operation | Description | Required Params |
|-----------|-------------|-----------------|
| `top_cpu` | List processes sorted by CPU usage | none (optional: `limit`) |
| `top_memory` | List processes sorted by memory usage | none (optional: `limit`) |
| `find` | Search for processes by name or command line | `name` |
| `tree` | Show process with its child processes | `pid` |
| `info` | Get detailed info about a specific process | `pid` |

## Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `operation` | string | **Required.** Operation to perform |
| `name` | string | Process name to search for (required for `find`) |
| `pid` | integer | Process ID (required for `tree` and `info`) |
| `limit` | integer | Max results (1-100, default: 10) |

## Output Fields

Each process entry includes:
- `pid` — Process ID
- `name` — Process name
- `status` — Running/sleeping/stopped
- `cpu_percent` — CPU usage percentage
- `mem_percent` — Memory usage percentage
- `mem_rss_bytes` — Resident memory in bytes
- `username` — User running the process
- `create_time` — Process start time (RFC3339)
- `num_threads` — Thread count
- `ppid` — Parent process ID
- `cmdline` — Full command line (for `find` and `info`)

## Examples

Find top CPU consumers:
```json
{"operation": "top_cpu", "limit": 5}
```

Find processes by name:
```json
{"operation": "find", "name": "nginx"}
```

Inspect a specific process:
```json
{"operation": "info", "pid": 1234}
```

View process tree:
```json
{"operation": "tree", "pid": 1}
```
