# Google Workspace (`google_workspace`)

Access Google account services: Gmail, Calendar, Drive, Docs, and Sheets. Handles OAuth2 authentication automatically via the vault.

## Operations

| Operation | Description |
|-----------|-------------|
| `gmail_list` | List recent emails from inbox |
| `gmail_read` | Read full content of a specific email |
| `gmail_send` | Send an email |
| `gmail_modify_labels` | Add or remove labels on an email |
| `calendar_list` | List upcoming calendar events |
| `calendar_create` | Create a new calendar event |
| `calendar_update` | Update an existing calendar event |
| `drive_search` | Search for files in Google Drive |
| `drive_get_content` | Get content of a file from Drive |
| `docs_get` | Retrieve text content of a Google Doc |
| `docs_create` | Create a new Google Doc |
| `docs_update` | Replace content of a Google Doc |
| `sheets_get` | Read values from a Google Sheet |
| `sheets_update` | Update values in a Google Sheet |
| `sheets_create` | Create a new spreadsheet |

## Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `operation` | string | yes | One of the operations above |
| `query` | string | for gmail_list, drive_search | Search query |
| `max_results` | integer | no | Max items to return (default: 10) |
| `message_id` | string | for gmail_read, gmail_modify_labels | Email message ID |
| `to` | string | for gmail_send | Recipient email address |
| `subject` | string | for gmail_send | Email subject |
| `body` | string | for gmail_send, docs_update | Content text |
| `add_labels` | array | for gmail_modify_labels | Label IDs to add |
| `remove_labels` | array | for gmail_modify_labels | Label IDs to remove |
| `time_min` | string | for calendar_list | ISO timestamp start bound |
| `time_max` | string | for calendar_list | ISO timestamp end bound |
| `summary` | string | for calendar_create, calendar_update | Event title |
| `start_time` | string | for calendar_create, calendar_update | ISO timestamp |
| `end_time` | string | for calendar_create, calendar_update | ISO timestamp |
| `description` | string | for calendar_create, calendar_update | Event description |
| `event_id` | string | for calendar_update | Calendar event ID |
| `file_id` | string | for drive_get_content, docs_get, docs_update, sheets_get, sheets_update | Drive/Docs/Sheets file ID |
| `title` | string | for docs_create, sheets_create | Document/spreadsheet title |
| `range` | string | for sheets_get, sheets_update | A1 notation range |
| `values` | array | for sheets_update | 2D array of values |

## Examples

**Read recent emails:**
```json
{"action": "google_workspace", "operation": "gmail_list", "max_results": 3}
```

**Send an email:**
```json
{"action": "google_workspace", "operation": "gmail_send", "to": "colleague@example.com", "subject": "Meeting Notes", "body": "Here are the notes from today's meeting..."}
```

**Search Drive for a document:**
```json
{"action": "google_workspace", "operation": "drive_search", "query": "name contains 'Invoice'"}
```

**Create a calendar event:**
```json
{"action": "google_workspace", "operation": "calendar_create", "summary": "Team standup", "start_time": "2024-03-01T09:00:00Z", "end_time": "2024-03-01T09:15:00Z"}
```

**Read a Google Doc:**
```json
{"action": "google_workspace", "operation": "docs_get", "file_id": "1abc123..."}
```

**Update a Google Sheet:**
```json
{"action": "google_workspace", "operation": "sheets_update", "file_id": "1xyz...", "range": "A1:B2", "values": [["Name", "Value"], ["Item 1", 100]]}
```

## Configuration

```yaml
google_workspace:
  enabled: true
  # OAuth2 credentials are stored in the vault
  # Configure via Web UI: Settings → Integrations → Google Workspace
```

## Notes

- **OAuth2**: Authentication is handled automatically via OAuth2 stored in the vault. No manual token handling required.
- **Gmail queries**: Use Gmail search syntax (e.g., `from:boss is:unread has:attachment`)
- **Drive queries**: Use Google Drive search syntax (e.g., `name contains 'Invoice' and mimeType='application/pdf'`)
- **A1 notation**: Sheets ranges use standard A1 notation (e.g., `Sheet1!A1:B10` or just `A1:B10` for first sheet)
- **File IDs**: The file ID is the long string in the Google Drive URL, not the human-readable name
