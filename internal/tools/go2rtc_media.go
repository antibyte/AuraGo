package tools

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
)

type go2RTCMediaRuntime struct {
	config       config.Go2RTCStreamConfig
	key          string
	revision     uint64
	ctx          context.Context
	cancel       context.CancelFunc
	aliases      map[string]string
	registered   bool
	ready        bool
	lastSnapshot string
	lastError    string
}

// Go2RTCMediaLease owns a media operation until Release. Its context is also
// cancelled on a source change, disable, stop or shutdown of its camera.
type Go2RTCMediaLease struct {
	Context  context.Context
	StreamID string
	Role     string
	Alias    string
	Revision uint64
	release  func()
}

func (l *Go2RTCMediaLease) Release() {
	if l != nil && l.release != nil {
		l.release()
	}
}

func go2RTCMediaKey(s config.Go2RTCStreamConfig, hardware config.Go2RTCHardwareConfig) string {
	raw := strings.Join([]string{s.Source, s.SubSource, s.SnapshotSource, s.PreviewSource, s.DetectionSource, s.SnapshotMode, s.CodecProfile, s.Transport, s.PlaybackMode, fmt.Sprint(s.TalkEnabled), hardware.Backend}, "\x00")
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// syncMediaConfig runs after publishing the manager snapshot, outside mu.
// Never wait for network I/O while holding mediaMu.
func (m *Go2RTCManager) syncMediaConfig(cfg config.Go2RTCConfig) {
	m.mediaMu.Lock()
	defer m.mediaMu.Unlock()
	if m.media == nil {
		m.media = make(map[string]*go2RTCMediaRuntime)
		m.transcodes = make(map[string]int)
		m.hls = make(map[string]*go2RTCHLSSession)
		var nonce [8]byte
		_, _ = rand.Read(nonce[:])
		m.mediaEpoch = hex.EncodeToString(nonce[:])
	}
	wanted := map[string]bool{}
	for _, s := range cfg.Streams {
		if !cfg.Enabled || !s.Enabled {
			continue
		}
		wanted[s.ID] = true
		key := go2RTCMediaKey(s, cfg.Hardware)
		old := m.media[s.ID]
		if old != nil && old.key == key {
			old.config = s
			continue
		}
		revision := uint64(1)
		if old != nil {
			revision = old.revision + 1
			old.cancel()
			for _, a := range old.aliases {
				m.retiredAliases = append(m.retiredAliases, a)
			}
		}
		ctx, cancel := context.WithCancel(context.Background())
		m.media[s.ID] = &go2RTCMediaRuntime{config: s, key: key, revision: revision, ctx: ctx, cancel: cancel, ready: old == nil, aliases: map[string]string{"main": Go2RTCRuntimeStreamName(s.ID)}}
	}
	for id, r := range m.media {
		if !wanted[id] {
			r.cancel()
			for _, a := range r.aliases {
				m.retiredAliases = append(m.retiredAliases, a)
			}
			delete(m.media, id)
		}
	}
}

func (m *Go2RTCManager) cancelMedia() {
	m.mediaMu.Lock()
	defer m.mediaMu.Unlock()
	for _, r := range m.media {
		r.cancel()
		r.ready = false
	}
}

func mediaRole(s config.Go2RTCStreamConfig, purpose string) (string, error) {
	role := purpose
	switch purpose {
	case "", "main", "detail":
		role = "main"
	case "preview", "detection", "auto", "stream":
		role = s.PreviewSource
		if purpose == "detection" {
			role = s.DetectionSource
		}
		if role == "" || role == "auto" {
			role = "main"
			if s.SubSource != "" {
				role = "sub"
			}
		}
	default:
		if purpose != "sub" && purpose != "snapshot" {
			return "", fmt.Errorf("invalid camera source role")
		}
	}
	if role != "main" && config.Go2RTCRoleSource(s, role) == "" {
		return "", fmt.Errorf("camera source role is not configured")
	}
	return role, nil
}

func (m *Go2RTCManager) AcquireMedia(ctx context.Context, id, purpose string, compatible bool) (*Go2RTCMediaLease, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	m.mediaMu.Lock()
	defer m.mediaMu.Unlock()
	r := m.media[id]
	if r == nil || !r.ready || r.ctx.Err() != nil {
		return nil, fmt.Errorf("camera is disabled or its source update is pending")
	}
	role, err := mediaRole(r.config, purpose)
	if err != nil {
		return nil, err
	}
	key := role
	transcode := compatible && role != "snapshot" && r.config.CodecProfile != "" && r.config.CodecProfile != "original"
	if transcode {
		key += "/" + r.config.CodecProfile
	}
	alias := r.aliases[key]
	if alias == "" {
		return nil, fmt.Errorf("camera media profile is not ready")
	}
	if transcode {
		if m.transcodes[alias] == 0 && len(m.transcodes) >= 2 {
			return nil, fmt.Errorf("camera transcoding capacity reached (two profiles)")
		}
		m.transcodes[alias]++
	}
	owned, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(r.ctx, cancel)
	var once sync.Once
	release := func() {
		once.Do(func() {
			stop()
			cancel()
			if transcode {
				m.mediaMu.Lock()
				m.transcodes[alias]--
				if m.transcodes[alias] <= 0 {
					delete(m.transcodes, alias)
				}
				m.mediaMu.Unlock()
			}
		})
	}
	context.AfterFunc(owned, release)
	return &Go2RTCMediaLease{Context: owned, StreamID: id, Role: role, Alias: alias, Revision: r.revision, release: release}, nil
}

func (m *Go2RTCManager) StreamRevision(id string) uint64 {
	m.mediaMu.Lock()
	defer m.mediaMu.Unlock()
	if r := m.media[id]; r != nil {
		return r.revision
	}
	return 0
}
func (m *Go2RTCManager) MediaRevisionCurrent(id string, revision uint64) bool {
	m.mediaMu.Lock()
	defer m.mediaMu.Unlock()
	r := m.media[id]
	return r != nil && r.ready && r.ctx.Err() == nil && r.revision == revision
}

func (m *Go2RTCManager) mediaAliases(id string) map[string]string {
	m.mediaMu.Lock()
	defer m.mediaMu.Unlock()
	out := map[string]string{}
	if r := m.media[id]; r != nil {
		for k, v := range r.aliases {
			out[k] = v
		}
	}
	return out
}

func go2RTCTransportSource(source, transport string) string {
	if transport == "" || transport == "inherit" {
		return source
	}
	u, err := url.Parse(source)
	if err != nil || (u.Scheme != "rtsp" && u.Scheme != "rtsps" && u.Scheme != "rtspx" && u.Scheme != "onvif") {
		return source
	}
	parts := []string{}
	for _, p := range strings.Split(u.Fragment, "#") {
		if p != "" && !strings.HasPrefix(p, "transport=") {
			parts = append(parts, p)
		}
	}
	parts = append(parts, "transport="+transport)
	u.Fragment = strings.Join(parts, "#")
	return u.String()
}

func (m *Go2RTCManager) reconcileMediaStreams(ctx context.Context) (int, error) {
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
		m.mediaMu.Lock()
		r := m.media[stream.ID]
		if r == nil {
			m.mediaMu.Unlock()
			continue
		}
		if r.ctx.Err() != nil {
			r.ctx, r.cancel = context.WithCancel(context.Background())
			r.revision++
			r.registered = false
		}
		if !r.registered && r.ready {
			r.cancel()
			r.ctx, r.cancel = context.WithCancel(context.Background())
			r.revision++
			r.ready = false
			for _, alias := range r.aliases {
				m.retiredAliases = append(m.retiredAliases, alias)
			}
		}

		revision := r.revision
		prefix := fmt.Sprintf("aurago~%s~%x~%d~", m.mediaEpoch, sha256.Sum256([]byte(stream.ID)), revision)
		aliases := map[string]string{}
		for _, role := range []string{"main", "sub", "snapshot"} {
			if config.Go2RTCRoleSource(stream, role) != "" {
				aliases[role] = prefix + role
				if role != "snapshot" && stream.CodecProfile != "" && stream.CodecProfile != "original" {
					aliases[role+"/"+stream.CodecProfile] = prefix + role + "~" + stream.CodecProfile
				}
			}
		}
		if r.registered {
			aliases = map[string]string{}
			for role, alias := range r.aliases {
				aliases[role] = alias
			}
		}
		m.mediaMu.Unlock()
		for _, role := range []string{"main", "sub", "snapshot"} {
			source := config.Go2RTCRoleSource(stream, role)
			if source == "" {
				if role == "main" {
					return count, fmt.Errorf("camera %q needs its main source", stream.ID)
				}
				continue
			}
			if ValidateGo2RTCSource(source) != nil {
				return count, fmt.Errorf("invalid camera %s source", role)
			}
			if err := m.patchMediaSource(ctx, aliases[role], go2RTCTransportSource(source, stream.Transport)); err != nil {
				return count, err
			}
			if derived := aliases[role+"/"+stream.CodecProfile]; derived != "" {
				audio := "opus"
				if stream.CodecProfile == "h264_aac" {
					audio = "aac"
				}
				input := "ffmpeg:" + aliases[role] + "#video=h264#audio=" + audio
				if cfg.Hardware.Backend != "" && cfg.Hardware.Backend != "software" {
					input += "#hardware=" + cfg.Hardware.Backend
				}
				if err := m.patchMediaSource(ctx, derived, input); err != nil {
					return count, err
				}
			}
		}
		m.mediaMu.Lock()
		if current := m.media[stream.ID]; current == r && current.revision == revision {
			r.aliases = aliases
			r.registered = true
			r.ready = true
		}
		m.mediaMu.Unlock()
		if err := m.reconcilePreload(ctx, stream); err != nil {
			return count, err
		}
		count++
	}
	m.mediaMu.Lock()
	retired := m.retiredAliases
	m.retiredAliases = nil
	m.mediaMu.Unlock()
	for _, alias := range retired {
		// Runtime deletion can patch only the non-secret tmpfs config.
		_ = m.request(ctx, http.MethodDelete, "/api/preload?"+url.Values{"src": {alias}}.Encode(), nil, 1<<20, nil)
		if err := m.request(ctx, http.MethodDelete, "/api/streams?"+url.Values{"src": {alias}}.Encode(), nil, 1<<20, nil); err != nil {
			m.mediaMu.Lock()
			m.retiredAliases = append(m.retiredAliases, alias)
			m.mediaMu.Unlock()
		}
	}
	return count, nil
}

func (m *Go2RTCManager) patchMediaSource(ctx context.Context, alias, source string) error {
	return m.request(ctx, http.MethodPatch, "/api/streams?"+url.Values{"name": {alias}, "src": {source}}.Encode(), nil, 1<<20, nil)
}

func (m *Go2RTCManager) reconcilePreload(ctx context.Context, stream config.Go2RTCStreamConfig) error {
	m.mediaMu.Lock()
	alias := ""
	if r := m.media[stream.ID]; r != nil {
		alias = r.aliases["main"]
	}
	previous := m.preloaded[stream.ID]
	m.mediaMu.Unlock()
	if stream.Preload {
		var live map[string]interface{}
		if err := m.requestJSON(ctx, http.MethodGet, "/api/preload", nil, &live, 1<<20); err == nil {
			if _, ok := live[alias]; ok {
				return nil
			}
		}
	}

	if previous != "" {
		_ = m.request(ctx, http.MethodDelete, "/api/preload?"+url.Values{"src": {previous}}.Encode(), nil, 1<<20, nil)
	}
	if stream.Preload {
		if err := m.request(ctx, http.MethodPut, "/api/preload?"+url.Values{"src": {alias}, "video": {}}.Encode(), nil, 1<<20, nil); err != nil {
			return fmt.Errorf("camera preload unavailable: %w", err)
		}
	}
	m.mediaMu.Lock()
	if m.preloaded == nil {
		m.preloaded = map[string]string{}
	}
	if stream.Preload {
		m.preloaded[stream.ID] = alias
	} else {
		delete(m.preloaded, stream.ID)
	}
	m.mediaMu.Unlock()
	return nil
}

type go2RTCHLSSession struct {
	lease *Go2RTCMediaLease
	timer *time.Timer
}

func (m *Go2RTCManager) RememberHLS(id, purpose, sid string, revision uint64) error {
	lease, err := m.AcquireMedia(context.Background(), id, purpose, true)
	if err == nil && lease.Revision != revision {
		lease.Release()
		return fmt.Errorf("camera HLS revision expired")
	}
	if err != nil {
		return err
	}
	m.mediaMu.Lock()
	if previous := m.hls[sid]; previous != nil {
		previous.timer.Stop()
		m.mediaMu.Unlock()
		lease.Release()
		return nil
	}
	entry := &go2RTCHLSSession{lease: lease}
	m.hls[sid] = entry
	entry.timer = time.AfterFunc(5*time.Second, func() { m.expireHLS(sid, entry) })
	m.mediaMu.Unlock()
	context.AfterFunc(lease.Context, func() { m.expireHLS(sid, entry) })
	return nil
}
func (m *Go2RTCManager) expireHLS(sid string, entry *go2RTCHLSSession) {
	m.mediaMu.Lock()
	if m.hls[sid] == entry {
		delete(m.hls, sid)
		entry.timer.Stop()
	}
	m.mediaMu.Unlock()
	entry.lease.Release()
}
func (m *Go2RTCManager) HLSLease(ctx context.Context, sid string) (*Go2RTCMediaLease, error) {
	m.mediaMu.Lock()
	entry := m.hls[sid]
	if entry == nil || entry.lease.Context.Err() != nil {
		m.mediaMu.Unlock()
		return nil, fmt.Errorf("camera HLS session expired")
	}
	entry.timer.Reset(5 * time.Second)
	id, role := entry.lease.StreamID, entry.lease.Role
	m.mediaMu.Unlock()
	return m.AcquireMedia(ctx, id, role, true)
}

type Go2RTCMediaSettings struct {
	PreviewSource   string `json:"preview_source"`
	DetectionSource string `json:"detection_source"`
	SnapshotMode    string `json:"snapshot_mode"`
	CodecProfile    string `json:"codec_profile"`
	Transport       string `json:"transport"`
	PlaybackMode    string `json:"playback_mode"`
	Preload         bool   `json:"preload"`
	TalkEnabled     bool   `json:"talk_enabled"`
}

func (m *Go2RTCManager) applyMediaStatus(out *Go2RTCStreamStatus, s config.Go2RTCStreamConfig) {
	out.HasSubstream = s.SubSource != ""
	out.HasCameraSnapshot = s.SnapshotSource != ""
	out.VideoCodecs = []string{}
	out.AudioCodecs = []string{}
	out.State = "idle"
	out.Media = Go2RTCMediaSettings{s.PreviewSource, s.DetectionSource, s.SnapshotMode, s.CodecProfile, s.Transport, s.PlaybackMode, s.Preload, s.TalkEnabled}
	m.mediaMu.Lock()
	defer m.mediaMu.Unlock()
	if r := m.media[s.ID]; r != nil {
		out.Revision = r.revision
		out.LastSnapshotAt = r.lastSnapshot
		out.LastError = r.lastError
		if !r.ready {
			out.State = "connecting"
		} else if r.lastSnapshot != "" {
			out.State = "online"
		}
		if r.lastError != "" {
			out.State = "error"
		}
	}
}

func (m *Go2RTCManager) recordCameraSnapshot(id string, revision uint64, ok, fallback bool) {
	m.mediaMu.Lock()
	defer m.mediaMu.Unlock()
	r := m.media[id]
	if r == nil || r.revision != revision || r.ctx.Err() != nil {
		return
	}
	if ok {
		r.lastSnapshot = time.Now().UTC().Format(time.RFC3339Nano)
		r.lastError = ""
		if fallback {
			r.lastError = "camera_snapshot_fallback"
		}
	} else {
		r.lastError = "snapshot_unavailable"
	}
}

func go2RTCViewerCount(value interface{}) int {
	items, _ := value.([]interface{})
	count := 0
	for _, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		format := strings.ToLower(firstString(m, "format_name", "type"))
		agent := strings.ToLower(firstString(m, "user_agent"))
		if format == "probe" || format == "preload" || strings.Contains(agent, "aurago camera snapshot") {
			continue
		}
		count++
	}
	return count
}

func readGo2RTCTelemetry(value interface{}, out *Go2RTCStreamStatus) {
	switch x := value.(type) {
	case map[string]interface{}:
		if n, ok := x["bytes_recv"].(float64); ok && n >= 0 {
			out.ReceivedBytes += uint64(n)
		}
		if n, ok := x["bytes_send"].(float64); ok && n >= 0 {
			out.SentBytes += uint64(n)
		}
		if kind, _ := x["kind"].(string); kind == "video" || kind == "audio" {
			for _, codec := range collectGo2RTCCodecs(x) {
				if kind == "video" {
					out.VideoCodecs = append(out.VideoCodecs, codec)
				} else {
					out.AudioCodecs = append(out.AudioCodecs, codec)
				}
			}
		}
		for _, v := range x {
			readGo2RTCTelemetry(v, out)
		}
	case []interface{}:
		for _, v := range x {
			readGo2RTCTelemetry(v, out)
		}
	}
}
