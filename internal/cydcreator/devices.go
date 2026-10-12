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
	"unicode"
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

// validDeviceName trims name and checks 1..64 characters without control characters.
func validDeviceName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > maxDeviceNameRunes || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return "", NewError(CodeBadRequest,
			fmt.Sprintf("device name must be 1..%d characters without control characters", maxDeviceNameRunes), nil)
	}
	return name, nil
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
	d, err := scanDevice(s.db.QueryRowContext(ctx, `SELECT `+deviceColumns+` FROM cc_devices WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return deviceRow{}, NewError(CodeDeviceNotFound, "device not found", map[string]any{"device_id": id})
	}
	if err != nil {
		return deviceRow{}, fmt.Errorf("read cyd creator device: %w", err)
	}
	return d, nil
}

// removeDevice deletes a device row and announces it as revoked.
func (s *Service) removeDevice(ctx context.Context, d deviceRow, now time.Time) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM cc_devices WHERE id=?`, d.ID)
	if err != nil {
		return fmt.Errorf("delete cyd creator device: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return NewError(CodeDeviceNotFound, "device not found", map[string]any{"device_id": d.ID})
	}
	d.Status = DeviceRevoked
	s.emitDevice(d, now)
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
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM cc_devices WHERE project_id=?`, projectID).Scan(&count); err != nil {
		return MintedDevice{}, s.internalError("mint device", err)
	}
	if count >= maxDevicesPerProject {
		return MintedDevice{}, NewError(CodeBusy, "this project has too many devices; remove one first",
			map[string]any{"limit": maxDevicesPerProject})
	}
	if name == "" {
		name = defaultDeviceName(row.Name, count+1)
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

// ListDevices returns the devices of a project, oldest first. Pending devices
// whose first heartbeat is overdue are removed first.
func (s *Service) ListDevices(ctx context.Context, projectID string) ([]Device, error) {
	if _, err := s.loadProjectRow(ctx, projectID); err != nil {
		return nil, s.internalError("list devices", err)
	}
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
		return Device{}, NewError(CodeDeviceNotFound, "device not found", map[string]any{"device_id": deviceID})
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
	if _, err := s.db.ExecContext(ctx, `UPDATE cc_devices SET name=?, target_build_id=?, last_error=? WHERE id=?`,
		d.Name, d.TargetBuildID, d.LastError, d.ID); err != nil {
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
