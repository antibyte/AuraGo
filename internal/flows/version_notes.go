package flows

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"unicode/utf8"
)

type VersionNote struct {
	Note     string `json:"note"`
	Revision int    `json:"revision"`
	Redacted bool   `json:"redacted"`
	Editable bool   `json:"editable"`
}

func safeVersionNote(note VersionNote) VersionNote {
	var redacted bool
	note.Note, redacted = visibleLibraryText(note.Note)
	note.Redacted = note.Redacted || redacted
	note.Editable = !note.Redacted
	return note
}

func (n *VersionNote) RefreshPrivacy() { *n = safeVersionNote(*n) }

func readVersionNote(row scanner) (VersionNote, error) {
	var note VersionNote
	if err := row.Scan(&note.Note, &note.Revision); err != nil {
		return note, err
	}
	if len(note.Note) > 4096 || !utf8.ValidString(note.Note) || utf8.RuneCountInString(note.Note) > 2048 || note.Revision <= 0 {
		return VersionNote{}, ErrLibraryInvalid
	}
	return note, nil
}

// validateVersionOwner checks both the stored owner and the immutable version
// inside the same transaction that reads or writes a note.
func validateVersionOwner(ctx context.Context, tx *sql.Tx, id string, kind Kind, version int) error {
	if version <= 0 || (kind != KindFlow && kind != KindBlock) {
		return ErrLibraryInvalid
	}
	rec, err := scanFlow(tx.QueryRowContext(ctx, `SELECT `+flowColumns+` FROM flows WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if rec.Kind != kind || kind == KindBlock && rec.MissionID != "" {
		return ErrWrongFlowKind
	}
	var raw string
	err = tx.QueryRowContext(ctx, `SELECT CASE WHEN length(CAST(json AS BLOB))<=? THEN json ELSE NULL END FROM flow_versions WHERE flow_id=? AND revision=?`, MaxDocumentBytes, id, version).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	doc, err := ParseFlow([]byte(raw))
	if err != nil {
		return err
	}
	if doc.ID != id || doc.Kind != kind {
		return ErrWrongFlowKind
	}
	return nil
}

func (s *Service) GetVersionNote(ctx context.Context, id string, kind Kind, version int) (*VersionNote, error) {
	tx, err := s.store.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = validateVersionOwner(ctx, tx, id, kind, version); err != nil {
		return nil, err
	}
	note, err := readVersionNote(tx.QueryRowContext(ctx, `SELECT CASE WHEN length(CAST(note AS BLOB))<=4096 THEN note ELSE NULL END,note_revision FROM flow_version_notes WHERE flow_id=? AND revision=?`, id, version))
	if errors.Is(err, sql.ErrNoRows) {
		note = VersionNote{}
	} else if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	note = safeVersionNote(note)
	return &note, nil
}

func (s *Service) SetVersionNote(ctx context.Context, id string, kind Kind, version, baseRevision int, text string) (*VersionNote, error) {
	if baseRevision < 0 {
		return nil, ErrLibraryInvalid
	}
	if err := libraryText(text, 2048, 4096, true); err != nil {
		return nil, err
	}
	tx, err := libraryWrite(ctx, s.store.db)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = validateVersionOwner(ctx, tx, id, kind, version); err != nil {
		return nil, err
	}
	old, err := readVersionNote(tx.QueryRowContext(ctx, `SELECT CASE WHEN length(CAST(note AS BLOB))<=4096 THEN note ELSE NULL END,note_revision FROM flow_version_notes WHERE flow_id=? AND revision=?`, id, version))
	if errors.Is(err, sql.ErrNoRows) {
		old = VersionNote{}
	} else if err != nil {
		return nil, err
	}
	if old.Revision != baseRevision {
		return nil, ErrVersionNoteConflict
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO flow_version_notes(flow_id,revision,note,note_revision) VALUES(?,?,?,1) ON CONFLICT(flow_id,revision) DO UPDATE SET note=excluded.note,note_revision=flow_version_notes.note_revision+1`, id, version, text)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	result := safeVersionNote(VersionNote{Note: text, Revision: old.Revision + 1})
	return &result, nil
}

// publishVersionNote is called only within the publication transaction.
func publishVersionNote(ctx context.Context, tx *sql.Tx, id string, version int, note *string, created bool) error {
	if note == nil {
		return nil
	}
	if created {
		if *note == "" {
			return nil
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO flow_version_notes(flow_id,revision,note,note_revision) VALUES(?,?,?,1)`, id, version, *note)
		return err
	}
	old, err := readVersionNote(tx.QueryRowContext(ctx, `SELECT CASE WHEN length(CAST(note AS BLOB))<=4096 THEN note ELSE NULL END,note_revision FROM flow_version_notes WHERE flow_id=? AND revision=?`, id, version))
	if errors.Is(err, sql.ErrNoRows) {
		old = VersionNote{}
	} else if err != nil {
		return err
	}
	if old.Note != *note {
		return ErrVersionNoteConflict
	}
	return nil
}

func (v *FlowVersionSummary) RefreshNotePrivacy() {
	full, redacted := visibleLibraryText(v.noteFull)
	v.noteFull = full
	line, _, _ := strings.Cut(full, "\n")
	line = strings.TrimSuffix(line, "\r")
	v.NotePreview = truncateRunes(line, 120)
	v.NoteRedacted = v.NoteRedacted || redacted
	v.NoteEditable = !v.NoteRedacted
}
