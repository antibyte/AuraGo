package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"aurago/internal/bluetooth"
	"aurago/internal/config"
	"aurago/internal/tools"
)

type bluetoothToolArgs struct {
	Operation      string
	Device         string
	LocalPath      string
	MediaID        int64
	Text           string
	Language       string
	TimeoutSeconds int
}

func decodeBluetoothToolArgs(tc ToolCall) bluetoothToolArgs {
	mediaID := int64(toolArgInt(tc.Params, 0, "media_id"))
	if mediaID == 0 {
		if raw := strings.TrimSpace(toolArgString(tc.Params, "media_id")); raw != "" {
			mediaID, _ = strconv.ParseInt(raw, 10, 64)
		}
	}
	return bluetoothToolArgs{
		Operation:      strings.ToLower(firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation"))),
		Device:         firstNonEmptyToolString(toolArgString(tc.Params, "device"), tc.DeviceAddr, tc.DeviceName),
		LocalPath:      toolArgString(tc.Params, "local_path"),
		MediaID:        mediaID,
		Text:           firstNonEmptyToolString(tc.Text, tc.Content, toolArgString(tc.Params, "text", "content")),
		Language:       firstNonEmptyToolString(tc.Language, toolArgString(tc.Params, "language")),
		TimeoutSeconds: toolArgInt(tc.Params, 0, "timeout_seconds"),
	}
}

func dispatchBluetooth(ctx context.Context, tc ToolCall, dc *DispatchContext) string {
	if dc == nil || dc.Cfg == nil {
		return bluetoothToolResult(nil, fmt.Errorf("Bluetooth configuration is unavailable"))
	}
	manager := bluetooth.DefaultManager()
	if manager == nil {
		return bluetoothToolResult(nil, &bluetooth.CodedError{Code: bluetooth.ErrorUnavailable, Message: "Bluetooth manager is unavailable."})
	}
	manager.Configure(config.BluetoothRuntimeOptions(dc.Cfg))
	request := decodeBluetoothToolArgs(tc)

	switch request.Operation {
	case "status":
		return bluetoothToolResult(map[string]interface{}{
			"status":   manager.Status(),
			"playback": manager.PlaybackStatus(),
		}, nil)
	case "list":
		devices, err := manager.List(ctx)
		return bluetoothToolResult(map[string]interface{}{"devices": devices}, err)
	case "discover":
		timeout := time.Duration(request.TimeoutSeconds) * time.Second
		devices, err := manager.Discover(ctx, timeout)
		return bluetoothToolResult(map[string]interface{}{"devices": devices}, err)
	case "pair":
		// Agent calls intentionally support Just Works only. A transient PIN can
		// only be supplied through the admin UI and is never exposed in schema.
		err := manager.Pair(ctx, request.Device, "")
		return bluetoothToolResult(map[string]interface{}{"device": request.Device}, err)
	case "connect":
		err := manager.Connect(ctx, request.Device)
		return bluetoothToolResult(map[string]interface{}{"device": request.Device}, err)
	case "disconnect":
		err := manager.Disconnect(ctx, request.Device)
		return bluetoothToolResult(map[string]interface{}{"device": request.Device}, err)
	case "play":
		source, err := resolveBluetoothPlaybackSource(dc.Cfg, dc.MediaRegistryDB, request)
		if err != nil {
			return bluetoothToolResult(nil, err)
		}
		playback, err := manager.Play(ctx, source, request.Device)
		return bluetoothToolResult(map[string]interface{}{"playback": playback}, err)
	case "speak":
		if !isTTSConfigured(dc.Cfg) {
			return bluetoothToolResult(nil, fmt.Errorf("TTS is not configured"))
		}
		if strings.TrimSpace(request.Text) == "" {
			return bluetoothToolResult(nil, &bluetooth.CodedError{Code: bluetooth.ErrorInvalidArgument, Message: "text is required for speak"})
		}
		ttsCfg := buildRuntimeTTSConfig(dc.Cfg, request.Language)
		filename, err := tools.TTSSynthesize(ttsCfg, request.Text)
		if err != nil {
			return bluetoothToolResult(nil, fmt.Errorf("synthesize Bluetooth TTS: %w", err))
		}
		source := filepath.Join(dc.Cfg.Directories.DataDir, "tts", filename)
		playback, err := manager.Play(ctx, source, request.Device)
		return bluetoothToolResult(map[string]interface{}{"playback": playback}, err)
	case "playback_status":
		if !dc.Cfg.Bluetooth.AllowPlayback || !dc.Cfg.Runtime.Bluetooth.Audio.Usable {
			return bluetoothToolResult(nil, &bluetooth.CodedError{Code: bluetooth.ErrorPlaybackDisabled, Message: "Bluetooth playback is not enabled and usable."})
		}
		return bluetoothToolResult(map[string]interface{}{"playback": manager.PlaybackStatus()}, nil)
	case "stop":
		if !dc.Cfg.Bluetooth.AllowPlayback || !dc.Cfg.Runtime.Bluetooth.Audio.Usable {
			return bluetoothToolResult(nil, &bluetooth.CodedError{Code: bluetooth.ErrorPlaybackDisabled, Message: "Bluetooth playback is not enabled and usable."})
		}
		err := manager.Stop()
		return bluetoothToolResult(map[string]interface{}{"playback": manager.PlaybackStatus()}, err)
	default:
		return bluetoothToolResult(nil, &bluetooth.CodedError{
			Code:    bluetooth.ErrorInvalidArgument,
			Message: "Unknown Bluetooth operation.",
		})
	}
}

func resolveBluetoothPlaybackSource(cfg *config.Config, mediaDB *sql.DB, request bluetoothToolArgs) (string, error) {
	hasLocalPath := strings.TrimSpace(request.LocalPath) != ""
	hasMediaID := request.MediaID > 0
	if hasLocalPath == hasMediaID {
		return "", &bluetooth.CodedError{
			Code:    bluetooth.ErrorInvalidArgument,
			Message: "play requires exactly one of local_path or media_id",
		}
	}
	if hasLocalPath {
		return resolveBluetoothLocalPath(cfg, request.LocalPath)
	}
	item, err := tools.GetMedia(mediaDB, request.MediaID)
	if err != nil {
		return "", fmt.Errorf("resolve Bluetooth media_id: %w", err)
	}
	switch strings.ToLower(strings.TrimSpace(item.MediaType)) {
	case "audio", "music", "tts":
	default:
		return "", &bluetooth.CodedError{
			Code:    bluetooth.ErrorInvalidArgument,
			Message: fmt.Sprintf("media_id %d is not an audio or music item", request.MediaID),
		}
	}
	return resolveBluetoothRegistryPath(cfg, item.FilePath)
}

func resolveBluetoothLocalPath(cfg *config.Config, rawPath string) (string, error) {
	rawPath = strings.TrimSpace(rawPath)
	slashPath := filepath.ToSlash(rawPath)
	switch {
	case strings.HasPrefix(slashPath, "/workdir/"):
		rawPath = filepath.Join(cfg.Directories.WorkspaceDir, filepath.FromSlash(strings.TrimPrefix(slashPath, "/workdir/")))
	case strings.HasPrefix(slashPath, "workdir/"):
		rawPath = filepath.Join(cfg.Directories.WorkspaceDir, filepath.FromSlash(strings.TrimPrefix(slashPath, "workdir/")))
	case strings.HasPrefix(slashPath, "/data/"):
		rawPath = filepath.Join(cfg.Directories.DataDir, filepath.FromSlash(strings.TrimPrefix(slashPath, "/data/")))
	case strings.HasPrefix(slashPath, "data/"):
		rawPath = filepath.Join(cfg.Directories.DataDir, filepath.FromSlash(strings.TrimPrefix(slashPath, "data/")))
	}
	resolved, err := resolveBluetoothRegistryPath(cfg, rawPath)
	if err != nil {
		return "", fmt.Errorf("resolve Bluetooth local_path: %w", err)
	}
	return resolved, nil
}

func bluetoothToolResult(payload map[string]interface{}, err error) string {
	if payload == nil {
		payload = make(map[string]interface{})
	}
	if err != nil {
		payload["status"] = "error"
		payload["message"] = err.Error()
		if code := bluetooth.ErrorCode(err); code != "" {
			payload["code"] = code
		}
	} else {
		payload["status"] = "ok"
	}
	encoded, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		return `Tool Output: {"status":"error","message":"Could not encode Bluetooth result."}`
	}
	return "Tool Output: " + string(encoded)
}

func resolveBluetoothRegistryPath(cfg *config.Config, rawPath string) (string, error) {
	rawPath = strings.TrimSpace(rawPath)
	if rawPath == "" {
		return "", fmt.Errorf("media registry item has no local file path")
	}
	candidates := []string{rawPath}
	if !filepath.IsAbs(rawPath) {
		candidates = []string{
			filepath.Join(cfg.Directories.WorkspaceDir, rawPath),
			filepath.Join(cfg.Directories.DataDir, rawPath),
		}
	}
	for _, candidate := range candidates {
		if resolved, ok := existingPathWithinRoots(candidate, cfg.Directories.WorkspaceDir, cfg.Directories.DataDir); ok {
			return resolved, nil
		}
	}
	return "", fmt.Errorf("media registry path is outside the configured workspace and data directories")
}

func existingPathWithinRoots(candidate string, roots ...string) (string, bool) {
	absolute, err := filepath.Abs(candidate)
	if err != nil {
		return "", false
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", false
	}
	info, err := os.Stat(resolved)
	if err != nil || info.IsDir() {
		return "", false
	}
	for _, root := range roots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		rootAbs, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		rootResolved, err := filepath.EvalSymlinks(rootAbs)
		if err != nil {
			rootResolved = rootAbs
		}
		relative, err := filepath.Rel(rootResolved, resolved)
		if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative) {
			return resolved, true
		}
	}
	return "", false
}
