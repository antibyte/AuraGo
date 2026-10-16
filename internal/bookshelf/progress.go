package bookshelf

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
)

const maxLocatorBytes = 16 << 10

func (s *Service) SaveProgress(ctx context.Context, bookID string, expectedRevision int64, renditionRevision string, locator json.RawMessage, fraction float64) (Progress, error) {
	if err := s.checkOpen(); err != nil {
		return Progress{}, err
	}
	if !validID(bookID) || !validLocator(locator) || !validateText(renditionRevision, 1024) || math.IsNaN(fraction) || math.IsInf(fraction, 0) || fraction < 0 || fraction > 1 {
		return Progress{}, fmt.Errorf("invalid bookshelf progress")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Progress{}, err
	}
	defer tx.Rollback()
	var contentRevision, readingState string
	var readingStateManual int
	if err := tx.QueryRowContext(ctx, `SELECT content_revision,reading_state,reading_state_manual FROM books WHERE id=?`, bookID).Scan(&contentRevision, &readingState, &readingStateManual); errors.Is(err, sql.ErrNoRows) {
		return Progress{}, ErrNotFound
	} else if err != nil {
		return Progress{}, fmt.Errorf("read bookshelf rendition revision: %w", err)
	}
	valid, err := validRenditionRevision(ctx, tx, bookID, contentRevision, renditionRevision)
	if err != nil {
		return Progress{}, fmt.Errorf("validate bookshelf progress rendition: %w", err)
	}
	if !valid {
		return Progress{}, ErrStaleRendition
	}
	var current int64
	err = tx.QueryRowContext(ctx, `SELECT revision FROM progress WHERE book_id=?`, bookID).Scan(&current)
	if errors.Is(err, sql.ErrNoRows) {
		current = 0
	} else if err != nil {
		return Progress{}, fmt.Errorf("read bookshelf progress revision: %w", err)
	}
	if current != expectedRevision {
		return Progress{}, ErrConflict
	}
	next := current + 1
	updated := nowText()
	if _, err := tx.ExecContext(ctx, `INSERT INTO progress(book_id,revision,rendition_revision,locator,fraction,updated_at) VALUES(?,?,?,?,?,?)
		ON CONFLICT(book_id) DO UPDATE SET revision=excluded.revision,rendition_revision=excluded.rendition_revision,locator=excluded.locator,fraction=excluded.fraction,updated_at=excluded.updated_at`,
		bookID, next, renditionRevision, string(locator), fraction, updated); err != nil {
		return Progress{}, fmt.Errorf("save bookshelf progress: %w", err)
	}
	if readingStateManual == 0 {
		autoState := "unread"
		if fraction >= 1 {
			autoState = "finished"
		} else if fraction > 0 {
			autoState = "reading"
		}
		if autoState != readingState {
			if _, err := tx.ExecContext(ctx, `UPDATE books SET reading_state=?,revision=revision+1 WHERE id=? AND reading_state_manual=0`, autoState, bookID); err != nil {
				return Progress{}, fmt.Errorf("update derived bookshelf reading state: %w", err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return Progress{}, fmt.Errorf("commit bookshelf progress: %w", err)
	}
	return Progress{Revision: next, RenditionRevision: renditionRevision, Locator: append(json.RawMessage(nil), locator...), Fraction: fraction, UpdatedAt: parseTime(updated)}, nil
}

func validRenditionRevision(ctx context.Context, db interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, bookID, contentRevision, renditionRevision string) (bool, error) {
	if renditionRevision == "" {
		return false, nil
	}
	if contentRevision != "" && contentRevision == renditionRevision {
		return true, nil
	}
	if contentRevision == "" {
		return false, nil
	}
	var exists int
	err := db.QueryRowContext(ctx, `SELECT 1 FROM renditions WHERE book_id=? AND base_revision=? AND revision=? LIMIT 1`, bookID, contentRevision, renditionRevision).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (s *Service) ListAnnotations(ctx context.Context, bookID string) ([]Annotation, error) {
	if err := s.checkOpen(); err != nil {
		return nil, err
	}
	if !validID(bookID) {
		return nil, ErrNotFound
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,book_id,kind,locator,text,color,rendition_revision,revision,created_at,updated_at FROM annotations WHERE book_id=? ORDER BY created_at,id`, bookID)
	if err != nil {
		return nil, fmt.Errorf("list bookshelf annotations: %w", err)
	}
	defer rows.Close()
	var annotations []Annotation
	for rows.Next() {
		var a Annotation
		var locator, created, updated string
		if err := rows.Scan(&a.ID, &a.BookID, &a.Kind, &locator, &a.Text, &a.Color, &a.RenditionRevision, &a.Revision, &created, &updated); err != nil {
			return nil, err
		}
		a.Locator = json.RawMessage(locator)
		a.CreatedAt = parseTime(created)
		a.UpdatedAt = parseTime(updated)
		annotations = append(annotations, a)
	}
	return annotations, rows.Err()
}

func (s *Service) CreateAnnotation(ctx context.Context, bookID, kind string, locator json.RawMessage, text, color string) (Annotation, error) {
	book, err := s.GetBook(ctx, bookID)
	if err != nil {
		return Annotation{}, err
	}
	return s.CreateAnnotationForRendition(ctx, bookID, kind, book.ContentRevision, locator, text, color)
}

// CreateAnnotationForRendition retains the exact reader snapshot alongside the
// locator so stale highlights are never silently drawn on a different book.
func (s *Service) CreateAnnotationForRendition(ctx context.Context, bookID, kind, renditionRevision string, locator json.RawMessage, text, color string) (Annotation, error) {
	if err := s.checkOpen(); err != nil {
		return Annotation{}, err
	}
	kind = strings.ToLower(strings.TrimSpace(kind))
	text = strings.TrimSpace(text)
	color = strings.TrimSpace(color)
	if !validID(bookID) || !validAnnotationKind(kind) || !validLocator(locator) || !validateText(text, 32_768) || !validColor(color) || !validateText(renditionRevision, 1024) {
		return Annotation{}, fmt.Errorf("invalid bookshelf annotation")
	}
	if kind != "bookmark" && text == "" {
		return Annotation{}, fmt.Errorf("annotation text is required")
	}
	id, err := newID()
	if err != nil {
		return Annotation{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Annotation{}, err
	}
	defer tx.Rollback()
	var contentRevision string
	if err := tx.QueryRowContext(ctx, `SELECT content_revision FROM books WHERE id=?`, bookID).Scan(&contentRevision); errors.Is(err, sql.ErrNoRows) {
		return Annotation{}, ErrNotFound
	} else if err != nil {
		return Annotation{}, fmt.Errorf("read annotation source revision: %w", err)
	}
	valid, err := validRenditionRevision(ctx, tx, bookID, contentRevision, renditionRevision)
	if err != nil {
		return Annotation{}, err
	}
	if !valid {
		return Annotation{}, ErrStaleRendition
	}
	created := nowText()
	_, err = tx.ExecContext(ctx, `INSERT INTO annotations(id,book_id,kind,locator,text,color,rendition_revision,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?)`, id, bookID, kind, string(locator), text, color, renditionRevision, created, created)
	if err != nil {
		var exists int
		if checkErr := s.db.QueryRowContext(ctx, `SELECT 1 FROM books WHERE id=?`, bookID).Scan(&exists); errors.Is(checkErr, sql.ErrNoRows) {
			return Annotation{}, ErrNotFound
		}
		return Annotation{}, fmt.Errorf("create bookshelf annotation: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Annotation{}, fmt.Errorf("commit bookshelf annotation: %w", err)
	}
	return Annotation{ID: id, BookID: bookID, Kind: kind, Locator: append(json.RawMessage(nil), locator...), Text: text, Color: color, RenditionRevision: renditionRevision, Revision: 1, CreatedAt: parseTime(created), UpdatedAt: parseTime(created)}, nil
}

func (s *Service) UpdateAnnotation(ctx context.Context, id string, expectedRevision int64, locator json.RawMessage, text, color string) (Annotation, error) {
	if err := s.checkOpen(); err != nil {
		return Annotation{}, err
	}
	text = strings.TrimSpace(text)
	color = strings.TrimSpace(color)
	if !validID(id) || !validLocator(locator) || !validateText(text, 32_768) || !validColor(color) {
		return Annotation{}, fmt.Errorf("invalid bookshelf annotation")
	}
	res, err := s.db.ExecContext(ctx, `UPDATE annotations SET locator=?,text=?,color=?,revision=revision+1,updated_at=? WHERE id=? AND revision=?`, string(locator), text, color, nowText(), id, expectedRevision)
	if err != nil {
		return Annotation{}, fmt.Errorf("update bookshelf annotation: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Annotation{}, checkAnnotationRevision(ctx, s.db, id)
	}
	return s.getAnnotation(ctx, id)
}

func (s *Service) DeleteAnnotation(ctx context.Context, id string, expectedRevision int64) error {
	if err := s.checkOpen(); err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM annotations WHERE id=? AND revision=?`, id, expectedRevision)
	if err != nil {
		return fmt.Errorf("delete bookshelf annotation: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return checkAnnotationRevision(ctx, s.db, id)
	}
	return nil
}

func (s *Service) getAnnotation(ctx context.Context, id string) (Annotation, error) {
	var a Annotation
	var locator, created, updated string
	err := s.db.QueryRowContext(ctx, `SELECT id,book_id,kind,locator,text,color,rendition_revision,revision,created_at,updated_at FROM annotations WHERE id=?`, id).
		Scan(&a.ID, &a.BookID, &a.Kind, &locator, &a.Text, &a.Color, &a.RenditionRevision, &a.Revision, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return Annotation{}, ErrNotFound
	}
	if err != nil {
		return Annotation{}, err
	}
	a.Locator = json.RawMessage(locator)
	a.CreatedAt = parseTime(created)
	a.UpdatedAt = parseTime(updated)
	return a, nil
}

func checkAnnotationRevision(ctx context.Context, db interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, id string) error {
	var revision int64
	err := db.QueryRowContext(ctx, `SELECT revision FROM annotations WHERE id=?`, id).Scan(&revision)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	return ErrConflict
}

func validLocator(raw json.RawMessage) bool {
	if len(raw) == 0 || len(raw) > maxLocatorBytes {
		return false
	}
	var value map[string]json.RawMessage
	return json.Unmarshal(raw, &value) == nil && value != nil
}

func validAnnotationKind(kind string) bool {
	return kind == "bookmark" || kind == "highlight" || kind == "note"
}

func validColor(color string) bool {
	if color == "" {
		return true
	}
	switch strings.ToLower(color) {
	case "yellow", "green", "blue", "pink", "purple":
		return true
	default:
		return false
	}
}
