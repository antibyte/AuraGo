# invasion_tasks

Invasion must be enabled. Read-only blocks new tasks and secret delivery. Only
workers admitted by Bridge protocol 3 can receive work; report upgrade refusals
to the operator. Keep secret values out of task text and result messages.

Send, inspect, list, and cancel Invasion Control tasks.

Egg names are not tool names. Use `egg_id` or `egg_name` plus a task instruction for `send_task`.

Nest-name lookup is case-insensitive and must match exactly one nest. If the name
is ambiguous, list the nests and use the intended `nest_id`; never guess a target.
