package homedash

import (
	"context"
	"database/sql"
	"encoding/base32"
	"errors"
	"strings"
	"time"

	"aurago/internal/dbutil"
)

const (
	DeviceTokenPrefix = "hdk_"
	pairingCodeLen    = 26 // base32 of 16 bytes, no padding
)

// KioskDevice is one paired display. The token itself is never stored or returned again.
type KioskDevice struct {
	ID          string     `json:"id"`
	DashboardID string     `json:"dashboard_id"`
	Label       string     `json:"label"`
	ViewOnly    bool       `json:"view_only"`
	CreatedAt   time.Time  `json:"created_at"`
	LastSeenAt  *time.Time `json:"last_seen_at,omitempty"`
	LastIP      string     `json:"last_ip,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

// errPairingInvalid is the one error every pairing failure returns (unknown, used, expired,
// foreign dashboard, kiosk full), so a caller cannot tell the cases apart.
var errPairingInvalid = newError(CodePairingInvalid, "the pairing code is invalid or expired")

// CreatePairing stores only the SHA-256 of a single-use 128-bit code and returns the code once.
func (s *Store) CreatePairing(ctx context.Context, dashboardID, label string, viewOnly bool, ttl time.Duration) (string, time.Time, error) {
	if label = CleanText(label, 60); label == "" {
		label = "Display"
	}
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM hd_dashboards WHERE id = ?`, dashboardID).Scan(&n); err != nil {
		return "", time.Time{}, err
	} else if n == 0 {
		return "", time.Time{}, ErrDashboardNotFound
	}
	now := s.now()
	pairing := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(randomBytes(16))
	expires := now.Add(ttl)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", time.Time{}, err
	}
	defer tx.Rollback()
	// Expired rows go whether used or not, so they never pile up.
	if _, err := tx.ExecContext(ctx, `DELETE FROM hd_kiosk_pairings WHERE expires_at < ?`, ts(now)); err != nil {
		return "", time.Time{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO hd_kiosk_pairings (code_hash, dashboard_id, label, view_only, expires_at) VALUES (?, ?, ?, ?, ?)`,
		hashSecret(pairing), dashboardID, label, dbutil.BoolToInt(viewOnly), ts(expires)); err != nil {
		return "", time.Time{}, err
	}
	if err := tx.Commit(); err != nil {
		return "", time.Time{}, err
	}
	return pairing, expires, nil
}

// RedeemPairing consumes a code once and creates a device. Unknown, used, expired, foreign codes
// and a full kiosk all return the same pairing_invalid. The device token is returned once and
// stored as SHA-256.
func (s *Store) RedeemPairing(ctx context.Context, dashboardID, pairing, ip string, deviceTTL time.Duration, maxDevices int) (KioskDevice, string, error) {
	pairing = strings.ToUpper(strings.TrimSpace(pairing))
	if len(pairing) != pairingCodeLen {
		return KioskDevice{}, "", errPairingInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return KioskDevice{}, "", err
	}
	defer tx.Rollback()
	now := s.now()
	var label, expires string
	var viewOnly int
	var used sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT label, view_only, expires_at, used_at FROM hd_kiosk_pairings WHERE code_hash = ? AND dashboard_id = ?`,
		hashSecret(pairing), dashboardID).Scan(&label, &viewOnly, &expires, &used)
	if errors.Is(err, sql.ErrNoRows) {
		return KioskDevice{}, "", errPairingInvalid
	} else if err != nil {
		return KioskDevice{}, "", err
	}
	if used.Valid || !parseTS(expires).After(now) {
		return KioskDevice{}, "", errPairingInvalid
	}
	if maxDevices <= 0 { // fail closed: no cap configured means no pairing
		return KioskDevice{}, "", errPairingInvalid
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM hd_kiosk_devices WHERE expires_at IS NOT NULL AND expires_at < ?`, ts(now)); err != nil {
		return KioskDevice{}, "", err
	}
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM hd_kiosk_devices`).Scan(&count); err != nil {
		return KioskDevice{}, "", err
	}
	if count >= maxDevices {
		return KioskDevice{}, "", errPairingInvalid
	}
	res, err := tx.ExecContext(ctx, `UPDATE hd_kiosk_pairings SET used_at = ? WHERE code_hash = ? AND used_at IS NULL`, ts(now), hashSecret(pairing))
	if err != nil {
		return KioskDevice{}, "", err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return KioskDevice{}, "", errPairingInvalid
	}
	token := DeviceTokenPrefix + randomHex(32)
	dev := KioskDevice{ID: "kd_" + randomBase32(12), DashboardID: dashboardID, Label: label, ViewOnly: viewOnly == 1, CreatedAt: now, LastIP: CleanText(ip, 64)}
	var exp any
	if deviceTTL > 0 {
		e := now.Add(deviceTTL)
		dev.ExpiresAt, exp = &e, ts(e)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO hd_kiosk_devices (id, dashboard_id, label, token_hash, view_only, created_at, last_seen_at, last_ip, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, dev.ID, dashboardID, label, hashSecret(token), viewOnly, ts(now), ts(now), dev.LastIP, exp); err != nil {
		return KioskDevice{}, "", err
	}
	dev.LastSeenAt = &now
	if err := tx.Commit(); err != nil {
		return KioskDevice{}, "", err
	}
	return dev, token, nil
}

// DeviceByToken resolves a cookie token by the hash of its value; an expired device is deleted
// and reported revoked.
func (s *Store) DeviceByToken(ctx context.Context, token string) (KioskDevice, error) {
	unpaired := newError(CodeKioskUnpaired, "this display is not paired")
	if !strings.HasPrefix(token, DeviceTokenPrefix) || len(token) != len(DeviceTokenPrefix)+64 {
		return KioskDevice{}, unpaired
	}
	var d KioskDevice
	var viewOnly int
	var created string
	var seen, ip, expires sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT id, dashboard_id, label, view_only, created_at, last_seen_at, last_ip, expires_at
		FROM hd_kiosk_devices WHERE token_hash = ?`, hashSecret(token)).Scan(&d.ID, &d.DashboardID, &d.Label, &viewOnly, &created, &seen, &ip, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return KioskDevice{}, unpaired
	} else if err != nil {
		return KioskDevice{}, err
	}
	d.ViewOnly, d.CreatedAt, d.LastIP = viewOnly == 1, parseTS(created), ip.String
	if seen.Valid {
		t := parseTS(seen.String)
		d.LastSeenAt = &t
	}
	if expires.Valid {
		t := parseTS(expires.String)
		d.ExpiresAt = &t
		if !t.After(s.now()) {
			_, _ = s.db.ExecContext(ctx, `DELETE FROM hd_kiosk_devices WHERE id = ?`, d.ID)
			return KioskDevice{}, newError(CodeKioskRevoked, "this display's pairing expired")
		}
	}
	return d, nil
}

// TouchDevice records the last time and address a display was seen.
func (s *Store) TouchDevice(ctx context.Context, id, ip string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE hd_kiosk_devices SET last_seen_at = ?, last_ip = ? WHERE id = ?`, ts(s.now()), CleanText(ip, 64), id)
	return err
}

// ListDevices lists paired displays by creation time; an empty dashboardID lists all. The token
// hash is never selected.
func (s *Store) ListDevices(ctx context.Context, dashboardID string) ([]KioskDevice, error) {
	query := `SELECT id, dashboard_id, label, view_only, created_at, last_seen_at, last_ip, expires_at FROM hd_kiosk_devices`
	var args []any
	if dashboardID != "" {
		query += ` WHERE dashboard_id = ?`
		args = append(args, dashboardID)
	}
	query += ` ORDER BY created_at, id`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []KioskDevice{}
	for rows.Next() {
		var d KioskDevice
		var viewOnly int
		var created string
		var seen, ip, expires sql.NullString
		if err := rows.Scan(&d.ID, &d.DashboardID, &d.Label, &viewOnly, &created, &seen, &ip, &expires); err != nil {
			return nil, err
		}
		d.ViewOnly, d.CreatedAt, d.LastIP = viewOnly == 1, parseTS(created), ip.String
		if seen.Valid {
			t := parseTS(seen.String)
			d.LastSeenAt = &t
		}
		if expires.Valid {
			t := parseTS(expires.String)
			d.ExpiresAt = &t
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// RevokeDevice deletes a paired display; its token stops working at once.
func (s *Store) RevokeDevice(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM hd_kiosk_devices WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return newError(CodeKioskUnpaired, "this display is not paired")
	}
	return nil
}
