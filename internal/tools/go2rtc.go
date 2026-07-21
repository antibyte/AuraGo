package tools

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
)

const (
	go2RTCAPIUser          = "aurago"
	go2RTCMaxResponseBytes = 20 << 20
	go2RTCMaxCacheEntries  = 16
	go2RTCMaxCacheBytes    = 64 << 20
	go2RTCMaxStoredItems   = 1000
	go2RTCMaxStoredBytes   = int64(2 << 30)
)

// Go2RTCStreamStatus is the deliberately sanitized public stream view.
type Go2RTCStreamStatus struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Reachable bool     `json:"reachable"`
	Codecs    []string `json:"codecs"`
	Producers int      `json:"producers"`
	Consumers int      `json:"consumers"`
}

// Go2RTCStatus distinguishes configuration, container, and API state.
type Go2RTCStatus struct {
	Configured       bool   `json:"configured"`
	Enabled          bool   `json:"enabled"`
	ContainerRunning bool   `json:"container_running"`
	APIUsable        bool   `json:"api_usable"`
	Version          string `json:"version,omitempty"`
	LastError        string `json:"last_error,omitempty"`
	LastChecked      string `json:"last_checked,omitempty"`
}

// Go2RTCSnapshotOptions contains safe, bounded snapshot transformations.
type Go2RTCSnapshotOptions struct {
	Width        int
	Height       int
	Rotate       int
	CacheSeconds int
	Store        bool
}

// Go2RTCSnapshotResult describes a verified JPEG snapshot.
type Go2RTCSnapshotResult struct {
	Status      string `json:"status"`
	StreamID    string `json:"stream_id"`
	ContentType string `json:"content_type"`
	Bytes       int    `json:"bytes"`
	Stored      bool   `json:"stored"`
	LocalPath   string `json:"-"`
	WebPath     string `json:"web_path,omitempty"`
	SHA256      string `json:"sha256"`
	MediaID     int64  `json:"media_id,omitempty"`
	Cached      bool   `json:"cached,omitempty"`
}

type go2RTCSnapshotCacheEntry struct {
	data      []byte
	expiresAt time.Time
	lastUsed  time.Time
}

// Go2RTCManager owns AuraGo's client, sidecar lifecycle, and runtime reconciliation.
type Go2RTCManager struct {
	lifecycleMu  sync.Mutex
	credentialMu sync.Mutex
	mu           sync.RWMutex
	cfg          config.Go2RTCConfig
	docker       DockerConfig
	dataDir      string
	configDir    string
	vault        *security.Vault
	mediaDB      *sql.DB
	logger       *slog.Logger
	client       *http.Client
	status       Go2RTCStatus
	cache        map[string]go2RTCSnapshotCacheEntry
	cacheBytes   int64
	cancel       context.CancelFunc
	inDocker     bool
	manualStop   bool
}

var defaultGo2RTCManager atomic.Pointer[Go2RTCManager]

// SetDefaultGo2RTCManager publishes the server-owned manager to native tools.
func SetDefaultGo2RTCManager(manager *Go2RTCManager) {
	defaultGo2RTCManager.Store(manager)
}

// DefaultGo2RTCManager returns the server-owned manager, when initialized.
func DefaultGo2RTCManager() *Go2RTCManager {
	return defaultGo2RTCManager.Load()
}

// NewGo2RTCManager creates a manager and hydrates its vault-only credentials.
func NewGo2RTCManager(cfg *config.Config, vault *security.Vault, mediaDB *sql.DB, logger *slog.Logger) *Go2RTCManager {
	if logger == nil {
		logger = slog.Default()
	}
	m := &Go2RTCManager{
		vault:   vault,
		mediaDB: mediaDB,
		logger:  logger,
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				Proxy:                 http.ProxyFromEnvironment,
				MaxIdleConns:          10,
				MaxIdleConnsPerHost:   5,
				IdleConnTimeout:       60 * time.Second,
				ResponseHeaderTimeout: 15 * time.Second,
			},
		},
		cache: make(map[string]go2RTCSnapshotCacheEntry),
	}
	m.Configure(cfg)
	return m
}

// Configure atomically applies the safe runtime config and rehydrates secrets.
func (m *Go2RTCManager) Configure(cfg *config.Config) {
	if m == nil || cfg == nil {
		return
	}
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()
	m.configureLocked(cfg)
}

func (m *Go2RTCManager) configureLocked(cfg *config.Config) {
	m.credentialMu.Lock()
	defer m.credentialMu.Unlock()

	goCfg := cfg.Go2RTC
	if strings.TrimSpace(goCfg.Image) == "" {
		goCfg.Image = config.Go2RTCDefaultImage
	}
	if strings.TrimSpace(goCfg.ContainerName) == "" {
		goCfg.ContainerName = "aurago_go2rtc"
	}
	if goCfg.APIHostPort <= 0 {
		goCfg.APIHostPort = 1984
	}
	if goCfg.WebRTC.Port <= 0 {
		goCfg.WebRTC.Port = 8555
	}
	for i := range goCfg.Streams {
		if m.vault != nil {
			if value, err := m.vault.ReadSecret(config.Go2RTCStreamSourceVaultKey(goCfg.Streams[i].ID)); err == nil {
				goCfg.Streams[i].Source = strings.TrimSpace(value)
				goCfg.Streams[i].SourceConfigured = goCfg.Streams[i].Source != ""
				security.RegisterSensitive(goCfg.Streams[i].Source)
			}
		}
	}
	if m.vault != nil {
		if value, err := m.vault.ReadSecret(config.Go2RTCAPIPasswordVaultKey); err == nil {
			goCfg.APIPassword = strings.TrimSpace(value)
		}
	}
	if strings.TrimSpace(goCfg.APIPassword) != "" {
		security.RegisterSensitive(goCfg.APIPassword)
	}
	m.mu.Lock()
	wasEnabled := m.cfg.Enabled
	m.cfg = goCfg
	m.docker = DockerConfig{Host: cfg.Docker.Host}
	m.dataDir = cfg.Directories.DataDir
	if strings.TrimSpace(m.dataDir) == "" {
		m.dataDir = "data"
	}
	m.configDir = filepath.Join(m.dataDir, "go2rtc")
	m.inDocker = cfg.Runtime.IsDocker || browserAutomationRunsInDocker()
	m.status.Configured = len(goCfg.Streams) > 0
	m.status.Enabled = goCfg.Enabled
	m.cache = make(map[string]go2RTCSnapshotCacheEntry)
	m.cacheBytes = 0
	if !goCfg.Enabled || !wasEnabled {
		m.manualStop = false
	}
	m.mu.Unlock()
}

// StartBackground launches lifecycle monitoring and performs the initial reconcile.
func (m *Go2RTCManager) StartBackground(parent context.Context) {
	if m == nil {
		return
	}
	m.mu.Lock()
	if m.cancel != nil {
		m.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(parent)
	m.cancel = cancel
	m.mu.Unlock()
	go func() {
		m.reconcileTick(ctx)
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.reconcileTick(ctx)
			}
		}
	}()
}

// Close stops background reconciliation.
func (m *Go2RTCManager) Close() {
	if m == nil {
		return
	}
	m.mu.Lock()
	cancel := m.cancel
	m.cancel = nil
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (m *Go2RTCManager) reconcileTick(ctx context.Context) {
	cfg := m.Config()
	if !cfg.Enabled {
		m.setAPIStatus(false, "", "")
		return
	}
	m.mu.RLock()
	manualStop := m.manualStop
	m.mu.RUnlock()
	if manualStop {
		m.setAPIStatus(false, "", "")
		return
	}
	if cfg.AutoStart {
		if err := m.StartContainer(ctx); err != nil {
			m.setAPIStatus(false, "", err.Error())
			return
		}
	} else if _, err := m.Test(ctx); err != nil {
		return
	}
	if _, err := m.ReconcileStreams(ctx); err != nil {
		m.setAPIStatus(false, "", err.Error())
		return
	}
	_, _ = m.Test(ctx)
}

// Config returns a copy of the current integration config.
func (m *Go2RTCManager) Config() config.Go2RTCConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg := m.cfg
	cfg.Streams = append([]config.Go2RTCStreamConfig(nil), m.cfg.Streams...)
	return cfg
}

// Available reports whether agent use is currently safe and functional.
func (m *Go2RTCManager) Available() bool {
	if m == nil {
		return false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg.Enabled && m.cfg.AgentAccess && m.status.APIUsable
}

// Status returns the current non-secret lifecycle view.
func (m *Go2RTCManager) Status(ctx context.Context) Go2RTCStatus {
	if m == nil {
		return Go2RTCStatus{LastError: "go2rtc manager is not initialized"}
	}
	m.mu.RLock()
	status := m.status
	cfg := m.cfg
	docker := m.docker
	m.mu.RUnlock()
	status.Enabled = cfg.Enabled
	status.Configured = len(cfg.Streams) > 0
	owner := m.go2RTCOwner()
	status.ContainerRunning = go2RTCContainerRunning(docker, cfg.ContainerName, owner)
	if ctx != nil && cfg.Enabled {
		tested, err := m.Test(ctx)
		if err == nil {
			status = tested
			status.ContainerRunning = go2RTCContainerRunning(docker, cfg.ContainerName, owner)
		}
	}
	return status
}

// Test probes the authenticated go2rtc API without exposing credentials.
func (m *Go2RTCManager) Test(ctx context.Context) (Go2RTCStatus, error) {
	var raw interface{}
	err := m.requestJSON(ctx, http.MethodGet, "/api", nil, &raw, 1<<20)
	version := ""
	if object, ok := raw.(map[string]interface{}); ok {
		version = sanitizeGo2RTCToken(firstString(object, "version", "go2rtc"), 32)
	}
	if version == "" {
		version = "reachable"
	}
	if err != nil {
		m.setAPIStatus(false, "", err.Error())
		return m.currentStatus(), err
	}
	m.setAPIStatus(true, version, "")
	return m.currentStatus(), nil
}

func (m *Go2RTCManager) currentStatus() Go2RTCStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

func (m *Go2RTCManager) setAPIStatus(usable bool, version, lastError string) {
	m.mu.Lock()
	m.status.APIUsable = usable
	m.status.Version = version
	m.status.LastError = lastError
	m.status.LastChecked = time.Now().UTC().Format(time.RFC3339)
	m.status.Enabled = m.cfg.Enabled
	m.status.Configured = len(m.cfg.Streams) > 0
	m.mu.Unlock()
}

// ReconcileStreams injects enabled vault-only sources through go2rtc's runtime API.
func (m *Go2RTCManager) ReconcileStreams(ctx context.Context) (int, error) {
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()
	return m.reconcileStreamsLocked(ctx)
}

func (m *Go2RTCManager) reconcileStreamsLocked(ctx context.Context) (int, error) {
	cfg := m.Config()
	if !cfg.Enabled {
		return 0, fmt.Errorf("go2rtc integration is disabled")
	}
	count := 0
	for _, stream := range cfg.Streams {
		if !stream.Enabled {
			continue
		}
		if err := ValidateGo2RTCStreamID(stream.ID); err != nil {
			return count, err
		}
		if err := ValidateGo2RTCSource(stream.Source); err != nil {
			return count, fmt.Errorf("stream %q source: %w", stream.ID, err)
		}
		query := url.Values{"name": {Go2RTCRuntimeStreamName(stream.ID)}, "src": {stream.Source}}
		if err := m.request(ctx, http.MethodPatch, "/api/streams?"+query.Encode(), nil, 1<<20, nil); err != nil {
			return count, fmt.Errorf("reconcile stream %q: %w", stream.ID, err)
		}
		count++
	}
	return count, nil
}

// ListStreams returns only configured, enabled streams and sanitized telemetry.
func (m *Go2RTCManager) ListStreams(ctx context.Context) ([]Go2RTCStreamStatus, error) {
	var upstream map[string]interface{}
	if err := m.requestJSON(ctx, http.MethodGet, "/api/streams", nil, &upstream, 4<<20); err != nil {
		return nil, err
	}
	cfg := m.Config()
	result := make([]Go2RTCStreamStatus, 0, len(cfg.Streams))
	for _, stream := range cfg.Streams {
		if !stream.Enabled {
			continue
		}
		entry := Go2RTCStreamStatus{ID: stream.ID, Name: stream.Name, Codecs: []string{}}
		if raw, ok := upstream[Go2RTCRuntimeStreamName(stream.ID)].(map[string]interface{}); ok {
			entry.Producers = sliceLength(raw["producers"])
			entry.Consumers = sliceLength(raw["consumers"])
			entry.Codecs = collectGo2RTCCodecs(raw)
			// A configured go2rtc source already appears as one producer even before
			// it has connected. Runtime codec metadata is the passive signal that a
			// producer has actually been initialized successfully.
			entry.Reachable = len(entry.Codecs) > 0
		}
		result = append(result, entry)
	}
	return result, nil
}

// StreamStatus returns one configured, enabled stream.
func (m *Go2RTCManager) StreamStatus(ctx context.Context, streamID string) (Go2RTCStreamStatus, error) {
	streamID = strings.TrimSpace(streamID)
	streams, err := m.ListStreams(ctx)
	if err != nil {
		return Go2RTCStreamStatus{}, err
	}
	for _, stream := range streams {
		if stream.ID == streamID {
			return stream, nil
		}
	}
	return Go2RTCStreamStatus{}, fmt.Errorf("stream %q is not configured or enabled", streamID)
}

// Snapshot fetches, validates, optionally stores, and registers a JPEG snapshot.
func (m *Go2RTCManager) Snapshot(ctx context.Context, streamID string, opts Go2RTCSnapshotOptions) (Go2RTCSnapshotResult, []byte, error) {
	stream, data, cached, err := m.fetchSnapshotBytes(ctx, streamID, opts)
	if err != nil {
		return Go2RTCSnapshotResult{}, nil, err
	}
	result, err := m.buildSnapshotResult(stream, data, opts)
	result.Cached = cached
	return result, data, err
}

// SnapshotBytes fetches and validates a JPEG without ever persisting it. It is
// used by high-frequency thumbnail views even when store_media is enabled.
func (m *Go2RTCManager) SnapshotBytes(ctx context.Context, streamID string, opts Go2RTCSnapshotOptions) (Go2RTCSnapshotResult, []byte, error) {
	stream, data, cached, err := m.fetchSnapshotBytes(ctx, streamID, opts)
	if err != nil {
		return Go2RTCSnapshotResult{}, nil, err
	}
	result := snapshotMetadata(stream.ID, data)
	result.Cached = cached
	return result, data, nil
}

func (m *Go2RTCManager) fetchSnapshotBytes(ctx context.Context, streamID string, opts Go2RTCSnapshotOptions) (config.Go2RTCStreamConfig, []byte, bool, error) {
	stream, err := m.findEnabledStream(streamID)
	if err != nil {
		return config.Go2RTCStreamConfig{}, nil, false, err
	}
	if opts.Width < 0 || opts.Width > 7680 || opts.Height < 0 || opts.Height > 4320 {
		return config.Go2RTCStreamConfig{}, nil, false, fmt.Errorf("snapshot size is outside the supported bounds")
	}
	switch opts.Rotate {
	case 0, 90, 180, 270:
	default:
		return config.Go2RTCStreamConfig{}, nil, false, fmt.Errorf("rotation must be 0, 90, 180, or 270")
	}
	if opts.CacheSeconds < 0 || opts.CacheSeconds > 3600 {
		return config.Go2RTCStreamConfig{}, nil, false, fmt.Errorf("cache duration must be between 0 and 3600 seconds")
	}
	cacheKey := fmt.Sprintf("%s:%d:%d:%d", stream.ID, opts.Width, opts.Height, opts.Rotate)
	if opts.CacheSeconds > 0 {
		now := time.Now()
		m.mu.Lock()
		m.pruneSnapshotCacheLocked(now, 0)
		cached, ok := m.cache[cacheKey]
		if ok && now.Before(cached.expiresAt) {
			cached.lastUsed = now
			m.cache[cacheKey] = cached
		}
		m.mu.Unlock()
		if ok && now.Before(cached.expiresAt) {
			return stream, append([]byte(nil), cached.data...), true, nil
		}
	}
	query := url.Values{"src": {Go2RTCRuntimeStreamName(stream.ID)}}
	if opts.Width > 0 {
		query.Set("width", strconv.Itoa(opts.Width))
	}
	if opts.Height > 0 {
		query.Set("height", strconv.Itoa(opts.Height))
	}
	if opts.Rotate != 0 {
		query.Set("rotate", strconv.Itoa(opts.Rotate))
	}
	var contentType string
	data, err := m.requestBytes(ctx, http.MethodGet, "/api/frame.jpeg?"+query.Encode(), nil, go2RTCMaxResponseBytes, &contentType)
	if err != nil {
		return config.Go2RTCStreamConfig{}, nil, false, err
	}
	if len(data) < 4 || data[0] != 0xff || data[1] != 0xd8 || data[len(data)-2] != 0xff || data[len(data)-1] != 0xd9 {
		return config.Go2RTCStreamConfig{}, nil, false, fmt.Errorf("go2rtc returned invalid JPEG data")
	}
	if mediaType, _, _ := strings.Cut(strings.ToLower(contentType), ";"); mediaType != "" && mediaType != "image/jpeg" {
		return config.Go2RTCStreamConfig{}, nil, false, fmt.Errorf("go2rtc returned an unexpected content type")
	}
	if opts.CacheSeconds > 0 {
		now := time.Now()
		m.mu.Lock()
		m.putSnapshotCacheLocked(cacheKey, data, now.Add(time.Duration(opts.CacheSeconds)*time.Second), now)
		m.mu.Unlock()
	}
	return stream, data, false, nil
}

func (m *Go2RTCManager) buildSnapshotResult(stream config.Go2RTCStreamConfig, data []byte, opts Go2RTCSnapshotOptions) (Go2RTCSnapshotResult, error) {
	result := snapshotMetadata(stream.ID, data)
	cfg := m.Config()
	if !opts.Store && !cfg.StoreMedia {
		return result, nil
	}
	if m.mediaDB != nil {
		var existingID int64
		var existingPath, existingWebPath string
		if err := m.mediaDB.QueryRow(
			"SELECT id, file_path, web_path FROM media_items WHERE hash = ? AND deleted = 0 LIMIT 1",
			result.SHA256,
		).Scan(&existingID, &existingPath, &existingWebPath); err == nil {
			result.Stored = true
			result.LocalPath = existingPath
			result.WebPath = existingWebPath
			result.MediaID = existingID
			return result, nil
		}
	}
	now := time.Now().UTC()
	relativeDir := filepath.Join("go2rtc", "snapshots", now.Format("2006"), now.Format("01"), now.Format("02"))
	destinationDir := filepath.Join(m.dataDir, relativeDir)
	if err := os.MkdirAll(destinationDir, 0o750); err != nil {
		return result, fmt.Errorf("create go2rtc snapshot directory: %w", err)
	}
	filename := fmt.Sprintf("%s_%s.jpg", stream.ID, now.Format("150405.000000000"))
	localPath := filepath.Join(destinationDir, filename)
	tmp, err := os.CreateTemp(destinationDir, ".snapshot-*.tmp")
	if err != nil {
		return result, fmt.Errorf("create temporary go2rtc snapshot: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		_ = tmp.Close()
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := io.Copy(tmp, bytes.NewReader(data)); err != nil {
		return result, fmt.Errorf("write temporary go2rtc snapshot: %w", err)
	}
	if err := tmp.Chmod(0o640); err != nil {
		return result, fmt.Errorf("secure go2rtc snapshot: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return result, fmt.Errorf("close temporary go2rtc snapshot: %w", err)
	}
	if err := os.Rename(tmpPath, localPath); err != nil {
		return result, fmt.Errorf("publish go2rtc snapshot: %w", err)
	}
	cleanup = false
	result.Stored = true
	result.LocalPath = localPath
	result.WebPath = "/files/" + filepath.ToSlash(filepath.Join(relativeDir, filename))
	if m.mediaDB != nil {
		mediaID, _, err := RegisterMedia(m.mediaDB, MediaItem{
			MediaType:   "image",
			SourceTool:  "go2rtc",
			Filename:    filename,
			FilePath:    localPath,
			WebPath:     result.WebPath,
			FileSize:    int64(len(data)),
			Format:      "jpg",
			Provider:    "go2rtc",
			Description: "go2rtc snapshot: " + stream.Name,
			Tags:        []string{"go2rtc", "snapshot", stream.ID},
			Hash:        result.SHA256,
		})
		if err != nil {
			_ = os.Remove(localPath)
			return Go2RTCSnapshotResult{}, fmt.Errorf("register go2rtc snapshot: %w", err)
		}
		result.MediaID = mediaID
	}
	if err := m.pruneStoredSnapshots(go2RTCMaxStoredItems, go2RTCMaxStoredBytes); err != nil && m.logger != nil {
		m.logger.Warn("[go2rtc] Failed to enforce snapshot retention", "error", err)
	}
	return result, nil
}

func snapshotMetadata(streamID string, data []byte) Go2RTCSnapshotResult {
	sum := sha256.Sum256(data)
	return Go2RTCSnapshotResult{
		Status:      "ok",
		StreamID:    streamID,
		ContentType: "image/jpeg",
		Bytes:       len(data),
		SHA256:      hex.EncodeToString(sum[:]),
	}
}

func (m *Go2RTCManager) putSnapshotCacheLocked(key string, data []byte, expiresAt, now time.Time) {
	if int64(len(data)) > go2RTCMaxCacheBytes {
		return
	}
	if previous, ok := m.cache[key]; ok {
		m.cacheBytes -= int64(len(previous.data))
		delete(m.cache, key)
	}
	m.pruneSnapshotCacheLocked(now, int64(len(data)))
	copyData := append([]byte(nil), data...)
	m.cache[key] = go2RTCSnapshotCacheEntry{data: copyData, expiresAt: expiresAt, lastUsed: now}
	m.cacheBytes += int64(len(copyData))
}

func (m *Go2RTCManager) pruneSnapshotCacheLocked(now time.Time, incomingBytes int64) {
	for key, entry := range m.cache {
		if !now.Before(entry.expiresAt) {
			m.cacheBytes -= int64(len(entry.data))
			delete(m.cache, key)
		}
	}
	for len(m.cache) > go2RTCMaxCacheEntries ||
		(incomingBytes > 0 && len(m.cache) >= go2RTCMaxCacheEntries) ||
		m.cacheBytes+incomingBytes > go2RTCMaxCacheBytes {
		oldestKey := ""
		var oldest time.Time
		for key, entry := range m.cache {
			if oldestKey == "" || entry.lastUsed.Before(oldest) {
				oldestKey = key
				oldest = entry.lastUsed
			}
		}
		if oldestKey == "" {
			break
		}
		m.cacheBytes -= int64(len(m.cache[oldestKey].data))
		delete(m.cache, oldestKey)
	}
	if m.cacheBytes < 0 {
		m.cacheBytes = 0
	}
}

type go2RTCStoredSnapshot struct {
	path    string
	size    int64
	modTime time.Time
}

func (m *Go2RTCManager) pruneStoredSnapshots(maxItems int, maxBytes int64) error {
	if maxItems <= 0 || maxBytes <= 0 {
		return fmt.Errorf("go2rtc snapshot retention limits must be positive")
	}
	root := filepath.Join(m.dataDir, "go2rtc", "snapshots")
	var items []go2RTCStoredSnapshot
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !strings.EqualFold(filepath.Ext(entry.Name()), ".jpg") {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			items = append(items, go2RTCStoredSnapshot{path: path, size: info.Size(), modTime: info.ModTime()})
		}
		return nil
	})
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("scan go2rtc snapshots: %w", err)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].modTime.Equal(items[j].modTime) {
			return items[i].path > items[j].path
		}
		return items[i].modTime.After(items[j].modTime)
	})
	var keptBytes int64
	for index, item := range items {
		if index < maxItems && keptBytes+item.size <= maxBytes {
			keptBytes += item.size
			continue
		}
		if m.mediaDB != nil {
			if _, err := m.mediaDB.Exec("UPDATE media_items SET deleted = 1, updated_at = CURRENT_TIMESTAMP WHERE source_tool = 'go2rtc' AND file_path = ? AND deleted = 0", item.path); err != nil {
				return fmt.Errorf("retire expired go2rtc media entry: %w", err)
			}
		}
		if err := os.Remove(item.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove expired go2rtc snapshot: %w", err)
		}
	}
	return nil
}

// ViewerPath returns AuraGo's stable viewer route without source URLs or credentials.
func (m *Go2RTCManager) ViewerPath(streamID string) (string, error) {
	if _, err := m.findEnabledStream(streamID); err != nil {
		return "", err
	}
	return "/api/go2rtc/viewer/" + url.PathEscape(strings.TrimSpace(streamID)), nil
}

// ProxyTarget returns the internal upstream URL for the reverse proxy.
func (m *Go2RTCManager) ProxyTarget() (*url.URL, error) {
	cfg := m.Config()
	m.mu.RLock()
	inDocker := m.inDocker
	m.mu.RUnlock()
	if err := ValidateGo2RTCInternalURL(cfg.URL, cfg.APIHostPort, inDocker); err != nil {
		return nil, err
	}
	return url.Parse(strings.TrimRight(cfg.URL, "/"))
}

// ProxyCredentials returns the internal credential for the server-side proxy only.
func (m *Go2RTCManager) ProxyCredentials() (string, string, error) {
	password, err := m.ensureAPIPassword()
	if err != nil {
		return "", "", err
	}
	return go2RTCAPIUser, password, nil
}

// EnabledStreamAlias maps a public stream ID to its internal runtime alias.
func (m *Go2RTCManager) EnabledStreamAlias(streamID string) (string, error) {
	stream, err := m.findEnabledStream(streamID)
	if err != nil {
		return "", err
	}
	return Go2RTCRuntimeStreamName(stream.ID), nil
}

func (m *Go2RTCManager) findEnabledStream(streamID string) (config.Go2RTCStreamConfig, error) {
	streamID = strings.TrimSpace(streamID)
	if err := ValidateGo2RTCStreamID(streamID); err != nil {
		return config.Go2RTCStreamConfig{}, err
	}
	for _, stream := range m.Config().Streams {
		if stream.ID == streamID && stream.Enabled {
			return stream, nil
		}
	}
	return config.Go2RTCStreamConfig{}, fmt.Errorf("stream %q is not configured or enabled", streamID)
}

// Go2RTCRuntimeStreamName returns a non-secret alias never derived from a URL.
func Go2RTCRuntimeStreamName(streamID string) string {
	return "aurago_" + strings.ToLower(strings.TrimSpace(streamID))
}

// ValidateGo2RTCStreamID enforces stable IDs suitable for vault keys, routes, and runtime aliases.
func ValidateGo2RTCStreamID(streamID string) error {
	streamID = strings.TrimSpace(streamID)
	if streamID == "" || streamID != strings.ToLower(streamID) || config.Go2RTCStreamSourceVaultKey(streamID) == "" {
		return fmt.Errorf("stream id %q must contain only lowercase letters, digits, hyphens, or underscores", streamID)
	}
	return nil
}

// ValidateGo2RTCSource enforces the v1 network-only source contract.
func ValidateGo2RTCSource(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed == nil || strings.TrimSpace(parsed.Host) == "" {
		return fmt.Errorf("a complete network URL is required")
	}
	switch strings.ToLower(parsed.Scheme) {
	case "rtsp", "rtsps", "rtspx", "http", "https", "onvif":
		return nil
	default:
		return fmt.Errorf("source scheme %q is not allowed", parsed.Scheme)
	}
}

// ValidateGo2RTCInternalURL prevents the managed client/proxy from becoming an SSRF primitive.
func ValidateGo2RTCInternalURL(raw string, apiHostPort int, inDocker bool) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed == nil || !strings.EqualFold(parsed.Scheme, "http") ||
		parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" ||
		(parsed.Path != "" && parsed.Path != "/") {
		return fmt.Errorf("go2rtc internal URL must be a plain HTTP sidecar URL without credentials, path, query, or fragment")
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	if inDocker {
		if host != "go2rtc" {
			return fmt.Errorf("go2rtc internal URL must use the managed Docker network alias go2rtc")
		}
	} else if host != "127.0.0.1" {
		return fmt.Errorf("go2rtc internal URL must use 127.0.0.1 for native AuraGo")
	}
	if apiHostPort < 1 || apiHostPort > 65535 || parsed.Port() != strconv.Itoa(apiHostPort) {
		return fmt.Errorf("go2rtc internal URL port must match api_host_port")
	}
	return nil
}

func (m *Go2RTCManager) ensureAPIPassword() (string, error) {
	m.credentialMu.Lock()
	defer m.credentialMu.Unlock()

	m.mu.RLock()
	current := strings.TrimSpace(m.cfg.APIPassword)
	m.mu.RUnlock()
	if current != "" {
		security.RegisterSensitive(current)
		return current, nil
	}
	if m.vault == nil {
		return "", fmt.Errorf("go2rtc API credential requires an initialized vault")
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate go2rtc API credential: %w", err)
	}
	current = hex.EncodeToString(buf)
	if err := m.vault.WriteSecret(config.Go2RTCAPIPasswordVaultKey, current); err != nil {
		return "", fmt.Errorf("store go2rtc API credential: %w", err)
	}
	security.RegisterSensitive(current)
	m.mu.Lock()
	m.cfg.APIPassword = current
	m.mu.Unlock()
	return current, nil
}

func (m *Go2RTCManager) requestJSON(ctx context.Context, method, path string, body io.Reader, target interface{}, maxBytes int64) error {
	data, err := m.requestBytes(ctx, method, path, body, maxBytes, nil)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("decode go2rtc response: %w", err)
	}
	return nil
}

func (m *Go2RTCManager) request(ctx context.Context, method, path string, body io.Reader, maxBytes int64, target interface{}) error {
	data, err := m.requestBytes(ctx, method, path, body, maxBytes, nil)
	if err != nil {
		return err
	}
	if target != nil && len(data) > 0 {
		if err := json.Unmarshal(data, target); err != nil {
			return fmt.Errorf("decode go2rtc response: %w", err)
		}
	}
	return nil
}

func (m *Go2RTCManager) requestBytes(ctx context.Context, method, path string, body io.Reader, maxBytes int64, contentType *string) ([]byte, error) {
	cfg := m.Config()
	if !cfg.Enabled {
		return nil, fmt.Errorf("go2rtc integration is disabled")
	}
	password, err := m.ensureAPIPassword()
	if err != nil {
		return nil, err
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.URL), "/")
	m.mu.RLock()
	inDocker := m.inDocker
	m.mu.RUnlock()
	if err := ValidateGo2RTCInternalURL(baseURL, cfg.APIHostPort, inDocker); err != nil {
		return nil, err
	}
	if maxBytes <= 0 || maxBytes > go2RTCMaxResponseBytes {
		maxBytes = go2RTCMaxResponseBytes
	}
	req, err := http.NewRequestWithContext(ctx, method, baseURL+"/api/go2rtc/proxy"+path, body)
	if err != nil {
		return nil, fmt.Errorf("create go2rtc request")
	}
	req.SetBasicAuth(go2RTCAPIUser, password)
	resp, err := m.client.Do(req)
	if err != nil {
		var requestErr *url.Error
		if errors.As(err, &requestErr) && requestErr.Err != nil {
			return nil, fmt.Errorf("go2rtc request failed: %w", requestErr.Err)
		}
		return nil, fmt.Errorf("go2rtc request failed")
	}
	defer resp.Body.Close()
	limited := io.LimitReader(resp.Body, maxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read go2rtc response: %w", err)
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("go2rtc response exceeds %d bytes", maxBytes)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("go2rtc returned HTTP %d", resp.StatusCode)
	}
	if contentType != nil {
		*contentType = resp.Header.Get("Content-Type")
	}
	return data, nil
}

func sliceLength(value interface{}) int {
	if list, ok := value.([]interface{}); ok {
		return len(list)
	}
	return 0
}

func collectGo2RTCCodecs(value interface{}) []string {
	found := make(map[string]struct{})
	var walk func(interface{})
	walk = func(current interface{}) {
		switch typed := current.(type) {
		case map[string]interface{}:
			for key, child := range typed {
				lowerKey := strings.ToLower(key)
				if lowerKey == "codec" || lowerKey == "codec_name" || lowerKey == "format_name" {
					if text, ok := child.(string); ok {
						text = sanitizeGo2RTCToken(text, 32)
						if text != "" {
							found[text] = struct{}{}
						}
					}
				}
				walk(child)
			}
		case []interface{}:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(value)
	result := make([]string, 0, len(found))
	for codec := range found {
		result = append(result, codec)
	}
	sort.Strings(result)
	return result
}

func sanitizeGo2RTCToken(value string, maxLength int) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > maxLength {
		return ""
	}
	for _, r := range value {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
			r == '.' || r == '_' || r == '-' || r == '+' || r == '/') {
			return ""
		}
	}
	return value
}

func firstString(object map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := object[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func go2RTCJSON(value interface{}) string {
	data, err := json.Marshal(value)
	if err != nil {
		return `{"status":"error","message":"failed to encode go2rtc result"}`
	}
	return string(data)
}
