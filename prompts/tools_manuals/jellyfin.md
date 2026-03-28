# Jellyfin Media Server Tool

The `jellyfin` tool allows you to manage and interact with a Jellyfin media server. It supports browsing libraries, searching media, monitoring sessions, controlling playback, and server administration.

## Operations

### health
Check server health, version, and active session count.
```
jellyfin(operation="health")
```

### library_list
List all media libraries with their types and paths.
```
jellyfin(operation="library_list")
```

### search
Search for media items by name. Optionally filter by media type.
```
jellyfin(operation="search", query="The Matrix", media_type="movie")
jellyfin(operation="search", query="Beatles", media_type="music", limit=10)
```
Supported media types: movie, series, episode, music, album, artist

### item_details
Get detailed information about a specific media item.
```
jellyfin(operation="item_details", item_id="abc123")
```

### recent_items
Get recently added media items.
```
jellyfin(operation="recent_items", limit=10)
jellyfin(operation="recent_items", media_type="movie", limit=5)
```

### sessions
List active playback sessions with user, device, and now-playing info.
```
jellyfin(operation="sessions")
```

### playback_control
Send playback commands to an active session. **Requires read-only mode to be off.**
```
jellyfin(operation="playback_control", session_id="sess123", command="pause")
jellyfin(operation="playback_control", session_id="sess123", command="play")
jellyfin(operation="playback_control", session_id="sess123", command="stop")
jellyfin(operation="playback_control", session_id="sess123", command="next")
jellyfin(operation="playback_control", session_id="sess123", command="previous")
```

### library_refresh
Trigger a library scan/refresh. **Requires read-only mode to be off.**
```
jellyfin(operation="library_refresh", library_id="lib123")
```

### delete_item
Permanently delete a media item. **Requires allow_destructive to be enabled.**
```
jellyfin(operation="delete_item", item_id="abc123")
```

### activity_log
View recent server activity log entries.
```
jellyfin(operation="activity_log", limit=25)
```

## Permission Levels

| Operation | Minimum Permission |
|-----------|-------------------|
| health, library_list, search, item_details, recent_items, sessions, activity_log | Enabled |
| playback_control, library_refresh | Enabled + Read-Only off |
| delete_item | Enabled + Read-Only off + Allow Destructive |

## Configuration

Set in config.yaml:
```yaml
jellyfin:
  enabled: true
  read_only: false           # Block playback control & library refresh
  allow_destructive: false   # Block item deletion
  host: "jellyfin.local"
  port: 8096
  use_https: false
  insecure_ssl: false
```

The API key is stored securely in the vault under the key `jellyfin_api_key`. Configure it via the Web UI under Settings → Jellyfin.

## Common Workflows

- Check what's playing: `jellyfin(operation="sessions")`
- Find a movie: `jellyfin(operation="search", query="Inception", media_type="movie")`
- Browse recent additions: `jellyfin(operation="recent_items", limit=10)`
- Server health check: `jellyfin(operation="health")`
- Pause current playback: First get session ID from `sessions`, then `jellyfin(operation="playback_control", session_id="...", command="pause")`
