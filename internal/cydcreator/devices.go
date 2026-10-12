package cydcreator

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// TokenScopeDevice names the credential a flashed board uses for the device
// endpoints under /api/cyd-creator/device/. Device tokens are minted only by
// MintDevice, bound to one cc_devices row (and through it to one project),
// stored as SHA-256 in cc_devices.token_hash and never placed in the API token
// store: no other AuraGo route accepts them, and the device endpoints accept
// nothing else. POST /api/tokens refuses the scope (internal/server/token_scopes.go).
const TokenScopeDevice = "cyd_creator_device"

// Device states (overview "Device" JSON). A minted device is pending until its
// first heartbeat and expires DevicePendingTTL after minting without one.
// RevokeDevice deletes the row; "revoked" only appears in the device_changed
// event that announces a removal (also of an expired pending device).
const (
	DevicePending = "pending"
	DeviceActive  = "active"
	DeviceRevoked = "revoked"
)

// OTA states a device reports in its heartbeat (overview "Device ↔ server").
const (
	OTAStateIdle        = "idle"
	OTAStateDownloading = "downloading"
	OTAStateRolledBack  = "rolled_back"
	OTAStateFailed      = "failed"
)

const (
	// DevicePendingTTL is how long a minted device waits for its first heartbeat.
	DevicePendingTTL = time.Hour
	// deviceOnlineFactor: a device is online while its last heartbeat is at most
	// this many heartbeat intervals old.
	deviceOnlineFactor   = 3
	deviceTokenPrefix    = "ccdt_"
	deviceTokenBytes     = 32
	maxDeviceNameRunes   = 64
	maxDevicesPerProject = 64
)

// Device is one flashed board (overview "Device" JSON). Online is computed
// when the device is read.
type Device struct {
	ID              string `json:"id"`
	ProjectID       string `json:"project_id"`
	Name            string `json:"name"`
	Board           string `json:"board"`
	Status          string `json:"status"`
	BuildID         string `json:"build_id"`
	TargetBuildID   string `json:"target_build_id"`
	IP              string `json:"ip"`
	RSSI            int    `json:"rssi"`
	FreeHeap        int64  `json:"free_heap"`
	TemplateVersion string `json:"template_version"`
	OTAState        string `json:"ota_state"`
	LastError       string `json:"last_error"`
	LastSeen        string `json:"last_seen"`
	Online          bool   `json:"online"`
	CreatedAt       string `json:"created_at"`
}

// DeviceEndpoint is where a device reaches AuraGo: the base URL written into
// the config blob and the lowercase SHA-256 of the DER leaf certificate it
// pins (empty for http://). The HTTP layer resolves it from the configuration.
type DeviceEndpoint struct {
	URL        string
	CertSHA256 string
}

// MintDeviceInput is POST /projects/{id}/devices plus the resolved endpoint.
// Name is optional ("<project name> #<n>" when empty).
type MintDeviceInput struct {
	BuildID  string
	Name     string
	Endpoint DeviceEndpoint
}

// MintedDevice is the answer of POST /projects/{id}/devices. ConfigBlob is the
// 4096-byte cydcc partition (base64 in JSON), written by the flasher at
// ConfigOffset (ConfigPartitionOffset of the board's flash size).
type MintedDevice struct {
	Device       Device `json:"device"`
	ConfigBlob   []byte `json:"config_blob"`
	ConfigOffset int    `json:"config_offset"`
}

// DeviceUpdate is PATCH /devices/{id}: a new name and/or OTA target.
type DeviceUpdate struct {
	Name          *string `json:"name"`
	TargetBuildID *string `json:"target_build_id"`
}

type deviceRow struct {
	Device
	TokenHash string
	ExpiresAt string
}

const deviceColumns = `id,project_id,name,board,status,token_hash,build_id,target_build_id,ip,rssi,free_heap,template_version,ota_state,last_error,last_seen,created_at,expires_at`

func scanDevice(row rowScanner) (deviceRow, error) {
	var d deviceRow
	err := row.Scan(&d.ID, &d.ProjectID, &d.Name, &d.Board, &d.Status, &d.TokenHash, &d.BuildID, &d.TargetBuildID,
		&d.IP, &d.RSSI, &d.FreeHeap, &d.TemplateVersion, &d.OTAState, &d.LastError, &d.LastSeen, &d.CreatedAt, &d.ExpiresAt)
	return d, err
}

// newDeviceToken returns a fresh device token (ccdt_ + 64 hex characters, 256
// random bits) and the SHA-256 hex digest that is stored instead of it.
func newDeviceToken() (token, hash string, err error) {
	b := make([]byte, deviceTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generate device token: %w", err)
	}
	token = deviceTokenPrefix + hex.EncodeToString(b)
	return token, sha256Hex([]byte(token)), nil
}

// normalizeDeviceEndpoint returns the URL without surrounding spaces and
// trailing slashes (the firmware appends /api/cyd-creator/device/…) and checks
// it with ValidateDeviceURL, the config blob's own rule.
func normalizeDeviceEndpoint(e DeviceEndpoint) (DeviceEndpoint, error) {
	raw := strings.TrimRight(strings.TrimSpace(e.URL), "/")
	if err := ValidateDeviceURL(raw, e.CertSHA256); err != nil {
		return DeviceEndpoint{}, err
	}
	return DeviceEndpoint{URL: raw, CertSHA256: e.CertSHA256}, nil
}

// validDeviceName trims name and checks 1..64 characters of valid UTF-8
// without control characters, format characters (bidirectional overrides,
// zero-width marks) or line/paragraph separators (planRuneNotAllowed).
func validDeviceName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || !utf8.ValidString(name) || utf8.RuneCountInString(name) > maxDeviceNameRunes ||
		strings.IndexFunc(name, func(r rune) bool { return planRuneNotAllowed(r, false) }) >= 0 {
		return "", NewError(CodeBadRequest,
			fmt.Sprintf("device name must be 1..%d characters without control or format characters", maxDeviceNameRunes), nil)
	}
	return name, nil
}

// deviceIDPrefix starts every id MintDevice issues.
const deviceIDPrefix = "ccd_"

// validDeviceID reports an id the service could have issued ("ccd_" plus
// element id characters); anything else is not found without a query.
func validDeviceID(id string) bool {
	return strings.HasPrefix(id, deviceIDPrefix) && validElementID(id)
}

// deviceNotFound is CYD_DEVICE_NOT_FOUND with the (clipped) id.
func deviceNotFound(id string) error {
	return NewError(CodeDeviceNotFound, "device not found", map[string]any{"device_id": clipEcho(id)})
}

// defaultDeviceName is "<project name> #<n>", shortening the project name so
// the result stays within maxDeviceNameRunes.
func defaultDeviceName(projectName string, n int) string {
	suffix := fmt.Sprintf(" #%d", n)
	runes := []rune(strings.TrimSpace(projectName))
	if room := maxDeviceNameRunes - utf8.RuneCountInString(suffix); len(runes) > room {
		runes = runes[:room]
	}
	return strings.TrimSpace(string(runes)) + suffix
}

// heartbeatInterval is the configured heartbeat interval (30..600 s).
func (s *Service) heartbeatInterval() time.Duration {
	seconds := s.Limits().OTA.HeartbeatSeconds
	if seconds < 30 {
		seconds = 30
	}
	if seconds > 600 {
		seconds = 600
	}
	return time.Duration(seconds) * time.Second
}

// deviceOnline reports a heartbeat within deviceOnlineFactor intervals.
func deviceOnline(lastSeen string, now time.Time, interval time.Duration) bool {
	if lastSeen == "" {
		return false
	}
	seen, err := time.Parse(timeLayout, lastSeen)
	if err != nil {
		return false
	}
	return now.Sub(seen) <= time.Duration(deviceOnlineFactor)*interval
}

// deviceExpired reports a pending device whose first heartbeat is overdue.
func deviceExpired(d deviceRow, now time.Time) bool {
	return d.Status == DevicePending && d.ExpiresAt != "" && d.ExpiresAt <= formatTime(now)
}

func (s *Service) publicDevice(d deviceRow, now time.Time) Device {
	out := d.Device
	out.Online = out.Status == DeviceActive && deviceOnline(out.LastSeen, now, s.heartbeatInterval())
	return out
}

// emitDevice announces a new state of a device on the project stream.
func (s *Service) emitDevice(d deviceRow, now time.Time) Device {
	dev := s.publicDevice(d, now)
	s.Emit(d.ProjectID, EventDeviceChanged, "", "", map[string]any{"device": dev})
	return dev
}

// loadDeviceRow returns the device or *Error CYD_DEVICE_NOT_FOUND.
func (s *Service) loadDeviceRow(ctx context.Context, id string) (deviceRow, error) {
	if !validDeviceID(id) {
		return deviceRow{}, deviceNotFound(id)
	}
	d, err := scanDevice(s.db.QueryRowContext(ctx, `SELECT `+deviceColumns+` FROM cc_devices WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return deviceRow{}, deviceNotFound(id)
	}
	if err != nil {
		return deviceRow{}, fmt.Errorf("read cyd creator device: %w", err)
	}
	return d, nil
}

// removeDevice deletes a device row and announces it as revoked. Callers hold
// the project lock (every cc_devices delete does, DeleteProject included), so
// a writer that re-read the row under that lock never updates a deleted one.
func (s *Service) removeDevice(ctx context.Context, d deviceRow, now time.Time) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM cc_devices WHERE id=?`, d.ID)
	if err != nil {
		return fmt.Errorf("delete cyd creator device: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return deviceNotFound(d.ID)
	}
	d.Status = DeviceRevoked
	s.emitDevice(d, now)
	return nil
}

// removeExpiredDevice deletes d only while the stored row is still a pending
// device whose first heartbeat is overdue at now (d may have been read before
// the project lock: a device activated meanwhile stays). It announces the
// removal and reports whether a row was deleted.
func (s *Service) removeExpiredDevice(ctx context.Context, d deviceRow, now time.Time) (bool, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM cc_devices WHERE id=? AND status=? AND expires_at<>'' AND expires_at<=?`,
		d.ID, DevicePending, formatTime(now))
	if err != nil {
		return false, fmt.Errorf("delete expired cyd creator device: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil || n == 0 {
		return false, err
	}
	d.Status = DeviceRevoked
	s.emitDevice(d, now)
	return true, nil
}

// saveDeviceSettings stores the user-editable fields of d (name, target,
// last_error). A row that no longer exists is CYD_DEVICE_NOT_FOUND, so the
// caller emits nothing (no "pending" event after the "revoked" one).
func (s *Service) saveDeviceSettings(ctx context.Context, d deviceRow) error {
	res, err := s.db.ExecContext(ctx, `UPDATE cc_devices SET name=?, target_build_id=?, last_error=? WHERE id=?`,
		d.Name, d.TargetBuildID, d.LastError, d.ID)
	if err != nil {
		return fmt.Errorf("update cyd creator device: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return fmt.Errorf("update cyd creator device: %w", err)
	} else if n == 0 {
		return deviceNotFound(d.ID)
	}
	return nil
}

// purgeExpiredDevices removes pending devices of a project whose first
// heartbeat is overdue (their token stops working at the same moment).
func (s *Service) purgeExpiredDevices(ctx context.Context, projectID string) {
	now := s.now()
	rows, err := s.db.QueryContext(ctx, `SELECT `+deviceColumns+` FROM cc_devices
		WHERE project_id=? AND status=? AND expires_at<>'' AND expires_at<=?`, projectID, DevicePending, formatTime(now))
	if err != nil {
		s.logger.Warn("CYD Creator expired devices could not be listed", "project_id", projectID, "error", err)
		return
	}
	var expired []deviceRow
	for rows.Next() {
		if d, err := scanDevice(rows); err == nil {
			expired = append(expired, d)
		}
	}
	_ = rows.Close()
	for _, d := range expired {
		if err := s.removeDevice(ctx, d, now); err != nil && ErrorCode(err) != CodeDeviceNotFound {
			s.logger.Warn("CYD Creator expired device could not be removed", "device_id", d.ID, "error", err)
		}
	}
}

// firmwareBuild returns a succeeded build of the project together with its
// firmware.bin artefact: the only builds a device may be flashed with or
// updated to.
func (s *Service) firmwareBuild(ctx context.Context, projectID, buildID string) (Build, ArtifactInfo, error) {
	if !validBuildID(buildID) {
		return Build{}, ArtifactInfo{}, NewError(CodeBadRequest, "a valid build_id is required", nil)
	}
	b, err := s.loadBuild(ctx, buildID)
	if ErrorCode(err) == CodeNotFound || (err == nil && b.ProjectID != projectID) {
		return Build{}, ArtifactInfo{}, NewError(CodeNotFound, "build not found in this project", map[string]any{"build_id": buildID})
	}
	if err != nil {
		return Build{}, ArtifactInfo{}, err
	}
	if b.Status != BuildSucceeded {
		return Build{}, ArtifactInfo{}, NewError(CodeBadRequest, "only a succeeded build can be flashed or installed",
			map[string]any{"build_id": buildID, "status": b.Status})
	}
	for _, a := range b.Artifacts {
		if a.Name == ArtifactFirmwareBin {
			return b, a, nil
		}
	}
	return Build{}, ArtifactInfo{}, NewError(CodeBadRequest, "the build has no firmware image", map[string]any{"build_id": buildID})
}

// MintDevice registers a device for flashing: it needs a succeeded build of the
// same project, creates a pending device bound to the project with a fresh
// token (only its hash is stored) and returns the cydcc config blob with the
// device id, the token, the endpoint URL and the certificate pin. The token is
// in the blob only; it is never logged or returned again.
func (s *Service) MintDevice(ctx context.Context, projectID string, in MintDeviceInput) (MintedDevice, error) {
	if !s.Limits().OTA.Enabled {
		return MintedDevice{}, NewError(CodeOTADisabled, "updates over Wi-Fi are switched off", nil)
	}
	endpoint, err := normalizeDeviceEndpoint(in.Endpoint)
	if err != nil {
		return MintedDevice{}, err
	}
	name := strings.TrimSpace(in.Name)
	if name != "" {
		if name, err = validDeviceName(name); err != nil {
			return MintedDevice{}, err
		}
	}
	unlock := s.lockProject(projectID)
	defer unlock()
	row, err := s.loadProjectRow(ctx, projectID)
	if err != nil {
		return MintedDevice{}, s.internalError("mint device", err)
	}
	build, _, err := s.firmwareBuild(ctx, projectID, in.BuildID)
	if err != nil {
		return MintedDevice{}, s.internalError("mint device", err)
	}
	board, err := BoardByID(build.Board)
	if err != nil {
		return MintedDevice{}, err
	}
	offset, err := ConfigPartitionOffset(board.FlashMB)
	if err != nil {
		return MintedDevice{}, s.internalError("mint device", err)
	}
	s.purgeExpiredDevices(ctx, projectID)
	names, err := s.deviceNames(ctx, projectID)
	if err != nil {
		return MintedDevice{}, s.internalError("mint device", err)
	}
	if len(names) >= maxDevicesPerProject {
		return MintedDevice{}, NewError(CodeBusy, "this project has too many devices; remove one first",
			map[string]any{"limit": maxDevicesPerProject})
	}
	if name == "" {
		// The next number from count+1 on that no device uses: after a revoke
		// count+1 may still be taken ("X #1", "X #3" → "X #4", never a second "X #3").
		n := len(names) + 1
		for names[defaultDeviceName(row.Name, n)] {
			n++
		}
		name = defaultDeviceName(row.Name, n)
	}
	token, hash, err := newDeviceToken()
	if err != nil {
		return MintedDevice{}, s.internalError("mint device", err)
	}
	now := s.now()
	d := deviceRow{
		Device: Device{ID: s.ids.NewID("ccd"), ProjectID: projectID, Name: name, Board: board.ID, Status: DevicePending,
			BuildID: build.ID, TargetBuildID: build.ID, CreatedAt: formatTime(now)},
		TokenHash: hash,
		ExpiresAt: formatTime(now.Add(DevicePendingTTL)),
	}
	blob, err := EncodeDeviceConfig(DeviceConfig{URL: endpoint.URL, DeviceID: d.ID, Token: token, CertSHA256: endpoint.CertSHA256})
	if err != nil {
		return MintedDevice{}, err
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO cc_devices(id,project_id,name,board,status,token_hash,build_id,target_build_id,created_at,expires_at)
		VALUES(?,?,?,?,?,?,?,?,?,?)`, d.ID, d.ProjectID, d.Name, d.Board, d.Status, d.TokenHash, d.BuildID, d.TargetBuildID,
		d.CreatedAt, d.ExpiresAt); err != nil {
		return MintedDevice{}, s.internalError("mint device", err)
	}
	return MintedDevice{Device: s.emitDevice(d, now), ConfigBlob: blob, ConfigOffset: offset}, nil
}

// deviceNames returns the set of device names of a project (at most
// maxDevicesPerProject rows) and, through its size, the device count.
func (s *Service) deviceNames(ctx context.Context, projectID string) (map[string]bool, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT name FROM cc_devices WHERE project_id=?`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	names := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names[name] = true
	}
	return names, rows.Err()
}

// ListDevices returns the devices of a project, oldest first. Pending devices
// whose first heartbeat is overdue are removed first, under the project lock
// like every other device write (lock order: project lock → eventsMu).
func (s *Service) ListDevices(ctx context.Context, projectID string) ([]Device, error) {
	if _, err := s.loadProjectRow(ctx, projectID); err != nil {
		return nil, s.internalError("list devices", err)
	}
	unlock := s.lockProject(projectID)
	defer unlock()
	s.purgeExpiredDevices(ctx, projectID)
	rows, err := s.db.QueryContext(ctx, `SELECT `+deviceColumns+` FROM cc_devices WHERE project_id=? ORDER BY created_at, id`, projectID)
	if err != nil {
		return nil, s.internalError("list devices", err)
	}
	defer rows.Close()
	now := s.now()
	out := []Device{}
	for rows.Next() {
		d, err := scanDevice(rows)
		if err != nil {
			return nil, s.internalError("list devices", err)
		}
		out = append(out, s.publicDevice(d, now))
	}
	return out, s.internalError("list devices", rows.Err())
}

// UpdateDevice renames a device and/or sets its OTA target. The target must be
// a succeeded build of the device's project for the device's board; a new
// target clears the last OTA error. Setting the build the device already runs
// cancels a pending update.
func (s *Service) UpdateDevice(ctx context.Context, deviceID string, in DeviceUpdate) (Device, error) {
	if in.Name == nil && in.TargetBuildID == nil {
		return Device{}, NewError(CodeBadRequest, "send name and/or target_build_id", nil)
	}
	d, err := s.loadDeviceRow(ctx, deviceID)
	if err != nil {
		return Device{}, s.internalError("update device", err)
	}
	unlock := s.lockProject(d.ProjectID)
	defer unlock()
	if d, err = s.loadDeviceRow(ctx, deviceID); err != nil {
		return Device{}, s.internalError("update device", err)
	}
	now := s.now()
	if deviceExpired(d, now) {
		_ = s.removeDevice(ctx, d, now)
		return Device{}, deviceNotFound(deviceID)
	}
	if in.Name != nil {
		name, err := validDeviceName(*in.Name)
		if err != nil {
			return Device{}, err
		}
		d.Name = name
	}
	if in.TargetBuildID != nil {
		if !s.Limits().OTA.Enabled {
			return Device{}, NewError(CodeOTADisabled, "updates over Wi-Fi are switched off", nil)
		}
		build, _, err := s.firmwareBuild(ctx, d.ProjectID, *in.TargetBuildID)
		if err != nil {
			return Device{}, s.internalError("update device", err)
		}
		if build.Board != d.Board {
			return Device{}, NewError(CodeBadRequest, "the build is for another board",
				map[string]any{"build_id": build.ID, "board": build.Board, "device_board": d.Board})
		}
		if d.TargetBuildID != build.ID {
			d.TargetBuildID = build.ID
			d.LastError = ""
		}
	}
	if err := s.saveDeviceSettings(ctx, d); err != nil {
		return Device{}, s.internalError("update device", err)
	}
	return s.emitDevice(d, now), nil
}

// RevokeDevice removes a device: its token stops working at once.
func (s *Service) RevokeDevice(ctx context.Context, deviceID string) error {
	d, err := s.loadDeviceRow(ctx, deviceID)
	if err != nil {
		return s.internalError("revoke device", err)
	}
	unlock := s.lockProject(d.ProjectID)
	defer unlock()
	return s.internalError("revoke device", s.removeDevice(ctx, d, s.now()))
}
