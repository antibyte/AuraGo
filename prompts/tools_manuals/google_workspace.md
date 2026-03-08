# Tool Manual: Google Workspace (`google_workspace`)

The `google_workspace` tool provides access to your Google account services including Gmail, Calendar, Drive, and Google Docs.

> [!TIP]
> **PREFER THIS TOOL FOR GMAIL:** Use this tool instead of the generic `fetch_email` or `send_email` tools when dealing with Gmail accounts. It handles OAuth2 authentication automatically via the vault.

## Recommended Usage

Use this tool when the user asks to:
- Check their emails or look for specific messages.
- View upcoming calendar events.
- Search for files in Google Drive.
- Read or write content to Google Documents.

## Action Reference

### `read_emails`
Lists recent emails from the primary inbox.
- `max_results` (optional): Number of emails to fetch (default: 5).

### `get_events`
Lists upcoming calendar events starting from now.
- `max_results` (optional): Number of events to fetch (default: 10).
- `time_min` (optional): ISO format timestamp (e.g., `2024-03-01T00:00:00Z`).

### `search_drive`
Searches for files in Google Drive.
- `query` (optional): Drive search query (e.g., `name contains 'Invoice'`).
- `max_results` (optional): Number of files to fetch (default: 5).

### `read_document`
Retrieves the text content and metadata of a Google Doc.
- `document_id` (required): The unique ID of the document.

### `write_document`
Creates or appends text to a Google Doc.
- `document_id` (optional): If provided, updates existing doc. If omitted, creates a new one.
- `title` (optional): Title for a new document.
- `text` (required): Content to write/append.
- `append` (optional): If `true`, adds to the end. If `false`, overwrites (default: `true`).

## Examples

**Read recent emails:**
```json
{
  "tool": "google_workspace",
  "action": "read_emails",
  "max_results": 3
}
```

**Search for a document:**
```json
{
  "tool": "google_workspace",
  "action": "search_drive",
  "query": "name contains 'Project Alpha'"
}
```
