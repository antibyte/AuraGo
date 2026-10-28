package tools

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
)

type go2RTCMediaRuntime struct {
	config       config.Go2RTCStreamConfig
	key          string
	desiredKey   string
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
	_, hardwareKey := m.hardwareCache(cfg)
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
	if hardwareKey != m.hardwareKey {
		m.hardwareKey = hardwareKey
		m.hardwareGeneration++
		m.hardwareVerified = false
		m.hardwareVerifiedAt = ""
	}
	m.hardwareBackend = cfg.Hardware.Backend
	wanted := map[string]bool{}
	for _, s := range cfg.Streams {
		if !cfg.Enabled || !s.Enabled {
			continue
		}
		wanted[s.ID] = true
		key := go2RTCMediaKey(s, cfg.Hardware)
		old := m.media[s.ID]
		if old != nil {
			old.desiredKey = key
			if old.key == key {
				old.config = s
			}
			continue
		}
		ctx, cancel := context.WithCancel(context.Background())
		m.mediaRevision++
		m.media[s.ID] = &go2RTCMediaRuntime{config: s, key: key, desiredKey: key, revision: m.mediaRevision, ctx: ctx, cancel: cancel, ready: true, aliases: map[string]string{"main": Go2RTCRuntimeStreamName(s.ID)}}
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
	if m.mediaClosed || r == nil || !r.ready || r.ctx.Err() != nil {
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
		if m.hardwareBackend != "" && m.hardwareBackend != "software" && !m.hardwareVerified {
			return nil, fmt.Errorf("selected camera hardware must be verified first")
		}
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
	u.Fragment, u.RawFragment = "", ""
	return u.String() + "#" + strings.Join(parts, "#")
}

func (m *Go2RTCManager) reconcileMediaStreams(ctx context.Context) (count int, err error) {
	m.mediaMu.Lock()
	closed := m.mediaClosed
	m.mediaMu.Unlock()
	if closed {
		return 0, fmt.Errorf("camera manager is closed")
	}
	cfg := m.Config()
	if !cfg.Enabled {
		return 0, fmt.Errorf("go2rtc integration is disabled")
	}
	defer func() { err = errors.Join(err, m.retireMediaAliases(ctx)) }()
	// Aliases are immutable for their revision. Avoid touching live producers
	// on periodic reconciliation; a missing alias after a sidecar restart is
	// registered again. Failure to read the inventory keeps the safe old retry.
	known := map[string]json.RawMessage{}
	_ = m.requestJSON(ctx, http.MethodGet, "/api/streams", nil, &known, 4<<20)
	m.mediaMu.Lock()
	for alias := range known {
		if strings.HasPrefix(alias, "aurago~") && !strings.HasPrefix(alias, "aurago~"+m.mediaEpoch+"~") {
			m.retiredAliases = append(m.retiredAliases, alias)
		}
	}
	m.mediaMu.Unlock()
	for _, stream := range cfg.Streams {
		if !stream.Enabled {
			continue
		}
		if err := ValidateGo2RTCStreamID(stream.ID); err != nil {
			return count, err
		}
		m.mediaMu.Lock()
		active := m.media[stream.ID]
		targetKey := go2RTCMediaKey(stream, cfg.Hardware)
		if active == nil || active.desiredKey != targetKey {
			m.mediaMu.Unlock()
			continue
		}
		r := active
		if active.key != targetKey || !active.registered || active.ctx.Err() != nil {
			// Keep the active revision usable until every replacement alias exists.
			owned, cancel := context.WithCancel(context.Background())
			// Allocate from the manager lifetime, including failed candidates and
			// removed cameras, so an old lease can never become current again.
			m.mediaRevision++
			r = &go2RTCMediaRuntime{config: stream, key: targetKey, desiredKey: targetKey, revision: m.mediaRevision, ctx: owned, cancel: cancel, aliases: map[string]string{}, lastSnapshot: active.lastSnapshot}
			prefix := fmt.Sprintf("aurago~%s~%x~%d~", m.mediaEpoch, sha256.Sum256([]byte(stream.ID)), r.revision)
			for _, role := range []string{"main", "sub", "snapshot"} {
				if config.Go2RTCRoleSource(stream, role) != "" {
					r.aliases[role] = prefix + role
					if role != "snapshot" && stream.CodecProfile != "" && stream.CodecProfile != "original" {
						r.aliases[role+"/"+stream.CodecProfile] = prefix + role + "~" + stream.CodecProfile
					}
				}
			}
			// Also clean partial registrations on an error or superseding config.
			for _, alias := range r.aliases {
				m.retiredAliases = append(m.retiredAliases, alias)
			}
			defer func(candidate *go2RTCMediaRuntime) {
				m.mediaMu.Lock()
				defer m.mediaMu.Unlock()
				if m.media[candidate.config.ID] != candidate {
					candidate.cancel()
				}
			}(r)
		}
		registered := r.registered
		aliases := r.aliases
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
			if !registered || len(known[aliases[role]]) == 0 {
				if err := m.patchMediaSource(ctx, aliases[role], go2RTCTransportSource(source, stream.Transport)); err != nil {
					return count, err
				}
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
				if !registered || len(known[derived]) == 0 {
					if err := m.patchMediaSource(ctx, derived, input); err != nil {
						return count, err
					}
				}
			}
		}
		m.mediaMu.Lock()
		published := !m.mediaClosed && m.media[stream.ID] == active && active.desiredKey == targetKey
		if published {
			if r != active {
				active.cancel()
				for _, alias := range active.aliases {
					m.retiredAliases = append(m.retiredAliases, alias)
				}
				m.media[stream.ID] = r
			}
			r.registered = true
			r.ready = true
		}
		m.mediaMu.Unlock()
		if !published {
			continue
		}
		if err := m.reconcilePreload(ctx, stream); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func (m *Go2RTCManager) retireMediaAliases(ctx context.Context) error {
	m.mediaMu.Lock()
	retired := map[string]bool{}
	for _, alias := range m.retiredAliases {
		retired[alias] = true
	}
	for _, r := range m.media {
		for _, alias := range r.aliases {
			delete(retired, alias)
		}
	}
	m.retiredAliases = nil
	m.mediaMu.Unlock()
	var failure error
	for alias := range retired {
		// Runtime deletion can patch only the non-secret tmpfs config.
		err := m.removePreload(ctx, alias)
		if err == nil {
			err = m.request(ctx, http.MethodDelete, "/api/streams?"+url.Values{"src": {alias}}.Encode(), nil, 1<<20, nil)
		}
		if err != nil {
			failure = errors.Join(failure, err)
			m.mediaMu.Lock()
			m.retiredAliases = append(m.retiredAliases, alias)
			m.mediaMu.Unlock()
		}
	}
	return failure
}

func (m *Go2RTCManager) removePreload(ctx context.Context, alias string) error {
	err := m.request(ctx, http.MethodDelete, "/api/preload?"+url.Values{"src": {alias}}.Encode(), nil, 1<<20, nil)
	if err == nil {
		return nil
	}
	// Upstream returns 500 for an already absent preload. Confirm absence
	// through inventory; an uncertain failed delete must remain retryable.
	var live map[string]json.RawMessage
	if m.requestJSON(ctx, http.MethodGet, "/api/preload", nil, &live, 1<<20) == nil && live != nil {
		if _, exists := live[alias]; !exists {
			return nil
		}
	}
	return fmt.Errorf("camera preload cleanup unavailable: %w", err)
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
	warm := false
	if stream.Preload {
		var live map[string]interface{}
		if err := m.requestJSON(ctx, http.MethodGet, "/api/preload", nil, &live, 1<<20); err == nil {
			_, warm = live[alias]
		}
	}

	if previous != "" && (previous != alias || !stream.Preload) {
		if err := m.removePreload(ctx, previous); err != nil {
			return err
		}
	}
	if stream.Preload && !warm {
		if err := m.request(ctx, http.MethodPut, "/api/preload?"+url.Values{"src": {alias}, "video": {}}.Encode(), nil, 1<<20, nil); err != nil {
			return fmt.Errorf("camera preload unavailable: %w", err)
		}
	}
	m.mediaMu.Lock()
	if m.mediaClosed {
		m.mediaMu.Unlock()
		if stream.Preload {
			cleanup, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_ = m.request(cleanup, http.MethodDelete, "/api/preload?"+url.Values{"src": {alias}}.Encode(), nil, 1<<20, nil)
		}
		return fmt.Errorf("camera manager closed during preload update")
	}
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

func (m *Go2RTCManager) clearPreloads(ctx context.Context) error {
	m.mediaMu.Lock()
	aliases := make(map[string]string, len(m.preloaded))
	for id, alias := range m.preloaded {
		aliases[id] = alias
	}
	m.mediaMu.Unlock()
	var failure error
	for id, alias := range aliases {
		if err := m.removePreload(ctx, alias); err != nil {
			failure = errors.Join(failure, err)
			continue
		}
		m.mediaMu.Lock()
		if m.preloaded[id] == alias {
			delete(m.preloaded, id)
		}
		m.mediaMu.Unlock()
	}
	return failure
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
		same := previous.lease.StreamID == lease.StreamID && previous.lease.Role == lease.Role && previous.lease.Revision == lease.Revision && previous.lease.Context.Err() == nil
		if same {
			previous.timer.Reset(5 * time.Second)
		}
		m.mediaMu.Unlock()
		lease.Release()
		if !same {
			return fmt.Errorf("camera HLS session belongs to a different source revision")
		}
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
	id, role, revision := entry.lease.StreamID, entry.lease.Role, entry.lease.Revision
	m.mediaMu.Unlock()
	lease, err := m.AcquireMedia(ctx, id, role, true)
	if err == nil && lease.Revision != revision {
		lease.Release()
		return nil, fmt.Errorf("camera HLS revision expired")
	}
	return lease, err
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
	out.SourceType = config.Go2RTCStreamSourceType(s)
	m.mediaMu.Lock()
	defer m.mediaMu.Unlock()
	if r := m.media[s.ID]; r != nil {
		s = r.config
		out.Revision = r.revision
		out.MediaReady = !m.mediaClosed && r.ready && r.ctx.Err() == nil
		out.SourcePending = r.key != r.desiredKey || !r.registered
		out.LastSnapshotAt = r.lastSnapshot
		out.LastError = r.lastError
	}
	out.HasSubstream = s.SubSource != ""
	out.HasCameraSnapshot = s.SnapshotSource != ""
	out.VideoCodecs = []string{}
	out.AudioCodecs = []string{}
	out.SourceRoles = []string{"main"}
	if s.SubSource != "" {
		out.SourceRoles = append(out.SourceRoles, "sub")
	}
	if s.SnapshotSource != "" {
		out.SourceRoles = append(out.SourceRoles, "snapshot")
	}
	out.State = "idle"
	out.Media = Go2RTCMediaSettingsFor(s)
	if r := m.media[s.ID]; r != nil {
		if !out.MediaReady {
			out.State = "connecting"
		}
		if r.lastError != "" && r.lastError != "camera_snapshot_fallback" {
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
		if format == "probe" || format == "preload" || strings.Contains(agent, "aurago camera snapshot") || strings.Contains(agent, "go2rtc/ffmpeg") || strings.Contains(agent, "aurago camera microphone") || format == "rtsp" && strings.Contains(agent, "go2rtc") {
			continue
		}
		count++
	}
	return count
}

type Go2RTCMediaCounters struct {
	ReceivedBytes   uint64 `json:"received_bytes"`
	SentBytes       uint64 `json:"sent_bytes"`
	ReceivedPackets uint64 `json:"received_packets"`
	SentPackets     uint64 `json:"sent_packets"`
}

func uniqueGo2RTCStrings(values []string) []string {
	found := map[string]bool{}
	result := []string{}
	for _, value := range values {
		if !found[value] {
			found[value] = true
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}
func readGo2RTCTelemetry(value interface{}, out *Go2RTCStreamStatus) {
	number := func(value interface{}) uint64 {
		n, _ := value.(float64)
		if n >= 0 && n < 1<<53 {
			return uint64(n)
		}
		return 0
	}
	addCodec := func(kind, codec string) {
		codec = sanitizeGo2RTCToken(codec, 32)
		if codec == "" {
			return
		}
		if kind == "video" {
			out.VideoCodecs = append(out.VideoCodecs, codec)
		}
		if kind == "audio" {
			out.AudioCodecs = append(out.AudioCodecs, codec)
		}
	}
	switch x := value.(type) {
	case map[string]interface{}:
		for _, key := range []string{"codec", "codec_name"} {
			if name, ok := x[key].(string); ok {
				switch strings.ToUpper(name) {
				case "H264", "H265", "JPEG", "MJPEG", "VP8", "VP9", "AV1":
					addCodec("video", name)
				case "AAC", "OPUS", "PCMA", "PCMU", "PCM", "PCML", "MP3", "G722", "FLAC":
					addCodec("audio", name)
				}
			}
		}
		out.ReceivedBytes += number(x["bytes_recv"])
		out.SentBytes += number(x["bytes_send"])
		if kind, _ := x["kind"].(string); kind == "video" || kind == "audio" {
			for _, codec := range collectGo2RTCCodecs(x) {
				addCodec(kind, codec)
			}
		}
		for key, child := range x {
			if key == "medias" {
				if items, ok := child.([]interface{}); ok {
					for _, item := range items {
						media, _ := item.(string)
						parts := strings.Split(media, ",")
						if len(parts) < 3 {
							continue
						}
						for _, part := range parts[2:] {
							addCodec(strings.TrimSpace(parts[0]), strings.TrimSpace(part))
						}
					}
				}
			}
			if key == "receivers" || key == "senders" {
				if items, ok := child.([]interface{}); ok {
					for _, item := range items {
						track, _ := item.(map[string]interface{})
						codec, _ := track["codec"].(map[string]interface{})
						name := strings.ToUpper(firstString(codec, "codec_name", "name"))
						counters := &out.Audio
						switch name {
						case "H264", "H265", "JPEG", "MJPEG", "VP8", "VP9", "AV1", "RAW":
							counters = &out.Video
						}
						if name == "" {
							continue
						}
						if key == "receivers" {
							counters.ReceivedBytes += number(track["bytes"])
							counters.ReceivedPackets += number(track["packets"])
						} else {
							counters.SentBytes += number(track["bytes"])
							counters.SentPackets += number(track["packets"])
						}
					}
				}
			}
			readGo2RTCTelemetry(child, out)
		}
	case []interface{}:
		for _, child := range x {
			readGo2RTCTelemetry(child, out)
		}
	}
}

func Go2RTCMediaSettingsFor(s config.Go2RTCStreamConfig) Go2RTCMediaSettings {
	return Go2RTCMediaSettings{s.PreviewSource, s.DetectionSource, s.SnapshotMode, s.CodecProfile, s.Transport, s.PlaybackMode, s.Preload, s.TalkEnabled}
}

// ConfiguredStreamStatuses keeps revisions and last successful image metadata
// available while the sidecar/API is stopped, without opening a camera source.
func (m *Go2RTCManager) ConfiguredStreamStatuses() []Go2RTCStreamStatus {
	result := []Go2RTCStreamStatus{}
	for _, stream := range m.Config().Streams {
		if !stream.Enabled {
			continue
		}
		entry := Go2RTCStreamStatus{ID: stream.ID, Name: stream.Name, Codecs: []string{}}
		m.applyMediaStatus(&entry, stream)
		result = append(result, entry)
	}
	return result
}
