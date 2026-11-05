package flows

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"aurago/internal/security"
)

var (
	ErrLibraryInvalid        = errors.New("invalid library data")
	ErrLibraryConflict       = errors.New("library data changed elsewhere")
	ErrLibraryFolderNotEmpty = errors.New("the folder contains flows")
	ErrVersionNoteConflict   = errors.New("the version note changed elsewhere")
	ErrLibrarySensitive      = errors.New("library data contains a secret or redacted value")
)

const maxLibraryFolders = 1000

type LibraryFolder struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Revision  int    `json:"revision"`
	FlowCount int    `json:"flow_count"`
	Redacted  bool   `json:"redacted"`
	Editable  bool   `json:"editable"`
}

type LibraryMetadata struct {
	FolderID     *string  `json:"folder_id"`
	Tags         []string `json:"tags"`
	Revision     int      `json:"revision"`
	TagsRedacted bool     `json:"tags_redacted"`
	TagsEditable bool     `json:"tags_editable"`
}

type LibraryUpdate struct {
	BaseRevision int
	SetFolder    bool
	FolderID     *string
	SetTags      bool
	Tags         []string
}

func libraryText(value string, maxRunes, maxBytes int, multiline bool) error {
	if !utf8.ValidString(value) || len(value) > maxBytes || utf8.RuneCountInString(value) > maxRunes {
		return ErrLibraryInvalid
	}
	for _, r := range value {
		if unicode.IsControl(r) && !(multiline && (r == '\n' || r == '\r' || r == '\t')) {
			return ErrLibraryInvalid
		}
	}
	if security.ScrubSecrets(value) != value || strings.Contains(value, redactedText) {
		return ErrLibrarySensitive
	}
	return nil
}

func libraryName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ErrLibraryInvalid
	}
	return name, libraryText(name, 80, 320, false)
}

func libraryTags(tags []string) ([]string, error) {
	if tags == nil || len(tags) > 16 {
		return nil, ErrLibraryInvalid
	}
	out := make([]string, 0, len(tags))
	seen := make(map[string]bool, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" || seen[tag] {
			return nil, ErrLibraryInvalid
		}
		if err := libraryText(tag, 40, 160, false); err != nil {
			return nil, err
		}
		seen[tag] = true
		out = append(out, tag)
	}
	sort.Strings(out)
	return out, nil
}

func visibleLibraryText(raw string) (string, bool) {
	visible := security.ScrubSecrets(raw)
	return visible, visible != raw
}

func safeFolder(folder LibraryFolder) LibraryFolder {
	var redacted bool
	folder.Name, redacted = visibleLibraryText(folder.Name)
	folder.Redacted = folder.Redacted || redacted
	folder.Editable = !folder.Redacted
	return folder
}

func (f *LibraryFolder) RefreshPrivacy() { *f = safeFolder(*f) }

func safeLibrary(meta LibraryMetadata) LibraryMetadata {
	if meta.Tags == nil {
		meta.Tags = []string{}
	}
	for i, tag := range meta.Tags {
		var redacted bool
		meta.Tags[i], redacted = visibleLibraryText(tag)
		meta.TagsRedacted = meta.TagsRedacted || redacted
	}
	meta.TagsEditable = !meta.TagsRedacted
	return meta
}

func (m *LibraryMetadata) RefreshPrivacy() { *m = safeLibrary(*m) }

func libraryWrite(ctx context.Context, db *sql.DB) (*sql.Tx, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE flows_meta SET value=value WHERE key='schema_version'`); err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	return tx, nil
}

func readLibrary(row scanner) (LibraryMetadata, error) {
	var folder sql.NullString
	var raw string
	var revision int
	if err := row.Scan(&folder, &raw, &revision); err != nil {
		return LibraryMetadata{}, err
	}
	return decodeLibrary(folder, raw, revision)
}

func decodeLibrary(folder sql.NullString, raw string, revision int) (LibraryMetadata, error) {
	m := LibraryMetadata{Revision: revision}
	if folder.Valid {
		m.FolderID = &folder.String
	}
	if revision <= 0 || len(raw) > 4096 || json.Unmarshal([]byte(raw), &m.Tags) != nil || m.Tags == nil || len(m.Tags) > 16 {
		return m, ErrLibraryInvalid
	}
	for _, tag := range m.Tags {
		if !utf8.ValidString(tag) || len(tag) > 160 || utf8.RuneCountInString(tag) > 40 {
			return m, ErrLibraryInvalid
		}
	}
	return m, nil
}

func (s *Service) ListLibraryFolders(ctx context.Context) ([]LibraryFolder, error) {
	rows, err := s.store.db.QueryContext(ctx, `SELECT f.id,CASE WHEN length(CAST(f.name AS BLOB))<=320 THEN f.name ELSE NULL END,f.revision,COUNT(m.flow_id) FROM flow_library_folders f LEFT JOIN flow_library_metadata m ON m.folder_id=f.id GROUP BY f.id ORDER BY f.name,f.id LIMIT ?`, maxLibraryFolders+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LibraryFolder{}
	for rows.Next() {
		var f LibraryFolder
		if err := rows.Scan(&f.ID, &f.Name, &f.Revision, &f.FlowCount); err != nil {
			return nil, err
		}
		if len(out) >= maxLibraryFolders || !blockFlowID.MatchString(f.ID) || len(f.Name) > 320 || !utf8.ValidString(f.Name) || utf8.RuneCountInString(f.Name) > 80 {
			return nil, ErrLibraryInvalid
		}
		out = append(out, safeFolder(f))
	}
	return out, rows.Err()
}

func (s *Service) CreateLibraryFolder(ctx context.Context, name string) (*LibraryFolder, error) {
	name, err := libraryName(name)
	if err != nil {
		return nil, err
	}
	tx, err := libraryWrite(ctx, s.store.db)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM flow_library_folders`).Scan(&count); err != nil {
		return nil, err
	}
	if count >= maxLibraryFolders {
		return nil, ErrLibraryInvalid
	}
	f := LibraryFolder{ID: "folder_" + randomSuffix(10), Name: name, Revision: 1}
	res, err := tx.ExecContext(ctx, `INSERT INTO flow_library_folders(id,name,revision,updated_at) VALUES(?,?,1,?) ON CONFLICT DO NOTHING`, f.ID, name, formatTime(s.now()))
	if err != nil {
		return nil, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n != 1 {
		return nil, ErrLibraryConflict
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	f = safeFolder(f)
	return &f, nil
}

func (s *Service) UpdateLibraryFolder(ctx context.Context, id, name string, baseRevision int) (*LibraryFolder, error) {
	if !blockFlowID.MatchString(id) || baseRevision <= 0 {
		return nil, ErrLibraryInvalid
	}
	name, err := libraryName(name)
	if err != nil {
		return nil, err
	}
	tx, err := libraryWrite(ctx, s.store.db)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var f LibraryFolder
	err = tx.QueryRowContext(ctx, `SELECT id,CASE WHEN length(CAST(name AS BLOB))<=320 THEN name ELSE NULL END,revision,(SELECT COUNT(*) FROM flow_library_metadata WHERE folder_id=?) FROM flow_library_folders WHERE id=?`, id, id).Scan(&f.ID, &f.Name, &f.Revision, &f.FlowCount)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if f.Revision != baseRevision {
		return nil, ErrLibraryConflict
	}
	res, err := tx.ExecContext(ctx, `UPDATE OR IGNORE flow_library_folders SET name=?,revision=revision+1,updated_at=? WHERE id=? AND revision=?`, name, formatTime(s.now()), id, baseRevision)
	if err != nil {
		return nil, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n != 1 {
		return nil, ErrLibraryConflict
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	f.Name = name
	f.Revision++
	f = safeFolder(f)
	return &f, nil
}

func (s *Service) DeleteLibraryFolder(ctx context.Context, id string, baseRevision int) error {
	if !blockFlowID.MatchString(id) || baseRevision <= 0 {
		return ErrLibraryInvalid
	}
	tx, err := libraryWrite(ctx, s.store.db)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var revision, count int
	err = tx.QueryRowContext(ctx, `SELECT revision,(SELECT COUNT(*) FROM flow_library_metadata WHERE folder_id=?) FROM flow_library_folders WHERE id=?`, id, id).Scan(&revision, &count)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if revision != baseRevision {
		return ErrLibraryConflict
	}
	if count != 0 {
		return ErrLibraryFolderNotEmpty
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM flow_library_folders WHERE id=? AND revision=?`, id, baseRevision); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) GetFlowLibrary(ctx context.Context, id string) (*LibraryMetadata, error) {
	tx, err := s.store.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rec, err := scanFlow(tx.QueryRowContext(ctx, `SELECT `+flowColumns+` FROM flows WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if rec.Kind != KindFlow {
		return nil, ErrWrongFlowKind
	}
	m, err := readLibrary(tx.QueryRowContext(ctx, `SELECT folder_id,CASE WHEN length(CAST(tags_json AS BLOB))<=4096 THEN tags_json ELSE NULL END,revision FROM flow_library_metadata WHERE flow_id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		m = LibraryMetadata{Tags: []string{}}
	} else if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	m = safeLibrary(m)
	return &m, nil
}

// listFlowsWithLibrary binds both sets to one read snapshot, without one query
// per flow. Existing unreadable documents are skipped as in Store.ListFlows.
func (s *Store) listFlowsWithLibrary(ctx context.Context) ([]*FlowRecord, map[string]LibraryMetadata, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT `+flowColumns+` FROM flows WHERE kind=? ORDER BY updated_at DESC,id`, string(KindFlow))
	if err != nil {
		return nil, nil, err
	}
	var records []*FlowRecord
	for rows.Next() {
		rec, e := scanFlow(rows)
		if e != nil {
			s.logger.Warn("skipping unreadable flow", "error", e)
			continue
		}
		records = append(records, rec)
	}
	err = rows.Err()
	closeErr := rows.Close()
	if err != nil {
		return nil, nil, err
	}
	if closeErr != nil {
		return nil, nil, closeErr
	}
	rows, err = tx.QueryContext(ctx, `SELECT m.flow_id,m.folder_id,CASE WHEN length(CAST(m.tags_json AS BLOB))<=4096 THEN m.tags_json ELSE NULL END,m.revision FROM flow_library_metadata m JOIN flows f ON f.id=m.flow_id WHERE f.kind='flow'`)
	if err != nil {
		return nil, nil, err
	}
	metadata := make(map[string]LibraryMetadata)
	for rows.Next() {
		var id string
		var folder sql.NullString
		var raw sql.NullString
		var revision int
		if err := rows.Scan(&id, &folder, &raw, &revision); err != nil {
			rows.Close()
			return nil, nil, err
		}
		if !raw.Valid {
			rows.Close()
			return nil, nil, ErrLibraryInvalid
		}
		m, e := decodeLibrary(folder, raw.String, revision)
		if e != nil {
			rows.Close()
			return nil, nil, e
		}
		metadata[id] = safeLibrary(m)
	}
	err = rows.Err()
	closeErr = rows.Close()
	if err != nil {
		return nil, nil, err
	}
	if closeErr != nil {
		return nil, nil, closeErr
	}
	if err = tx.Commit(); err != nil {
		return nil, nil, err
	}
	return records, metadata, nil
}

func (s *Service) UpdateFlowLibrary(ctx context.Context, id string, update LibraryUpdate) (*LibraryMetadata, error) {
	if update.BaseRevision < 0 || !update.SetFolder && !update.SetTags {
		return nil, ErrLibraryInvalid
	}
	var tags []string
	var err error
	if update.SetTags {
		tags, err = libraryTags(update.Tags)
		if err != nil {
			return nil, err
		}
	}
	if update.SetFolder && update.FolderID != nil && !blockFlowID.MatchString(*update.FolderID) {
		return nil, ErrLibraryInvalid
	}
	tx, err := libraryWrite(ctx, s.store.db)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rec, err := scanFlow(tx.QueryRowContext(ctx, `SELECT `+flowColumns+` FROM flows WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if rec.Kind != KindFlow {
		return nil, ErrWrongFlowKind
	}
	m, err := readLibrary(tx.QueryRowContext(ctx, `SELECT folder_id,CASE WHEN length(CAST(tags_json AS BLOB))<=4096 THEN tags_json ELSE NULL END,revision FROM flow_library_metadata WHERE flow_id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		m = LibraryMetadata{Tags: []string{}}
	} else if err != nil {
		return nil, err
	}
	if m.Revision != update.BaseRevision {
		return nil, ErrLibraryConflict
	}
	if update.SetFolder {
		m.FolderID = update.FolderID
		if m.FolderID != nil {
			var exists int
			err = tx.QueryRowContext(ctx, `SELECT 1 FROM flow_library_folders WHERE id=?`, *m.FolderID).Scan(&exists)
			if errors.Is(err, sql.ErrNoRows) {
				return nil, ErrNotFound
			}
			if err != nil {
				return nil, err
			}
		}
	}
	if update.SetTags {
		m.Tags = tags
	}
	encoded, err := json.Marshal(m.Tags)
	if err != nil {
		return nil, fmt.Errorf("encode library tags: %w", err)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO flow_library_metadata(flow_id,folder_id,tags_json,revision) VALUES(?,?,?,1) ON CONFLICT(flow_id) DO UPDATE SET folder_id=excluded.folder_id,tags_json=excluded.tags_json,revision=flow_library_metadata.revision+1`, id, m.FolderID, string(encoded))
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	m.Revision++
	m = safeLibrary(m)
	return &m, nil
}
