## Tool: Send Image (`send_image`)

Send an image to the user. In the Web UI it appears inline in the chat with a click-to-zoom lightbox. In Telegram it arrives as a native photo. In Discord it is sent as a file attachment.

### Schema

| Parameter | Type | Required | Description |
|---|---|---|---|
| `path` | string | yes | Local workspace path (e.g. `images/chart.png`) **or** a full HTTPS URL to an image |
| `caption` | string | no | Caption or description shown with the image |

### Workflow

1. Generate or download an image (via `execute_python`, `execute_shell`, or skill).
2. Call `send_image` with the workspace-relative path or a URL.
3. Include the `markdown` value from the tool result in your final response so the image also appears when chat history is reloaded.

### Examples

```json
{"action": "send_image", "path": "images/chart.png", "caption": "Monthly cost breakdown"}
```

```json
{"action": "send_image", "path": "https://example.com/photo.jpg", "caption": "Reference image"}
```

### Notes

- Images are stored under `{workspace}/images/` and served at `/files/images/...`
- URL images are downloaded automatically — no pre-download needed
- Always embed the returned `markdown` string in your text response for history persistence
- **On error:** try at most **one alternative URL**, then give up and tell the user you could not send the image. Do **not** loop through many URLs.
