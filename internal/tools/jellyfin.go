package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/jellyfin"
	"aurago/internal/security"
)

const defaultJellyfinRequestTimeout = 60 * time.Second

func jellyfinRequestContext(cfg config.JellyfinConfig) (context.Context, context.CancelFunc) {
	timeout := time.Duration(cfg.RequestTimeout) * time.Second
	if timeout <= 0 {
		timeout = defaultJellyfinRequestTimeout
	}
	return context.WithTimeout(context.Background(), timeout)
}

func jellyfinReadOnlyMutationError(cfg config.JellyfinConfig, operation string) string {
	if cfg.ReadOnly {
		return errJSON("Jellyfin is in read-only mode; %s is disabled", operation)
	}
	return ""
}

func jellyfinDestructiveMutationError(cfg config.JellyfinConfig, operation string) string {
	if !cfg.AllowDestructive {
		return errJSON("Destructive Jellyfin operations are disabled. Set jellyfin.allow_destructive=true in config.yaml. Operation: %s", operation)
	}
	return ""
}

// DispatchJellyfinTool routes Jellyfin tool calls by operation name.
func DispatchJellyfinTool(operation string, params map[string]string, cfg *config.Config, vault *security.Vault, logger *slog.Logger) string {
	if !cfg.Jellyfin.Enabled {
		return errJSON("Jellyfin integration is disabled")
	}

	switch operation {
	case "health":
		return JellyfinHealth(cfg.Jellyfin, vault, logger)
	case "library_list":
		return JellyfinLibraryList(cfg.Jellyfin, vault, logger)
	case "search":
		query := getString(params, "query")
		mediaType := getString(params, "media_type", "")
		limit := getInt(params, "limit", 20)
		return JellyfinSearch(cfg.Jellyfin, vault, query, mediaType, limit, logger)
	case "item_details":
		itemID := getString(params, "item_id")
		return JellyfinItemDetails(cfg.Jellyfin, vault, itemID, logger)
	case "recent_items":
		limit := getInt(params, "limit", 20)
		mediaType := getString(params, "media_type", "")
		return JellyfinRecentItems(cfg.Jellyfin, vault, limit, mediaType, logger)
	case "sessions":
		return JellyfinSessions(cfg.Jellyfin, vault, logger)
	case "playback_control":
		sessionID := getString(params, "session_id")
		command := getString(params, "command")
		return JellyfinPlaybackControl(cfg.Jellyfin, vault, sessionID, command, logger)
	case "library_refresh":
		libraryID := getString(params, "library_id")
		return JellyfinLibraryRefresh(cfg.Jellyfin, vault, libraryID, logger)
	case "delete_item":
		itemID := getString(params, "item_id")
		return JellyfinDeleteItem(cfg.Jellyfin, vault, itemID, logger)
	case "activity_log":
		limit := getInt(params, "limit", 25)
		return JellyfinActivityLog(cfg.Jellyfin, vault, limit, logger)
	default:
		return errJSON("Unknown Jellyfin operation: %s", operation)
	}
}

// JellyfinHealth returns server info, version, and active session count.
func JellyfinHealth(cfg config.JellyfinConfig, vault *security.Vault, logger *slog.Logger) string {
	client, err := jellyfin.NewClient(cfg, vault)
	if err != nil {
		return errJSON("Jellyfin connection failed: %v", err)
	}
	defer client.Close()

	ctx, cancel := jellyfinRequestContext(cfg)
	defer cancel()

	info, err := client.GetSystemInfo(ctx)
	if err != nil {
		return errJSON("Failed to get system info: %v", err)
	}

	sessions, err := client.GetSessions(ctx)
	if err != nil {
		logger.Error("Failed to get sessions", "error", err)
	}

	activeSessions := 0
	for _, s := range sessions {
		if s.NowPlayingItem != nil {
			activeSessions++
		}
	}

	result, _ := json.Marshal(map[string]interface{}{
		"status": "ok",
		"health": map[string]interface{}{
			"server_name":      info.ServerName,
			"version":          info.Version,
			"os":               info.OperatingSystem,
			"local_address":    info.LocalAddress,
			"pending_restart":  info.HasPendingRestart,
			"update_available": info.HasUpdateAvailable,
			"active_sessions":  activeSessions,
			"total_sessions":   len(sessions),
			"timestamp":        time.Now().Format(time.RFC3339),
		},
	})
	return string(result)
}

// JellyfinLibraryList returns all media libraries with their types.
func JellyfinLibraryList(cfg config.JellyfinConfig, vault *security.Vault, logger *slog.Logger) string {
	client, err := jellyfin.NewClient(cfg, vault)
	if err != nil {
		return errJSON("Jellyfin connection failed: %v", err)
	}
	defer client.Close()

	ctx, cancel := jellyfinRequestContext(cfg)
	defer cancel()
	libs, err := client.GetLibraries(ctx)
	if err != nil {
		return errJSON("Failed to list libraries: %v", err)
	}

	type libraryInfo struct {
		Name           string   `json:"name"`
		ItemID         string   `json:"item_id"`
		CollectionType string   `json:"collection_type"`
		Locations      []string `json:"locations"`
	}

	items := make([]libraryInfo, 0, len(libs))
	for _, lib := range libs {
		items = append(items, libraryInfo{
			Name:           lib.Name,
			ItemID:         lib.ItemID,
			CollectionType: lib.CollectionType,
			Locations:      lib.Locations,
		})
	}

	result, _ := json.Marshal(map[string]interface{}{
		"status":    "ok",
		"libraries": items,
		"count":     len(items),
	})
	return string(result)
}

// JellyfinSearch searches for media items matching a query.
func JellyfinSearch(cfg config.JellyfinConfig, vault *security.Vault, query, mediaType string, limit int, logger *slog.Logger) string {
	if query == "" {
		return errJSON("search query is required")
	}

	client, err := jellyfin.NewClient(cfg, vault)
	if err != nil {
		return errJSON("Jellyfin connection failed: %v", err)
	}
	defer client.Close()

	ctx, cancel := jellyfinRequestContext(cfg)
	defer cancel()
	includeTypes := jellyfinMapMediaType(mediaType)

	resp, err := client.SearchItems(ctx, query, includeTypes, limit)
	if err != nil {
		return errJSON("Search failed: %v", err)
	}

	result, _ := json.Marshal(map[string]interface{}{
		"status":      "ok",
		"query":       query,
		"total_count": resp.TotalRecordCount,
		"items":       jellyfinFormatItems(resp.Items),
	})
	return string(result)
}

// JellyfinItemDetails returns detailed info for a specific item.
func JellyfinItemDetails(cfg config.JellyfinConfig, vault *security.Vault, itemID string, logger *slog.Logger) string {
	if itemID == "" {
		return errJSON("item_id is required")
	}

	client, err := jellyfin.NewClient(cfg, vault)
	if err != nil {
		return errJSON("Jellyfin connection failed: %v", err)
	}
	defer client.Close()

	ctx, cancel := jellyfinRequestContext(cfg)
	defer cancel()
	item, err := client.GetItem(ctx, itemID)
	if err != nil {
		return errJSON("Failed to get item: %v", err)
	}

	detail := map[string]interface{}{
		"name":     item.Name,
		"id":       item.ID,
		"type":     item.Type,
		"overview": item.Overview,
		"year":     item.ProductionYear,
		"rating":   item.CommunityRating,
		"genres":   item.Genres,
		"runtime":  jellyfinFormatRuntime(item.RunTimeTicks),
	}

	if item.OfficialRating != "" {
		detail["official_rating"] = item.OfficialRating
	}

	if item.SeriesName != "" {
		detail["series_name"] = item.SeriesName
		detail["season"] = item.SeasonName
		detail["episode_number"] = item.IndexNumber
		detail["season_number"] = item.ParentIndexNumber
	}

	if item.ChildCount > 0 {
		detail["child_count"] = item.ChildCount
	}

	if item.Path != "" {
		detail["path"] = item.Path
	}

	if item.PremiereDate != nil {
		detail["premiere_date"] = item.PremiereDate.Format("2006-01-02")
	}

	// User data (play state, favorites)
	if item.UserData != nil {
		ud := map[string]interface{}{
			"play_count":  item.UserData.PlayCount,
			"is_favorite": item.UserData.IsFavorite,
			"played":      item.UserData.Played,
		}
		if item.UserData.PlaybackPositionTicks > 0 {
			ud["position"] = jellyfinFormatRuntime(item.UserData.PlaybackPositionTicks)
		}
		if item.UserData.LastPlayedDate != "" {
			ud["last_played"] = item.UserData.LastPlayedDate
		}
		detail["user_data"] = ud
	}

	// Studios
	if len(item.Studios) > 0 {
		studios := make([]string, 0, len(item.Studios))
		for _, s := range item.Studios {
			studios = append(studios, s.Name)
		}
		detail["studios"] = studios
	}

	// People (actors, directors, writers) - top 10
	if len(item.People) > 0 {
		limit := len(item.People)
		if limit > 10 {
			limit = 10
		}
		people := make([]map[string]string, 0, limit)
		for i := 0; i < limit; i++ {
			p := item.People[i]
			entry := map[string]string{
				"name": p.Name,
				"type": p.Type,
			}
			if p.Role != "" {
				entry["role"] = p.Role
			}
			people = append(people, entry)
		}
		detail["people"] = people
	}

	// Media sources with stream details
	if len(item.MediaSources) > 0 {
		src := item.MediaSources[0]
		detail["container"] = src.Container
		detail["size_mb"] = src.Size / (1024 * 1024)

		if len(src.Streams) > 0 {
			streams := make([]map[string]interface{}, 0, len(src.Streams))
			for _, s := range src.Streams {
				stream := map[string]interface{}{
					"type":     s.Type,
					"codec":    s.Codec,
					"language": s.Language,
				}
				if s.Type == "Video" {
					if s.Width > 0 || s.Height > 0 {
						stream["resolution"] = fmt.Sprintf("%dx%d", s.Width, s.Height)
					}
					if s.BitRate > 0 {
						stream["bitrate"] = s.BitRate
					}
				}
				if s.Type == "Audio" {
					if s.Channels > 0 {
						stream["channels"] = s.Channels
					}
					if s.BitRate > 0 {
						stream["bitrate"] = s.BitRate
					}
				}
				streams = append(streams, stream)
			}
			detail["streams"] = streams
		}
	}

	result, _ := json.Marshal(map[string]interface{}{
		"status": "ok",
		"item":   detail,
	})
	return string(result)
}

// JellyfinRecentItems returns recently added media items.
func JellyfinRecentItems(cfg config.JellyfinConfig, vault *security.Vault, limit int, mediaType string, logger *slog.Logger) string {
	client, err := jellyfin.NewClient(cfg, vault)
	if err != nil {
		return errJSON("Jellyfin connection failed: %v", err)
	}
	defer client.Close()

	ctx, cancel := jellyfinRequestContext(cfg)
	defer cancel()
	includeTypes := jellyfinMapMediaType(mediaType)

	resp, err := client.GetRecentItems(ctx, includeTypes, limit)
	if err != nil {
		return errJSON("Failed to get recent items: %v", err)
	}

	result, _ := json.Marshal(map[string]interface{}{
		"status":      "ok",
		"total_count": resp.TotalRecordCount,
		"items":       jellyfinFormatItems(resp.Items),
	})
	return string(result)
}

// JellyfinSessions returns active playback sessions.
func JellyfinSessions(cfg config.JellyfinConfig, vault *security.Vault, logger *slog.Logger) string {
	client, err := jellyfin.NewClient(cfg, vault)
	if err != nil {
		return errJSON("Jellyfin connection failed: %v", err)
	}
	defer client.Close()

	ctx, cancel := jellyfinRequestContext(cfg)
	defer cancel()
	sessions, err := client.GetSessions(ctx)
	if err != nil {
		return errJSON("Failed to get sessions: %v", err)
	}

	type sessionInfo struct {
		ID         string      `json:"id"`
		UserName   string      `json:"user_name"`
		Client     string      `json:"client"`
		Device     string      `json:"device_name"`
		NowPlaying interface{} `json:"now_playing,omitempty"`
		IsPaused   bool        `json:"is_paused"`
		CanControl bool        `json:"supports_remote_control"`
	}

	items := make([]sessionInfo, 0, len(sessions))
	for _, s := range sessions {
		si := sessionInfo{
			ID:         s.ID,
			UserName:   s.UserName,
			Client:     s.Client,
			Device:     s.DeviceName,
			CanControl: s.SupportsRemoteControl,
		}
		if s.NowPlayingItem != nil {
			si.NowPlaying = map[string]interface{}{
				"name":    s.NowPlayingItem.Name,
				"type":    s.NowPlayingItem.Type,
				"id":      s.NowPlayingItem.ID,
				"runtime": jellyfinFormatRuntime(s.NowPlayingItem.RunTimeTicks),
			}
		}
		if s.PlayState != nil {
			si.IsPaused = s.PlayState.IsPaused
		}
		items = append(items, si)
	}

	result, _ := json.Marshal(map[string]interface{}{
		"status":   "ok",
		"sessions": items,
		"count":    len(items),
	})
	return string(result)
}

// JellyfinPlaybackControl sends a playback command to a session.
func JellyfinPlaybackControl(cfg config.JellyfinConfig, vault *security.Vault, sessionID, command string, logger *slog.Logger) string {
	if sessionID == "" {
		return errJSON("session_id is required")
	}
	if command == "" {
		return errJSON("command is required (Play, Pause, Unpause, Stop, NextTrack, PreviousTrack)")
	}
	if denied := jellyfinReadOnlyMutationError(cfg, "playback_control"); denied != "" {
		return denied
	}

	// Normalize command
	cmdMap := map[string]string{
		"play":     "Unpause",
		"pause":    "Pause",
		"unpause":  "Unpause",
		"stop":     "Stop",
		"next":     "NextTrack",
		"previous": "PreviousTrack",
	}
	normalized := cmdMap[strings.ToLower(command)]
	if normalized == "" {
		normalized = command // Pass through if already a valid Jellyfin command
	}

	client, err := jellyfin.NewClient(cfg, vault)
	if err != nil {
		return errJSON("Jellyfin connection failed: %v", err)
	}
	defer client.Close()

	ctx, cancel := jellyfinRequestContext(cfg)
	defer cancel()
	if err := client.SendPlayCommand(ctx, sessionID, normalized); err != nil {
		return errJSON("Playback control failed: %v", err)
	}

	result, _ := json.Marshal(map[string]interface{}{
		"status":     "ok",
		"message":    fmt.Sprintf("Sent '%s' command to session %s", normalized, sessionID),
		"session_id": sessionID,
		"command":    normalized,
	})
	return string(result)
}

// JellyfinLibraryRefresh triggers a library scan/refresh.
func JellyfinLibraryRefresh(cfg config.JellyfinConfig, vault *security.Vault, libraryID string, logger *slog.Logger) string {
	if libraryID == "" {
		return errJSON("library_id is required")
	}
	if denied := jellyfinReadOnlyMutationError(cfg, "library_refresh"); denied != "" {
		return denied
	}

	client, err := jellyfin.NewClient(cfg, vault)
	if err != nil {
		return errJSON("Jellyfin connection failed: %v", err)
	}
	defer client.Close()

	ctx, cancel := jellyfinRequestContext(cfg)
	defer cancel()
	if err := client.RefreshLibrary(ctx, libraryID); err != nil {
		return errJSON("Library refresh failed: %v", err)
	}

	result, _ := json.Marshal(map[string]interface{}{
		"status":     "ok",
		"message":    fmt.Sprintf("Library refresh started for %s", libraryID),
		"library_id": libraryID,
	})
	return string(result)
}

// JellyfinDeleteItem permanently deletes a media item.
func JellyfinDeleteItem(cfg config.JellyfinConfig, vault *security.Vault, itemID string, logger *slog.Logger) string {
	if itemID == "" {
		return errJSON("item_id is required")
	}
	if denied := jellyfinReadOnlyMutationError(cfg, "delete_item"); denied != "" {
		return denied
	}
	if denied := jellyfinDestructiveMutationError(cfg, "delete_item"); denied != "" {
		return denied
	}

	client, err := jellyfin.NewClient(cfg, vault)
	if err != nil {
		return errJSON("Jellyfin connection failed: %v", err)
	}
	defer client.Close()

	ctx, cancel := jellyfinRequestContext(cfg)
	defer cancel()
	if err := client.DeleteItem(ctx, itemID); err != nil {
		return errJSON("Failed to delete item: %v", err)
	}

	result, _ := json.Marshal(map[string]interface{}{
		"status":  "ok",
		"message": fmt.Sprintf("Item %s deleted", itemID),
		"item_id": itemID,
	})
	return string(result)
}

// JellyfinActivityLog returns recent activity log entries.
func JellyfinActivityLog(cfg config.JellyfinConfig, vault *security.Vault, limit int, logger *slog.Logger) string {
	client, err := jellyfin.NewClient(cfg, vault)
	if err != nil {
		return errJSON("Jellyfin connection failed: %v", err)
	}
	defer client.Close()

	ctx, cancel := jellyfinRequestContext(cfg)
	defer cancel()
	resp, err := client.GetActivityLog(ctx, limit)
	if err != nil {
		return errJSON("Failed to get activity log: %v", err)
	}

	type entry struct {
		Name     string `json:"name"`
		Type     string `json:"type"`
		Date     string `json:"date"`
		Severity string `json:"severity"`
		Overview string `json:"overview,omitempty"`
	}

	entries := make([]entry, 0, len(resp.Items))
	for _, e := range resp.Items {
		entries = append(entries, entry{
			Name:     e.Name,
			Type:     e.Type,
			Date:     e.Date,
			Severity: e.Severity,
			Overview: e.Overview,
		})
	}

	result, _ := json.Marshal(map[string]interface{}{
		"status":      "ok",
		"total_count": resp.TotalRecordCount,
		"entries":     entries,
	})
	return string(result)
}

// jellyfinMapMediaType maps user-friendly media type names to Jellyfin IncludeItemTypes.
func jellyfinMapMediaType(mediaType string) string {
	switch strings.ToLower(mediaType) {
	case "movie", "movies":
		return "Movie"
	case "series", "show", "shows", "tvshow":
		return "Series"
	case "episode", "episodes":
		return "Episode"
	case "music", "song", "songs", "audio":
		return "Audio"
	case "album", "albums":
		return "MusicAlbum"
	case "artist", "artists":
		return "MusicArtist"
	default:
		return mediaType // Pass through (empty or already correct)
	}
}

// jellyfinFormatItems formats a slice of MediaItems for tool output.
func jellyfinFormatItems(items []jellyfin.MediaItem) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		entry := map[string]interface{}{
			"name": item.Name,
			"id":   item.ID,
			"type": item.Type,
			"year": item.ProductionYear,
		}
		if item.CommunityRating > 0 {
			entry["rating"] = item.CommunityRating
		}
		if item.RunTimeTicks > 0 {
			entry["runtime"] = jellyfinFormatRuntime(item.RunTimeTicks)
		}
		if item.SeriesName != "" {
			entry["series"] = item.SeriesName
		}
		if item.DateCreated != nil {
			entry["date_added"] = item.DateCreated.Format("2006-01-02")
		}
		out = append(out, entry)
	}
	return out
}

// jellyfinFormatRuntime converts runtime ticks (100ns units) to a human-readable string.
func jellyfinFormatRuntime(ticks int64) string {
	if ticks <= 0 {
		return ""
	}
	d := time.Duration(ticks) * 100 * time.Nanosecond
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}
