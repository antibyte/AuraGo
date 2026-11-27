package homedash

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	SavedByUser  = "user"
	SavedByAgent = "agent"
	MaxRevisions = 20
)

// DashboardSummary is one row of the dashboard list.
type DashboardSummary struct {
	ID               string    `json:"id"`
	Slug             string    `json:"slug"`
	Name             string    `json:"name"`
	Icon             string    `json:"icon,omitempty"`
	Revision         int64     `json:"revision"`
	ApprovedRevision int64     `json:"approved_revision"`
	UpdatedAt        time.Time `json:"updated_at"`
	SavedBy          string    `json:"saved_by"`
}

// RevisionInfo is one entry of a dashboard's revision history.
type RevisionInfo struct {
	Revision int64     `json:"revision"`
	SavedAt  time.Time `json:"saved_at"`
	SavedBy  string    `json:"saved_by"`
}

func encodeDocument(d Dashboard) (string, error) {
	raw, err := json.Marshal(d)
	if err != nil {
		return "", err
	}
	if len(raw) > Catalog().Limits.DocumentBytes {
		return "", newError(CodeLimitExceeded, "the document is larger than 256 KiB")
	}
	return string(raw), nil
}

func slugError(err error) error {
	if err != nil && strings.Contains(err.Error(), "hd_dashboards.slug") {
		return newError(CodeSlugTaken, "another dashboard uses this slug")
	}
	return err
}

func validSavedBy(by string) error {
	if by != SavedByUser && by != SavedByAgent {
		return newError(CodeBadRequest, "saved_by must be user or agent")
	}
	return nil
}

// CreateDashboard stores doc as revision 1 under a new id. A user save approves the revision;
// an agent save leaves approved_revision at 0.
func (s *Store) CreateDashboard(ctx context.Context, doc Dashboard, by string) (Dashboard, error) {
	if err := validSavedBy(by); err != nil {
		return Dashboard{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Dashboard{}, err
	}
	defer tx.Rollback()
	var n int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM hd_dashboards`).Scan(&n); err != nil {
		return Dashboard{}, err
	}
	if int64(n) >= s.maxDashboards.Load() {
		return Dashboard{}, newError(CodeLimitExceeded, "too many dashboards")
	}
	now := s.now()
	var approved int64
	if by == SavedByUser {
		approved = 1
	}
	doc.ID, doc.Revision, doc.ApprovedRevision = NewDashboardID(), 1, approved
	doc.SchemaVersion, doc.UpdatedAt, doc.SavedBy = SchemaVersion, now, by
	raw, err := encodeDocument(doc)
	if err != nil {
		return Dashboard{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO hd_dashboards (id, slug, name, schema_version, revision, approved_revision,
		document_json, saved_by, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		doc.ID, doc.Slug, doc.Name, doc.SchemaVersion, doc.Revision, approved, raw, by, ts(now), ts(now)); err != nil {
		return Dashboard{}, slugError(err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO hd_dashboard_revisions (dashboard_id, revision, document_json, saved_by, saved_at)
		VALUES (?, ?, ?, ?, ?)`, doc.ID, doc.Revision, raw, by, ts(now)); err != nil {
		return Dashboard{}, err
	}
	if err := tx.Commit(); err != nil {
		return Dashboard{}, err
	}
	return doc, nil
}

// UpdateDashboard saves doc as the next revision when revision is still current. A user save
// approves the new revision; an agent save never moves approved_revision.
func (s *Store) UpdateDashboard(ctx context.Context, doc Dashboard, revision int64, by string) (Dashboard, error) {
	if err := validSavedBy(by); err != nil {
		return Dashboard{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Dashboard{}, err
	}
	defer tx.Rollback()
	var cur, approved int64
	var storedSchema int
	err = tx.QueryRowContext(ctx, `SELECT revision, approved_revision, schema_version FROM hd_dashboards WHERE id = ?`, doc.ID).Scan(&cur, &approved, &storedSchema)
	if errors.Is(err, sql.ErrNoRows) {
		return Dashboard{}, ErrDashboardNotFound
	} else if err != nil {
		return Dashboard{}, err
	}
	if storedSchema > SchemaVersion {
		// Written by a newer build: read-only here, so no field it added is dropped.
		return Dashboard{}, &Error{Code: CodeSchemaTooNew, Message: fmt.Sprintf("dashboard schema %d is newer than supported %d", storedSchema, SchemaVersion)}
	}
	if cur != revision {
		return Dashboard{}, &Error{Code: CodeRevisionConflict, Message: "the dashboard was changed elsewhere", CurrentRevision: cur}
	}
	next, now := cur+1, s.now()
	if by == SavedByUser {
		approved = next // a user save approves; an agent save never moves what displays show
	}
	doc.SchemaVersion, doc.Revision, doc.ApprovedRevision, doc.UpdatedAt, doc.SavedBy = SchemaVersion, next, approved, now, by
	raw, err := encodeDocument(doc)
	if err != nil {
		return Dashboard{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE hd_dashboards SET slug = ?, name = ?, schema_version = ?, revision = ?, approved_revision = ?,
		document_json = ?, saved_by = ?, updated_at = ? WHERE id = ?`, doc.Slug, doc.Name, doc.SchemaVersion, next, approved, raw, by, ts(now), doc.ID); err != nil {
		return Dashboard{}, slugError(err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO hd_dashboard_revisions (dashboard_id, revision, document_json, saved_by, saved_at)
		VALUES (?, ?, ?, ?, ?)`, doc.ID, next, raw, by, ts(now)); err != nil {
		return Dashboard{}, err
	}
	// Keep the last MaxRevisions and always the approved one (kiosk displays read it).
	if _, err := tx.ExecContext(ctx, `DELETE FROM hd_dashboard_revisions WHERE dashboard_id = ? AND revision <= ? AND revision <> ?`,
		doc.ID, next-MaxRevisions, approved); err != nil {
		return Dashboard{}, err
	}
	if err := tx.Commit(); err != nil {
		return Dashboard{}, err
	}
	return doc, nil
}

// Approve makes an existing revision the one kiosk displays show.
func (s *Store) Approve(ctx context.Context, id string, revision int64) (Dashboard, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Dashboard{}, err
	}
	defer tx.Rollback()
	var n int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM hd_dashboards WHERE id = ?`, id).Scan(&n); err != nil {
		return Dashboard{}, err
	} else if n == 0 {
		return Dashboard{}, ErrDashboardNotFound
	}
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM hd_dashboard_revisions WHERE dashboard_id = ? AND revision = ?`, id, revision).Scan(&n); err != nil {
		return Dashboard{}, err
	} else if n == 0 {
		return Dashboard{}, newError(CodeRevisionNotFound, "no such revision")
	}
	if _, err := tx.ExecContext(ctx, `UPDATE hd_dashboards SET approved_revision = ? WHERE id = ?`, revision, id); err != nil {
		return Dashboard{}, err
	}
	if err := tx.Commit(); err != nil {
		return Dashboard{}, err
	}
	return s.GetDashboard(ctx, id)
}

// GetDashboard returns the current document. A document from a newer schema is returned together
// with the schema_too_new error so that callers can show it read-only.
func (s *Store) GetDashboard(ctx context.Context, id string) (Dashboard, error) {
	var raw, savedBy, updated string
	var revision, approved int64
	err := s.db.QueryRowContext(ctx, `SELECT document_json, revision, approved_revision, saved_by, updated_at FROM hd_dashboards WHERE id = ?`, id).
		Scan(&raw, &revision, &approved, &savedBy, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return Dashboard{}, ErrDashboardNotFound
	} else if err != nil {
		return Dashboard{}, err
	}
	var doc Dashboard
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return Dashboard{}, fmt.Errorf("decode stored dashboard: %w", err)
	}
	doc.ID, doc.Revision, doc.ApprovedRevision, doc.SavedBy, doc.UpdatedAt = id, revision, approved, savedBy, parseTS(updated)
	if err := Migrate(&doc); err != nil {
		return doc, err
	}
	return doc, nil
}

// ListDashboards returns one summary per dashboard, ordered by name then id.
func (s *Store) ListDashboards(ctx context.Context) ([]DashboardSummary, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, slug, name, revision, approved_revision, saved_by, updated_at, document_json
		FROM hd_dashboards ORDER BY name, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DashboardSummary{}
	for rows.Next() {
		var sum DashboardSummary
		var updated, raw string
		if err := rows.Scan(&sum.ID, &sum.Slug, &sum.Name, &sum.Revision, &sum.ApprovedRevision, &sum.SavedBy, &updated, &raw); err != nil {
			return nil, err
		}
		sum.UpdatedAt = parseTS(updated)
		var icon struct {
			Icon string `json:"icon"`
		}
		if json.Unmarshal([]byte(raw), &icon) == nil {
			sum.Icon = icon.Icon
		}
		out = append(out, sum)
	}
	return out, rows.Err()
}

// DashboardNames returns id -> name for the dashboards that exist among ids. It reads only the
// name column (no document parsing); unknown ids are absent from the map.
func (s *Store) DashboardNames(ctx context.Context, ids []string) (map[string]string, error) {
	out := make(map[string]string, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	marks := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, name FROM hd_dashboards WHERE id IN (`+marks+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		out[id] = name
	}
	return out, rows.Err()
}

// Revisions lists the kept revisions of a dashboard, newest first.
func (s *Store) Revisions(ctx context.Context, id string) ([]RevisionInfo, error) {
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM hd_dashboards WHERE id = ?`, id).Scan(&n); err != nil {
		return nil, err
	} else if n == 0 {
		return nil, ErrDashboardNotFound
	}
	rows, err := s.db.QueryContext(ctx, `SELECT revision, saved_at, saved_by FROM hd_dashboard_revisions WHERE dashboard_id = ? ORDER BY revision DESC`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RevisionInfo{}
	for rows.Next() {
		var r RevisionInfo
		var at string
		if err := rows.Scan(&r.Revision, &at, &r.SavedBy); err != nil {
			return nil, err
		}
		r.SavedAt = parseTS(at)
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetRevision returns a stored revision with the dashboard's current approved_revision.
func (s *Store) GetRevision(ctx context.Context, id string, rev int64) (Dashboard, error) {
	var raw string
	var approved int64
	err := s.db.QueryRowContext(ctx, `SELECT r.document_json, d.approved_revision FROM hd_dashboard_revisions r
		JOIN hd_dashboards d ON d.id = r.dashboard_id WHERE r.dashboard_id = ? AND r.revision = ?`, id, rev).Scan(&raw, &approved)
	if errors.Is(err, sql.ErrNoRows) {
		return Dashboard{}, newError(CodeRevisionNotFound, "no such revision")
	} else if err != nil {
		return Dashboard{}, err
	}
	var doc Dashboard
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return Dashboard{}, fmt.Errorf("decode stored revision: %w", err)
	}
	doc.ID, doc.Revision, doc.ApprovedRevision = id, rev, approved
	if err := Migrate(&doc); err != nil {
		return doc, err
	}
	return doc, nil
}

// GetApproved returns the revision kiosk displays show.
func (s *Store) GetApproved(ctx context.Context, id string) (Dashboard, error) {
	var approved int64
	err := s.db.QueryRowContext(ctx, `SELECT approved_revision FROM hd_dashboards WHERE id = ?`, id).Scan(&approved)
	if errors.Is(err, sql.ErrNoRows) {
		return Dashboard{}, ErrDashboardNotFound
	} else if err != nil {
		return Dashboard{}, err
	}
	if approved == 0 {
		return Dashboard{}, &Error{Code: CodeDashboardNotFound, Message: "no approved revision"}
	}
	return s.GetRevision(ctx, id, approved)
}

// DeleteDashboard removes a dashboard whose revision is current and returns the ids of the kiosk
// devices that went with it, so the caller can end their streams.
func (s *Store) DeleteDashboard(ctx context.Context, id string, revision int64) ([]string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var cur int64
	err = tx.QueryRowContext(ctx, `SELECT revision FROM hd_dashboards WHERE id = ?`, id).Scan(&cur)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrDashboardNotFound
	} else if err != nil {
		return nil, err
	}
	if cur != revision {
		return nil, &Error{Code: CodeRevisionConflict, Message: "the dashboard was changed elsewhere", CurrentRevision: cur}
	}
	rows, err := tx.QueryContext(ctx, `SELECT id FROM hd_kiosk_devices WHERE dashboard_id = ? ORDER BY id`, id)
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for rows.Next() {
		var dev string
		if err := rows.Scan(&dev); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, dev)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	if _, err := tx.ExecContext(ctx, `DELETE FROM hd_dashboards WHERE id = ?`, id); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return ids, nil
}
