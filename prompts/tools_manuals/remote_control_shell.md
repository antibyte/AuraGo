# remote_control_shell

Execute shell commands on connected remote-control devices.

Use only for device-specific shell work. Prefer focused file or desktop tools when not executing commands.

Use `execute_command` for one-shot commands that should finish quickly. Use `shell_session_start` only for interactive or long-running commands, then poll with `shell_session_read`; `initial_wait_ms` is just the first read wait, not a session lifetime.

Session operations: `shell_session_start` forwards `command`, optional `cwd_id`, and optional `initial_wait_ms`; `shell_session_read` forwards `session_id`, optional `offset`, `limit`, and `wait_ms`; `shell_session_input` forwards `session_id` and `input`; `shell_session_stop` stops a session; `shell_session_list` discovers client-owned sessions after reconnect.

Never summarize full shell output as an activity/status message. Read bounded chunks, inspect the result, and stop sessions you started when they are no longer needed.
