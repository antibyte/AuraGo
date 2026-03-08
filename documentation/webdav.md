# WebDAV Integration

AuraGo can connect to any WebDAV-compatible cloud storage (Nextcloud, ownCloud, Synology, Box, etc.). This allows the agent to manage files in your cloud storage just like local files.

## Configuration

Add or update the `webdav` section in your `config.yaml`:

```yaml
webdav:
  enabled: true
  url: "https://your-cloud.example.com/remote.php/dav/files/username/"
  username: "your_username"
  password: "your_app_password" # Use an app-specific password if possible
```

> **Note for Nextcloud/ownCloud:** Use the "WebDAV URL" provided in the web interface (Files → Settings).

## Available Operations

The agent can perform the following operations via the `webdav` tool:

- **list**: List files and directories in a given path.
- **read**: Download and read the content of a text-based file.
- **write**: Create or overwrite a file with new content.
- **mkdir**: Create a new directory.
- **delete**: Permanently remove a file or directory.
- **move**: Rename or move a file or directory.
- **info**: Retrieve metadata (size, type, modification date) for a specific item.

## Security

- **App Passwords**: For services like Nextcloud, it is strongly recommended to generate a dedicated "App Password" rather than using your main account password.
- **Base URL**: The `url` should point to the root folder you want the agent to access. The agent cannot navigate "up" beyond this base URL.
- **TLS/SSL**: Always use `https://` to ensure your credentials and file data are encrypted in transit.

## Technical Details

AuraGo implements its own WebDAV client using standard Go `net/http` primitives. It supports:
- **PROPFIND** (Depth 0 and 1) for listing and metadata.
- **MKCOL** for directory creation.
- **PUT** / **GET** for file transfers.
- **DELETE** for removal.
- **MOVE** for renaming/moving.
- **Basic Authentication**.
