# Tool Manual: Google Workspace (`google_workspace`)

The `google_workspace` tool provides native access to your Google account services: Gmail, Calendar, Drive, Docs, and Sheets.

> [!TIP]
> **PREFER THIS TOOL FOR GMAIL:** Use this tool instead of the generic `fetch_email` or `send_email` tools when dealing with Gmail accounts. It handles OAuth2 authentication automatically via the vault.

## Recommended Usage

Use this tool when the user asks to:
- Check their emails or look for specific messages.
- Send an email.
- View upcoming calendar events or create new ones.
- Search for files in Google Drive.
- Read or write Google Documents.
- Read or update Google Sheets.

## Operations Reference

### Gmail

#### `gmail_list`
Lists recent emails from the primary inbox.
- `query` (optional): Gmail search query (e.g., `from:boss is:unread`)
- `max_results` (optional): Number of emails to fetch (default: 10)

#### `gmail_read`
Reads the full content of a specific email.
- `message_id` (required): The ID of the email to read

#### `gmail_send`
Sends an email.
- `to` (required): Recipient email address
- `subject` (required): Email subject
- `body` (required): Email body text

#### `gmail_modify_labels`
Adds or removes labels on an email.
- `message_id` (required): The ID of the email
- `add_labels` (optional): Array of label IDs to add
- `remove_labels` (optional): Array of label IDs to remove

### Calendar

#### `calendar_list`
Lists upcoming calendar events.
- `max_results` (optional): Number of events (default: 10)
- `time_min` (optional): ISO timestamp start bound
- `time_max` (optional): ISO timestamp end bound

#### `calendar_create`
Creates a new calendar event.
- `summary` (required): Event title
- `start_time` (required): ISO timestamp
- `end_time` (required): ISO timestamp
- `description` (optional): Event description

#### `calendar_update`
Updates an existing calendar event.
- `event_id` (required): The event ID
- `summary` (optional): Updated title
- `start_time` (optional): Updated start
- `end_time` (optional): Updated end
- `description` (optional): Updated description

### Drive

#### `drive_search`
Searches for files in Google Drive.
- `query` (required): Drive search query (e.g., `name contains 'Invoice'`)
- `max_results` (optional): Number of files (default: 10)

#### `drive_get_content`
Gets the content of a file from Drive.
- `file_id` (required): The file's Drive ID

### Docs

#### `docs_get`
Retrieves text content of a Google Doc.
- `file_id` (required): The document ID

#### `docs_create`
Creates a new Google Doc.
- `title` (required): Document title
- `body` (optional): Initial text content

#### `docs_update`
Replaces the content of a Google Doc.
- `file_id` (required): The document ID
- `body` (required): New text content

### Sheets

#### `sheets_get`
Reads values from a Google Sheet.
- `file_id` (required): The spreadsheet ID
- `range` (optional): A1 notation range (default: entire first sheet)

#### `sheets_update`
Updates values in a Google Sheet.
- `file_id` (required): The spreadsheet ID
- `range` (required): A1 notation range
- `values` (required): 2D array of values

#### `sheets_create`
Creates a new spreadsheet.
- `title` (required): Spreadsheet title

## Examples

**Read recent emails:**
```json
{
  "tool": "google_workspace",
  "operation": "gmail_list",
  "max_results": 3
}
```

**Search Drive for a document:**
```json
{
  "tool": "google_workspace",
  "operation": "drive_search",
  "query": "name contains 'Project Alpha'"
}
```

**Create a calendar event:**
```json
{
  "tool": "google_workspace",
  "operation": "calendar_create",
  "summary": "Team standup",
  "start_time": "2024-03-01T09:00:00Z",
  "end_time": "2024-03-01T09:15:00Z"
}
```
