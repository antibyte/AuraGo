# Wait For Event (`wait_for_event`)

Wait asynchronously for a concrete event and continue autonomously once it happens.

Use this when the next step depends on:

- an AuraGo-managed background process finishing
- an HTTP endpoint becoming reachable
- a workspace file being created or changed

## Event Types

| Event Type | Description |
|------------|-------------|
| `process_exited` | Wait for a background process to exit |
| `http_available` | Wait for an HTTP endpoint to respond |
| `file_changed` | Wait for a workspace file to be created or modified |

## When to use

- A shell/Python/background tool is still running and you want to continue after it exits
- A dev server or deployed site needs to come up before the next verification step
- A file is expected to appear or change and you want to continue automatically afterward

## When NOT to use

- For vague, human, or external events with no concrete check
- When you can proceed immediately without waiting
- To ask the user a question later

## Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `event_type` | string | yes | `process_exited`, `http_available`, or `file_changed` |
| `task_prompt` | string | yes | Task to continue with after the event occurs |
| `pid` | integer | for process_exited | AuraGo background process ID |
| `url` | string | for http_available | Full URL to probe |
| `host` | string | optional | Host for http_available when url is omitted |
| `port` | integer | optional | Port for http_available when host is used |
| `file_path` | string | for file_changed | Workspace file to watch |
| `timeout_secs` | integer | no | Max wait duration |
| `interval_seconds` | integer | no | Poll interval |

## Examples

```json
{"action":"wait_for_event","event_type":"process_exited","pid":12345,"task_prompt":"Inspect the finished process output and summarize the result."}
```

```json
{"action":"wait_for_event","event_type":"http_available","url":"http://127.0.0.1:4173","task_prompt":"Open the homepage in a screenshot and verify the layout."}
```

```json
{"action":"wait_for_event","event_type":"file_changed","file_path":"reports/build.log","task_prompt":"Read the updated log file and extract the failure cause."}
```

## Notes

- **Timeout**: Default timeout is 1 hour. Set `timeout_secs` to limit wait time.
- **Poll interval**: Default is 2 seconds. Use `interval_seconds` to adjust frequency.
- **PID lookup**: Use `process_analyzer` tool to find background process IDs.
- **URL probing**: For `http_available`, the agent waits until HTTP returns 2xx or 3xx.
- **File watching**: For `file_changed`, the agent watches for file creation or modification.
