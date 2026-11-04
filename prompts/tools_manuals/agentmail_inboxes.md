# agentmail_inboxes

Manage AgentMail inboxes: test connection, list, read, create, update, and delete inboxes.

Mutating operations respect AgentMail read-only configuration.

`delete_inbox` also requires `agentmail.allow_delete: true`; it is disabled by default.
