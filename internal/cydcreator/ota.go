package cydcreator

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

// Device endpoint rules (spec §9, §16). Every limit is per device except the
// failed-authentication limit, which is per client address.
const (
	// DeviceFirmwarePath is the download path the heartbeat answer points to.
	// The offer carries it as a path below the device URL (the firmware rejects
	// anything else, template cyd_link.cpp firmware_path), never as an absolute URL.
	DeviceFirmwarePath         = "/api/cyd-creator/device/firmware/"
	firmwareDownloadsPerWindow = 3
	firmwareDownloadWindow     = 10 * time.Minute
	heartbeatsPerMinute        = 12
	otaResultsPerWindow        = 10
	authFailuresPerWindow      = 20
	authFailureWindow          = 10 * time.Minute
	maxOTAErrorRunes           = 200
	// longestDeviceWindow is the longest window above; deviceLimiterSweepKeys
	// is the key count from which the limiter drops idle keys (at most once a
	// minute), so keys of removed devices cannot pile up.
	longestDeviceWindow    = 10 * time.Minute
	deviceLimiterSweepKeys = 1024
	// maxAuthClients caps the client keys of the failed-authentication limit;
	// while it is reached (after a sweep), further clients share the bucket
	// authOverflowKey, so rotating addresses cannot grow memory.
	maxAuthClients  = 4096
	authOverflowKey = "*"
	// rolledBackPrefix starts the ota_error of a boot after a rollback
	// (template cyd_link.cpp link_begin: "rolled back from <build id>").
	rolledBackPrefix = "rolled back from "
)

var (
	deviceVersionPattern = regexp.MustCompile(`^[0-9A-Za-z.+-]{0,32}$`)
	deviceReasonPattern  = regexp.MustCompile(`^[A-Za-z0-9_ -]{0,32}$`)
)

// deviceLimiter counts events per key in a sliding window (per-device limits)
// and the distinct failing tokens per client key (authFailed, authBlocked).
// The zero value is ready; keys without events in their window are dropped on
// the next check and, once there are many keys, by a sweep.
type deviceLimiter struct {
	mu    sync.Mutex
	hits  map[string][]time.Time
	swept time.Time

	auth      map[string]map[[sha256.Size]byte]time.Time // client key → failing token hash → last failure
	authSwept time.Time
}

// clientKey is the failed-authentication key of a client address: an IPv4
// address as is, an IPv6 address as its /64 (one subscriber or LAN usually
// owns a whole /64, so rotating the interface id gains nothing), anything
// else clipped.
func clientKey(addr string) string {
	addr = strings.TrimSpace(addr)
	ip, err := netip.ParseAddr(addr)
	if err != nil {
		return clipEcho(addr)
	}
	ip = ip.Unmap().WithZone("")
	if ip.Is4() {
		return ip.String()
	}
	prefix, err := ip.Prefix(64)
	if err != nil {
		return clipEcho(addr)
	}
	return prefix.String()
}

// authTokens returns the failures of one client key that are still within the
// window, dropping the key when none is left. Callers hold mu.
func (l *deviceLimiter) authTokens(key string, now time.Time) map[[sha256.Size]byte]time.Time {
	tokens, ok := l.auth[key]
	if !ok {
		return nil
	}
	for hash, at := range tokens {
		if now.Sub(at) >= authFailureWindow {
			delete(tokens, hash)
		}
	}
	if len(tokens) == 0 {
		delete(l.auth, key)
		return nil
	}
	return tokens
}

// authKey maps a client to its own key, or to authOverflowKey while the map
// holds maxAuthClients other keys (idle keys are swept at most once a minute
// first). Callers hold mu.
func (l *deviceLimiter) authKey(client string, now time.Time) string {
	key := clientKey(client)
	if _, ok := l.auth[key]; ok || len(l.auth) < maxAuthClients {
		return key
	}
	if now.Sub(l.authSwept) >= time.Minute {
		l.authSwept = now
		for k := range l.auth {
			l.authTokens(k, now)
		}
		if len(l.auth) < maxAuthClients {
			return key
		}
	}
	return authOverflowKey
}

// authBlocked reports whether the client failed with authFailuresPerWindow
// distinct tokens within the window.
func (l *deviceLimiter) authBlocked(client string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.authTokens(l.authKey(client, now), now)) >= authFailuresPerWindow
}

// authFailed records a failed authentication. The same token counts once per
// window: a revoked or orphaned device that repeats its stale token every
// heartbeat cannot lock out the devices sharing its address, while guessing
// needs a new token per try and stays capped. A blocked client records
// nothing (authBlocked runs first), so a key holds about authFailuresPerWindow
// tokens at most.
func (l *deviceLimiter) authFailed(client, token string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.auth == nil {
		l.auth = map[string]map[[sha256.Size]byte]time.Time{}
	}
	key := l.authKey(client, now)
	tokens := l.authTokens(key, now)
	if tokens == nil {
		tokens = map[[sha256.Size]byte]time.Time{}
		l.auth[key] = tokens
	}
	tokens[sha256.Sum256([]byte(token))] = now
}

func (l *deviceLimiter) recent(key string, now time.Time, window time.Duration) []time.Time {
	kept := l.hits[key][:0]
	for _, t := range l.hits[key] {
		if now.Sub(t) < window {
			kept = append(kept, t)
		}
	}
	if len(kept) == 0 {
		delete(l.hits, key)
		return nil
	}
	l.hits[key] = kept
	return kept
}

// prepare creates the map and, from deviceLimiterSweepKeys keys on and at most
// once a minute, drops every key whose newest event left the longest window.
func (l *deviceLimiter) prepare(now time.Time) {
	if l.hits == nil {
		l.hits = map[string][]time.Time{}
	}
	if len(l.hits) < deviceLimiterSweepKeys || now.Sub(l.swept) < time.Minute {
		return
	}
	l.swept = now
	for key, times := range l.hits {
		if now.Sub(times[len(times)-1]) >= longestDeviceWindow {
			delete(l.hits, key)
		}
	}
}

// allow records an event and reports whether it is within limit.
func (l *deviceLimiter) allow(key string, now time.Time, limit int, window time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.prepare(now)
	if len(l.recent(key, now, window)) >= limit {
		return false
	}
	l.hits[key] = append(l.hits[key], now)
	return true
}

// DeviceRequest is one call of a device endpoint: the bearer token and the
// client address, the key of the failed-authentication limit (IPv6 by /64).
// The HTTP layer passes the address its trusted-proxy rule resolves (server
// ClientIP), never the raw peer address: behind a reverse proxy every device
// would otherwise share one bucket, and one attacker could lock all of them out.
type DeviceRequest struct {
	Token    string
	ClientIP string
}

// Heartbeat is the body of POST /api/cyd-creator/device/heartbeat.
type Heartbeat struct {
	DeviceID        string `json:"device_id"`
	BuildID         string `json:"build_id"`
	TemplateVersion string `json:"template_version"`
	UptimeS         int64  `json:"uptime_s"`
	IP              string `json:"ip"`
	RSSI            int    `json:"rssi"`
	FreeHeap        int64  `json:"free_heap"`
	ResetReason     string `json:"reset_reason"`
	OTAState        string `json:"ota_state"`
	OTAError        string `json:"ota_error"`
}

// HeartbeatResponse tells the device when to report again and which update to
// install (nil: none).
type HeartbeatResponse struct {
	IntervalS int             `json:"interval_s"`
	Update    *FirmwareUpdate `json:"update"`
}

// FirmwareUpdate is the OTA offer: firmware.bin of BuildID with its size and
// SHA-256, downloadable at URL, a path below the device URL
// (DeviceFirmwarePath + id); the device prefixes the URL from its config blob.
type FirmwareUpdate struct {
	BuildID string `json:"build_id"`
	Size    int64  `json:"size"`
	SHA256  string `json:"sha256"`
	URL     string `json:"url"`
}

// OTAResult is the body of POST /api/cyd-creator/device/ota-result.
type OTAResult struct {
	BuildID string `json:"build_id"`
	OK      bool   `json:"ok"`
	Error   string `json:"error"`
}

func deviceBadRequest(message string) error {
	return NewError(CodeBadRequest, message, nil)
}

func validOTAState(state string) bool {
	switch state {
	case "", OTAStateIdle, OTAStateDownloading, OTAStateRolledBack, OTAStateFailed:
		return true
	}
	return false
}

func (hb Heartbeat) validate() error {
	switch {
	case hb.BuildID != "" && !validElementID(hb.BuildID):
		return deviceBadRequest("build_id is invalid")
	case !deviceVersionPattern.MatchString(hb.TemplateVersion):
		return deviceBadRequest("template_version is invalid")
	case hb.IP != "" && net.ParseIP(hb.IP) == nil:
		return deviceBadRequest("ip must be an IP address")
	case hb.RSSI < -127 || hb.RSSI > 0:
		return deviceBadRequest("rssi must be between -127 and 0")
	case hb.FreeHeap < 0 || hb.UptimeS < 0:
		return deviceBadRequest("free_heap and uptime_s must not be negative")
	case !deviceReasonPattern.MatchString(hb.ResetReason):
		return deviceBadRequest("reset_reason is invalid")
	case !validOTAState(hb.OTAState):
		return deviceBadRequest("ota_state must be idle, downloading, rolled_back or failed")
	case utf8.RuneCountInString(hb.OTAError) > maxOTAErrorRunes:
		return deviceBadRequest("ota_error is too long")
	}
	return nil
}

// cleanDeviceText replaces control characters and line/paragraph separators
// in device-supplied text by spaces and drops everything else that is not
// graphic (format characters such as bidirectional overrides and zero-width
// marks, unassigned or private code points, invalid UTF-8).
func cleanDeviceText(s string) string {
	s = strings.Map(func(r rune) rune {
		switch {
		case unicode.IsControl(r) || unicode.In(r, unicode.Zl, unicode.Zp):
			return ' '
		case r == utf8.RuneError || !unicode.IsGraphic(r):
			return -1
		}
		return r
	}, s)
	return strings.TrimSpace(s)
}

// otaError is the stored last_error of a failed or rolled back update:
// "<state>" or "<state>: <device message>". The UI translates the state.
func otaError(state, message string) string {
	if message = cleanDeviceText(message); message != "" {
		return state + ": " + message
	}
	return state
}

// authenticateDevice resolves the bearer token to its device. OTA must be on,
// the client address must not have failed with too many distinct tokens, the
// token must belong to a device that is not revoked and, while pending, not
// expired (an expired pending device is removed). Every failure is
// CYD_UNAUTHORIZED.
func (s *Service) authenticateDevice(ctx context.Context, req DeviceRequest) (deviceRow, error) {
	if !s.Limits().OTA.Enabled {
		return deviceRow{}, NewError(CodeOTADisabled, "updates over Wi-Fi are switched off", nil)
	}
	now := s.now()
	if s.deviceLimits.authBlocked(req.ClientIP, now) {
		return deviceRow{}, NewError(CodeRateLimited, "too many failed device requests; try again later", nil)
	}
	token := strings.TrimSpace(req.Token)
	fail := func() (deviceRow, error) {
		s.deviceLimits.authFailed(req.ClientIP, token, now)
		return deviceRow{}, NewError(CodeUnauthorized, "the device token is not valid", nil)
	}
	if len(token) != len(deviceTokenPrefix)+2*deviceTokenBytes || !strings.HasPrefix(token, deviceTokenPrefix) {
		return fail()
	}
	d, err := scanDevice(s.db.QueryRowContext(ctx, `SELECT `+deviceColumns+` FROM cc_devices WHERE token_hash=?`, sha256Hex([]byte(token))))
	if errors.Is(err, sql.ErrNoRows) {
		return fail()
	}
	if err != nil {
		return deviceRow{}, s.internalError("authenticate device", err)
	}
	if d.Status == DeviceRevoked {
		return fail()
	}
	if deviceExpired(d, now) {
		// d was read without the project lock: the delete only removes a row
		// that is still pending and overdue.
		unlock := s.lockProject(d.ProjectID)
		if _, err := s.removeExpiredDevice(ctx, d, now); err != nil {
			s.logger.Warn("CYD Creator expired device could not be removed", "device_id", d.ID, "error", err)
		}
		unlock()
		return fail()
	}
	return d, nil
}

// reloadDevice re-reads an authenticated device under the project lock; a
// device removed in between makes the request unauthorized.
func (s *Service) reloadDevice(ctx context.Context, id string) (deviceRow, error) {
	d, err := s.loadDeviceRow(ctx, id)
	if ErrorCode(err) == CodeDeviceNotFound {
		return deviceRow{}, NewError(CodeUnauthorized, "the device token is not valid", nil)
	}
	return d, err
}

// rolledBackFrom returns the build id of a rollback message ("rolled back
// from <build id>"), or "".
func rolledBackFrom(message string) string {
	id, ok := strings.CutPrefix(cleanDeviceText(message), rolledBackPrefix)
	if !ok {
		return ""
	}
	return strings.TrimSpace(id)
}

// applyHeartbeat returns the device state after a heartbeat, mirroring the
// firmware (plan 1b "Device behaviour"). The first heartbeat activates a
// pending device. ota_state "rolled_back" with ota_error "rolled back from
// <id>" is reported for the whole boot after a rollback. The rollback is
// judged by that failed build, not by the stored state: while the target is
// the failed build, the target is reset to the running build and last_error
// stored, so the build that did not start is not offered again. That also
// catches a second rollback in the same rolled_back state (the user picked
// another build during the boot and it failed too), and keeps any other
// target the user picks meanwhile. Picking the failed build again during the
// same boot is reset as well; it can be retried once the board restarted.
// Without a build id in the message only the transition into the state
// counts. "failed" is reported after a failed download or verification until
// the next attempt: entering it stores last_error and keeps the target,
// because the firmware retries a failed build after 10 minutes. A device that
// reports the target build after running another one finished its update
// (health confirmation): last_error is cleared.
func applyHeartbeat(d deviceRow, hb Heartbeat, now time.Time) deviceRow {
	running := hb.BuildID
	if running == "" {
		running = d.BuildID
	}
	switch hb.OTAState {
	case OTAStateRolledBack:
		failed := rolledBackFrom(hb.OTAError)
		if failed == "" && d.OTAState != OTAStateRolledBack {
			failed = d.TargetBuildID
		}
		if failed != "" && failed == d.TargetBuildID && failed != running {
			d.LastError = otaError(OTAStateRolledBack, hb.OTAError)
			d.TargetBuildID = running
		} else if d.OTAState != OTAStateRolledBack {
			d.LastError = otaError(OTAStateRolledBack, hb.OTAError)
		}
	case OTAStateFailed:
		if d.OTAState != OTAStateFailed {
			d.LastError = otaError(OTAStateFailed, hb.OTAError)
		}
	default:
		if running == d.TargetBuildID && running != d.BuildID {
			d.LastError = ""
		}
	}
	d.Status = DeviceActive
	d.ExpiresAt = ""
	d.LastSeen = formatTime(now)
	d.IP, d.RSSI, d.FreeHeap = hb.IP, hb.RSSI, hb.FreeHeap
	d.TemplateVersion = hb.TemplateVersion
	d.OTAState = hb.OTAState
	d.BuildID = running
	return d
}

// deviceChanged reports a change of state, builds, target, OTA state, error or
// name, which is always worth an event; plain heartbeats that only move
// last_seen, RSSI or free heap do not fill the persisted event log.
func deviceChanged(a, b deviceRow) bool {
	return a.Status != b.Status || a.BuildID != b.BuildID || a.TargetBuildID != b.TargetBuildID || a.OTAState != b.OTAState ||
		a.LastError != b.LastError || a.Name != b.Name
}

// deviceDetailsChanged reports a new IP or template version: an event at most
// once per heartbeat interval (DeviceHeartbeat), so a device that alternates
// them cannot flood the event log.
func deviceDetailsChanged(a, b deviceRow) bool {
	return a.IP != b.IP || a.TemplateVersion != b.TemplateVersion
}

// offerUpdate returns the update for a device whose target differs from the
// build it runs, or nil.
func (s *Service) offerUpdate(ctx context.Context, d deviceRow) *FirmwareUpdate {
	if d.TargetBuildID == "" || d.TargetBuildID == d.BuildID {
		return nil
	}
	b, fw, err := s.firmwareBuild(ctx, d.ProjectID, d.TargetBuildID)
	if err != nil || b.Board != d.Board {
		return nil
	}
	return &FirmwareUpdate{BuildID: b.ID, Size: fw.Size, SHA256: fw.SHA256, URL: DeviceFirmwarePath + b.ID}
}

// DeviceHeartbeat records a heartbeat (device_id must be the token's device)
// and answers with the interval and the pending update.
func (s *Service) DeviceHeartbeat(ctx context.Context, req DeviceRequest, hb Heartbeat) (HeartbeatResponse, error) {
	d, err := s.authenticateDevice(ctx, req)
	if err != nil {
		return HeartbeatResponse{}, err
	}
	if hb.DeviceID != d.ID {
		return HeartbeatResponse{}, NewError(CodeUnauthorized, "the device token belongs to another device", nil)
	}
	if err := hb.validate(); err != nil {
		return HeartbeatResponse{}, err
	}
	now := s.now()
	if !s.deviceLimits.allow("hb:"+d.ID, now, heartbeatsPerMinute, time.Minute) {
		return HeartbeatResponse{}, NewError(CodeRateLimited, "heartbeats arrive too often", nil)
	}
	unlock := s.lockProject(d.ProjectID)
	defer unlock()
	if d, err = s.reloadDevice(ctx, d.ID); err != nil {
		return HeartbeatResponse{}, s.internalError("device heartbeat", err)
	}
	interval := s.heartbeatInterval()
	wasOnline := d.Status == DeviceActive && deviceOnline(d.LastSeen, now, interval)
	next := applyHeartbeat(d, hb, now)
	if _, err := s.db.ExecContext(ctx, `UPDATE cc_devices SET status=?, expires_at=?, last_seen=?, ip=?, rssi=?, free_heap=?,
		template_version=?, ota_state=?, last_error=?, build_id=?, target_build_id=? WHERE id=?`,
		next.Status, next.ExpiresAt, next.LastSeen, next.IP, next.RSSI, next.FreeHeap, next.TemplateVersion, next.OTAState,
		next.LastError, next.BuildID, next.TargetBuildID, next.ID); err != nil {
		return HeartbeatResponse{}, s.internalError("device heartbeat", err)
	}
	switch {
	case !wasOnline || deviceChanged(d, next):
		s.emitDevice(next, now)
	case deviceDetailsChanged(d, next) && s.deviceLimits.allow("ev:"+d.ID, now, 1, interval):
		s.emitDevice(next, now)
	}
	return HeartbeatResponse{IntervalS: int(interval / time.Second), Update: s.offerUpdate(ctx, next)}, nil
}

// DeviceFirmware opens firmware.bin (the app image, never merged.bin) of the
// device's current target build, a succeeded build of its own project for its
// board, at most firmwareDownloadsPerWindow times per window.
func (s *Service) DeviceFirmware(ctx context.Context, req DeviceRequest, buildID string) (io.ReadSeekCloser, ArtifactInfo, error) {
	d, err := s.authenticateDevice(ctx, req)
	if err != nil {
		return nil, ArtifactInfo{}, err
	}
	notFound := NewError(CodeNotFound, "this build is not the device's update target", map[string]any{"build_id": clipEcho(buildID)})
	if buildID == "" || buildID != d.TargetBuildID {
		return nil, ArtifactInfo{}, notFound
	}
	b, _, err := s.firmwareBuild(ctx, d.ProjectID, buildID)
	if err != nil {
		if ErrorCode(err) != "" {
			return nil, ArtifactInfo{}, notFound
		}
		return nil, ArtifactInfo{}, s.internalError("device firmware", err)
	}
	if b.Board != d.Board {
		return nil, ArtifactInfo{}, notFound
	}
	if !s.deviceLimits.allow("fw:"+d.ID, s.now(), firmwareDownloadsPerWindow, firmwareDownloadWindow) {
		return nil, ArtifactInfo{}, NewError(CodeRateLimited, "at most 3 firmware downloads per 10 minutes", nil)
	}
	return s.openFirmware(ctx, b)
}

// firmwareReader streams firmware.bin cut to the size of its build record.
type firmwareReader struct {
	*io.SectionReader
	io.Closer
}

// openFirmware opens firmware.bin of b for streaming. The file must still have
// the size its build record lists (the size the update offer announced, which
// the device checks against Content-Length) and fit one app slot of the board;
// the reader never yields more than that size.
func (s *Service) openFirmware(ctx context.Context, b Build) (io.ReadSeekCloser, ArtifactInfo, error) {
	board, err := BoardByID(b.Board)
	if err != nil {
		return nil, ArtifactInfo{}, s.internalError("device firmware", fmt.Errorf("board of build %s: %v", b.ID, err))
	}
	slot, err := AppSlotSize(board.FlashMB)
	if err != nil {
		return nil, ArtifactInfo{}, s.internalError("device firmware", fmt.Errorf("app slot of build %s: %v", b.ID, err))
	}
	f, info, err := s.OpenArtifact(ctx, b.ID, ArtifactFirmwareBin)
	if err != nil {
		return nil, ArtifactInfo{}, err
	}
	at, ok := f.(io.ReaderAt)
	size, err := f.Seek(0, io.SeekEnd)
	if err != nil || !ok || size != info.Size || size <= 0 || size > int64(slot) {
		_ = f.Close()
		return nil, ArtifactInfo{}, s.internalError("device firmware", fmt.Errorf(
			"firmware.bin of %s does not match its build record (%d bytes on disk, %d recorded, app slot %d)", b.ID, size, info.Size, slot))
	}
	return firmwareReader{SectionReader: io.NewSectionReader(at, 0, size), Closer: f}, info, nil
}

// DeviceOTAResult records an installation report (plan 1b): the new image
// sends {build_id: <own>, ok: true} after its first good heartbeat, which
// clears last_error; an old image that booted after a rollback sends
// {build_id: <failed>, ok: false, error: "rolled_back"}, which stores
// last_error "rolled_back" and resets the target while it is the failed build
// (a repeated report changes nothing); a download or verification
// failure {ok: false, error: "<reason>"} stores "failed: <reason>" and keeps
// the target (the firmware retries after 10 minutes). build_id must be the
// device's target, the build it runs or another build of its project.
func (s *Service) DeviceOTAResult(ctx context.Context, req DeviceRequest, res OTAResult) error {
	d, err := s.authenticateDevice(ctx, req)
	if err != nil {
		return err
	}
	now := s.now()
	if !s.deviceLimits.allow("res:"+d.ID, now, otaResultsPerWindow, firmwareDownloadWindow) {
		return NewError(CodeRateLimited, "too many OTA results", nil)
	}
	if !validElementID(res.BuildID) || utf8.RuneCountInString(res.Error) > maxOTAErrorRunes {
		return deviceBadRequest("build_id is required and error may have at most 200 characters")
	}
	unlock := s.lockProject(d.ProjectID)
	defer unlock()
	if d, err = s.reloadDevice(ctx, d.ID); err != nil {
		return s.internalError("device ota result", err)
	}
	if res.BuildID != d.TargetBuildID && res.BuildID != d.BuildID {
		b, err := s.loadBuild(ctx, res.BuildID)
		if err != nil || b.ProjectID != d.ProjectID {
			return NewError(CodeBadRequest, "build_id is not a build of this device's project", map[string]any{"build_id": res.BuildID})
		}
	}
	before := d
	switch {
	case res.OK:
		d.LastError = ""
	case cleanDeviceText(res.Error) == OTAStateRolledBack:
		// Judged by the failed build like the heartbeat: a target that is the
		// failed build is reset even when the state already is rolled_back.
		if res.BuildID == d.TargetBuildID && res.BuildID != d.BuildID {
			d.TargetBuildID = d.BuildID
			d.LastError = OTAStateRolledBack
		} else if d.OTAState != OTAStateRolledBack {
			d.LastError = OTAStateRolledBack
		}
		d.OTAState = OTAStateRolledBack
	default:
		d.OTAState = OTAStateFailed
		d.LastError = otaError(OTAStateFailed, res.Error)
	}
	if !deviceChanged(before, d) {
		return nil
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE cc_devices SET ota_state=?, last_error=?, target_build_id=? WHERE id=?`,
		d.OTAState, d.LastError, d.TargetBuildID, d.ID); err != nil {
		return s.internalError("device ota result", err)
	}
	s.emitDevice(d, now)
	return nil
}
