# agentmail_messages

Read, label, send, reply, forward, delete, and inspect AgentMail messages.

Use `inbox_id` when not using the configured default inbox. Recipients are arrays of email addresses.

Writes require writable AgentMail. `delete_message` additionally requires
`agentmail.allow_delete: true` (default: false); ordinary sending does not grant deletion.
