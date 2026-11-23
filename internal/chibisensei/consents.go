package chibisensei

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Consent is one accepted base-model license version.
type Consent struct {
	ProfileID      string `json:"profile_id"`
	LicenseID      string `json:"license_id"`
	LicenseVersion string `json:"license_version"`
	AcceptedAt     string `json:"accepted_at"`
}

// RecordConsent records the consent once; a repeat returns the original row (idempotent).
func (s *Store) RecordConsent(ctx context.Context, profileID, licenseID, version string) (Consent, error) {
	if profileID == "" || licenseID == "" || version == "" {
		return Consent{}, errors.New("record consent: profile, license and version are required")
	}
	if _, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO consents(profile_id,license_id,license_version,accepted_at)
		VALUES(?,?,?,?)`, profileID, licenseID, version, s.stamp()); err != nil {
		return Consent{}, fmt.Errorf("record consent: %w", err)
	}
	c := Consent{ProfileID: profileID, LicenseID: licenseID, LicenseVersion: version}
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE(accepted_at,'') FROM consents WHERE profile_id=? AND license_id=?
		AND license_version=?`, profileID, licenseID, version).Scan(&c.AcceptedAt)
	return c, err
}

// HasConsent reports whether the exact license version was accepted for the profile.
func (s *Store) HasConsent(ctx context.Context, profileID, licenseID, version string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM consents WHERE profile_id=? AND license_id=? AND license_version=?`,
		profileID, licenseID, version).Scan(&n)
	return n > 0, err
}

// ListConsents lists every consent, oldest first.
func (s *Store) ListConsents(ctx context.Context) ([]Consent, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT profile_id, license_id, license_version, COALESCE(accepted_at,'') FROM consents
		ORDER BY accepted_at, profile_id, license_id, license_version`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Consent{}
	for rows.Next() {
		var c Consent
		if err := rows.Scan(&c.ProfileID, &c.LicenseID, &c.LicenseVersion, &c.AcceptedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetSetting returns a settings value and whether the key exists.
func (s *Store) GetSetting(ctx context.Context, key string) (string, bool, error) {
	var v string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key=?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return v, true, nil
}

// SetSetting stores a settings value.
func (s *Store) SetSetting(ctx context.Context, key, value string) error {
	if key == "" {
		return errors.New("set setting: empty key")
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO settings(key,value) VALUES(?,?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}
