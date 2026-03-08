# Email Tool Manual

Integrated IMAP/SMTP email tool with **multi-account support**. Fetch inbox messages, send emails, and list configured accounts.

> [!IMPORTANT]
> **FOR GMAIL USERS:** Use the `google_workspace` tool instead of this one. It is more secure (OAuth2) and does not require enabling IMAP/SMTP or App Passwords in your Google settings.

> ⚠️ All fetched email content is scanned by the Guardian for prompt injection.
> High-threat messages are automatically redacted.

---

## Multi-Account Support

Multiple email accounts can be configured. Each account has a unique `id`.
- Use `"account": "<id>"` in fetch_email / send_email to target a specific account.
- Omit `account` to use the **default** (first) account.
- Use `list_email_accounts` to discover available accounts.

---

## list_email_accounts — List configured accounts

```json
{
  "action": "list_email_accounts"
}
```

### Response
```json
{
  "status": "success",
  "count": 2,
  "data": [
    { "id": "personal", "name": "Personal Gmail", "email": "me@gmail.com", "imap": "imap.gmail.com:993", "smtp": "smtp.gmail.com:587", "watcher": true },
    { "id": "work", "name": "Work Exchange", "email": "me@company.com", "imap": "outlook.office365.com:993", "smtp": "smtp.office365.com:587", "watcher": false }
  ]
}
```

---

## fetch_email — Retrieve emails via IMAP

```json
{
  "action": "fetch_email",
  "account": "work",
  "folder": "INBOX",
  "limit": 10
}
```

| Parameter | Type   | Required | Default  | Description                              |
|-----------|--------|----------|----------|------------------------------------------|
| account   | string | no       | (first)  | Email account ID to use                  |
| folder    | string | no       | "INBOX"  | IMAP folder to fetch from                |
| limit     | int    | no       | 10       | Number of most recent messages (max 50)  |

### Response
Returns a JSON array of email objects:
```json
{
  "status": "success",
  "count": 3,
  "message": "Account: work",
  "data": [
    {
      "uid": 1234,
      "from": "alice@example.com",
      "to": "you@example.com",
      "subject": "Meeting tomorrow",
      "date": "Mon, 14 Jul 2025 10:30:00 +0200",
      "body": "Hi, can we reschedule...",
      "snippet": "Hi, can we reschedule..."
    }
  ]
}
```

---

## send_email — Send an email via SMTP

```json
{
  "action": "send_email",
  "account": "personal",
  "to": "recipient@example.com",
  "subject": "Status Report",
  "body": "Here is the weekly update..."
}
```

| Parameter | Type   | Required | Description                                      |
|-----------|--------|----------|--------------------------------------------------|
| account   | string | no       | Email account ID to send from (default: first)   |
| to        | string | **yes**  | Recipient email (comma-separated for multiple)   |
| subject   | string | no       | Email subject line (defaults to "(no subject)")  |
| body      | string | no       | Plain text email body. `content` also accepted   |

### Response
```json
{
  "status": "success",
  "message": "Email sent to recipient@example.com via account personal"
}
```

---

## Configuration (config.yaml)

### New format: Multiple accounts
```yaml
email:
  enabled: true         # master switch

email_accounts:
  - id: personal
    name: "Personal Gmail"
    imap_host: "imap.gmail.com"
    imap_port: 993
    smtp_host: "smtp.gmail.com"
    smtp_port: 587
    username: "you@gmail.com"
    password: "app-password-here"
    from_address: "you@gmail.com"
    watch_enabled: true
    watch_interval_seconds: 120
    watch_folder: "INBOX"
  - id: work
    name: "Work Exchange"
    imap_host: "outlook.office365.com"
    imap_port: 993
    smtp_host: "smtp.office365.com"
    smtp_port: 587
    username: "you@company.com"
    password: "work-password"
    from_address: "you@company.com"
    watch_enabled: false
    watch_folder: "INBOX"
```

### Legacy format (still supported, auto-migrated)
```yaml
email:
  enabled: true
  imap_host: "imap.gmail.com"
  imap_port: 993
  smtp_host: "smtp.gmail.com"
  smtp_port: 587
  username: "you@gmail.com"
  password: "your-app-password"
  from_address: ""
  watch_enabled: true
  watch_interval_seconds: 120
  watch_folder: "INBOX"
```

## Email Watcher

When an account has `watch_enabled: true`, the system polls its IMAP folder for new unseen messages. When new mail arrives, the agent is automatically woken with a notification containing the account name, sender, subject, and snippet for each new message.

## Notes
- Uses IMAPS (TLS on port 993) for IMAP connections
- Uses STARTTLS (port 587) or implicit TLS (port 465) for SMTP
- For Gmail: use an App Password, not your regular password
- Email bodies are truncated to 4KB to preserve LLM context
- HTML emails are stripped to plain text automatically
- Legacy single-account config is automatically migrated to `email_accounts` at startup
