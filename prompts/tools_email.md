---
id: "tools_email"
tags: ["conditional"]
priority: 31
conditions: ["email_enabled"]
---
### Email (Multi-Account)
| Tool | Purpose |
|---|---|
| `fetch_email` | Retrieve recent emails from IMAP inbox (Standard IMAP only. ⚠️ FOR GMAIL USE google_workspace) |
| `send_email` | Send an email via SMTP, optionally with `attachments` (list of workspace or documents-folder files; max 10, 20 MB) (⚠️ FOR GMAIL USE google_workspace) |
| `list_email_accounts` | List all configured email accounts with their IDs |

**Multi-Account:** Use `"account": "<id>"` to target a specific email account. Omit to use the default (first) account. Use `list_email_accounts` to discover available accounts.
