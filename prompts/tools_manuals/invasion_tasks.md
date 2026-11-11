# invasion_tasks

Invasion must be enabled. Read-only blocks new tasks and secret delivery. Only
workers admitted by Bridge protocol 3 can receive work; report upgrade refusals
to the operator. Keep secret values out of task text and result messages.

Send, inspect, list, and cancel Invasion Control tasks.

Egg names are not tool names. Use `egg_id` or `egg_name` plus a task instruction for `send_task`.

Read-only blocks new work and all operator mutations, including stop. Authenticated results and artifacts of admitted work can still be saved; Egg messages cannot wake the agent. Disabled Invasion Control blocks every worker endpoint. A disconnected socket does not prove the Egg stopped: stop requires process/container confirmation. Upgrade Master and Eggs together before enabling the integration.
