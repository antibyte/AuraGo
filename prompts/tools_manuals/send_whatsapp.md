---
id: "tool_send_whatsapp"
tags: ["tool", "whatsapp", "messaging"]
conditions: ["whatsapp_enabled"]
---
# WhatsApp (`send_whatsapp`)

Send a message, and optionally one file, to the owner's WhatsApp chat through the linked WhatsApp device. The tool is offered only while WhatsApp is enabled, linked and not read-only.

## Schema

```json
{"action": "send_whatsapp", "message": "Backup finished, 3.2 GB archived.", "file_path": "backup-report.pdf"}
```

## Parameters

| Parameter | Required | Description |
|-----------|----------|-------------|
| `message` | yes | Message text. Markdown is converted to WhatsApp formatting. With a file, it becomes the caption, so send text with every file |
| `file_path` | no | One file in the workspace or the documents folder, up to 50 MB |

`content` and `body` are accepted as aliases for `message`.

## Owner only

There is no recipient parameter. Messages go only to the owner:

- `dedicated` mode: the numbers in `whatsapp.owner_numbers`.
- `self_chat` mode: the linked account's own chat. Each text and caption starts with `whatsapp.self_chat_prefix`.

Any other argument, such as a phone number, is ignored.

## File Types and Limits

- JPEG and PNG go out as images. MP4 (`.mp4`, `.m4v`) goes out as video. MP3, M4A and OGG (also Opus and AAC) go out as audio.
- A file goes out as an image, video or audio only when its content matches that type. Anything else goes out as a document under its file name.
- The limit is 50 MB. Empty files, folders and larger files are refused before anything is sent.
- The file must lie in the workspace or the documents folder. Configuration, vault, databases and the master key are never sent.
- Only `file_path` names a file. A stray `path` argument sends nothing.

## Caption and Text Order

- With a file, the message becomes its caption. A caption holds at most 1024 characters, counted after Markdown conversion and including any self-chat prefix.
- Text that does not fit goes out first as its own message, and the file follows without a caption. Long texts are split into several messages.
- Audio has no caption, so its text always goes out first.
- If the text cannot be sent, no file follows it. If the file fails after the text went out, the result has `"text_sent": true`. A retry then sends the text again.

## Read-only

When `whatsapp.readonly` is true, the tool is not offered. If it is called anyway, it returns an error and sends nothing. Disabled WhatsApp or a missing device link also returns an error.

## When to Use `send_notification` Instead

Use `send_notification` with `channel: "whatsapp"` for a short alert with a title. The title appears in bold above the message, and no file is sent. `channel: "all"` also reaches WhatsApp when it is enabled and linked.

Use `send_whatsapp` for a chat message, for a file, or for audio and video.

## Examples

**Text only:**
```json
{"action": "send_whatsapp", "message": "Backup finished. 3.2 GB archived, no errors."}
```

**Message with a file:**
```json
{"action": "send_whatsapp", "message": "Here is the **monthly report**.", "file_path": "report.pdf"}
```
