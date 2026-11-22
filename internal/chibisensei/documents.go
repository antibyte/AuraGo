package chibisensei

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"path"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Document is a source text of a project (material for generation).
type Document struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	WorkspacePath string `json:"workspace_path"`
	Chars         int    `json:"chars"`
	Created       string `json:"created"`
	Text          string `json:"-"`
	SHA256        string `json:"-"`
}

const (
	MaxDocuments     = 20
	MaxDocumentChars = 1_000_000
)

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

func hasTextExtension(p string) bool {
	switch strings.ToLower(path.Ext(p)) {
	case ".txt", ".md", ".markdown":
		return true
	}
	return false
}

func docIssue(key, msg string) error {
	return &ValidationError{Issues: []Issue{issue("workspace_path", key, msg)}}
}

// AddDocument stores a TXT or Markdown file as a source document. All refusals are ValidationErrors.
func (s *Store) AddDocument(ctx context.Context, projectID, workspacePath string, data []byte) (Document, error) {
	if !hasTextExtension(workspacePath) {
		return Document{}, docIssue("document.unsupported_file", "Only TXT and Markdown files are supported.")
	}
	data = bytes.TrimPrefix(data, utf8BOM)
	if !utf8.Valid(data) {
		return Document{}, docIssue("document.not_utf8", "The file is not valid UTF-8 text.")
	}
	chars := utf8.RuneCount(data)
	if chars == 0 || len(bytes.TrimSpace(data)) == 0 {
		return Document{}, docIssue("document.empty", "The file is empty.")
	}
	if chars > MaxDocumentChars {
		return Document{}, docIssue("document.document_too_large", "The document is too large.")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Document{}, err
	}
	defer tx.Rollback()
	if _, err := loadProject(ctx, tx, projectID); err != nil {
		return Document{}, err
	}
	var n int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM documents WHERE project_id=?`, projectID).Scan(&n); err != nil {
		return Document{}, err
	}
	if n >= MaxDocuments {
		return Document{}, docIssue("document.too_many_documents", "A project holds at most 20 documents.")
	}
	sum := sha256.Sum256(data)
	d := Document{ID: uuid.NewString(), Name: path.Base(workspacePath), WorkspacePath: workspacePath, Chars: chars,
		Created: s.stamp(), Text: string(data), SHA256: hex.EncodeToString(sum[:])}
	if _, err := tx.ExecContext(ctx, `INSERT INTO documents(id,project_id,name,workspace_path,sha256,text,chars,created) VALUES(?,?,?,?,?,?,?,?)`,
		d.ID, projectID, d.Name, d.WorkspacePath, d.SHA256, d.Text, d.Chars, d.Created); err != nil {
		return Document{}, err
	}
	if err := tx.Commit(); err != nil {
		return Document{}, err
	}
	return d, nil
}

// ListDocuments lists the documents of a project without their text.
func (s *Store) ListDocuments(ctx context.Context, projectID string) ([]Document, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,workspace_path,chars,created FROM documents WHERE project_id=? ORDER BY created, rowid`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Document{}
	for rows.Next() {
		var d Document
		if err := rows.Scan(&d.ID, &d.Name, &d.WorkspacePath, &d.Chars, &d.Created); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// GetDocuments returns the documents with text, in the order of ids; a foreign or unknown id is ErrNotFound.
func (s *Store) GetDocuments(ctx context.Context, projectID string, ids []string) ([]Document, error) {
	out := make([]Document, 0, len(ids))
	for _, id := range ids {
		var d Document
		err := s.db.QueryRowContext(ctx, `SELECT id,name,workspace_path,chars,created,text,sha256 FROM documents WHERE id=? AND project_id=?`,
			id, projectID).Scan(&d.ID, &d.Name, &d.WorkspacePath, &d.Chars, &d.Created, &d.Text, &d.SHA256)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}

// DeleteDocument removes one document.
func (s *Store) DeleteDocument(ctx context.Context, projectID, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM documents WHERE id=? AND project_id=?`, id, projectID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
