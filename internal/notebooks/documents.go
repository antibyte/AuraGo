package notebooks

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"aurago/internal/kaggle"
)

const MaxNotebookBytes = 16 << 20

var ErrConflict = errors.New("notebook revision changed; reload or save a copy")

type Document struct {
	Content  json.RawMessage `json:"content"`
	Revision string          `json:"revision"`
}
type Documents struct {
	storage *kaggle.Service
	mu      sync.Mutex
}

func NewDocuments(storage *kaggle.Service) *Documents { return &Documents{storage: storage} }
func revision(b []byte) string                        { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func Validate(b []byte) error {
	if len(b) > MaxNotebookBytes {
		return errors.New("notebook exceeds 16 MiB")
	}
	var n struct {
		Format   int               `json:"nbformat"`
		Cells    []json.RawMessage `json:"cells"`
		Metadata json.RawMessage   `json:"metadata"`
	}
	if json.Unmarshal(b, &n) != nil || n.Format != 4 || n.Cells == nil || len(n.Cells) > 10000 {
		return errors.New("expected an nbformat 4 notebook with at most 10000 cells")
	}
	ids := map[string]bool{}
	for _, raw := range n.Cells {
		var c map[string]json.RawMessage
		if json.Unmarshal(raw, &c) != nil {
			return errors.New("invalid notebook cell")
		}
		var id string
		json.Unmarshal(c["id"], &id)
		if id != "" {
			if ids[id] {
				return errors.New("duplicate notebook cell ID")
			}
			ids[id] = true
		}
	}
	return nil
}
func (d *Documents) Load(project, file string) (Document, error) {
	if !strings.HasSuffix(strings.ToLower(file), ".ipynb") {
		return Document{}, errors.New("notebook filename must end with .ipynb")
	}
	f, err := d.storage.OpenFile(project, file)
	if err != nil {
		return Document{}, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, MaxNotebookBytes+1))
	if err != nil {
		return Document{}, err
	}
	if err = Validate(b); err != nil {
		return Document{}, err
	}
	return Document{b, revision(b)}, nil
}
func (d *Documents) Save(ctx context.Context, project, file, expected string, content json.RawMessage) (Document, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.storage.RequireWrite(); err != nil {
		return Document{}, err
	}
	if err := Validate(content); err != nil {
		return Document{}, err
	}
	if expected == "" {
		return Document{}, ErrConflict
	}
	before, err := d.Load(project, file)
	if expected == "*" {
		if !errors.Is(err, os.ErrNotExist) {
			return Document{}, ErrConflict
		}
	} else if err != nil || before.Revision != expected {
		return Document{}, ErrConflict
	}
	_, err = d.storage.WriteFileVersion(ctx, project, file, expected, bytes.NewReader(content), nil)
	if errors.Is(err, kaggle.ErrFileConflict) {
		return Document{}, ErrConflict
	}
	if err != nil {
		return Document{}, err
	}
	return Document{content, revision(content)}, nil
}

func CellSource(doc Document, cellID string) (string, error) {
	var n struct {
		Cells []map[string]json.RawMessage `json:"cells"`
	}
	if json.Unmarshal(doc.Content, &n) != nil {
		return "", errors.New("invalid notebook")
	}
	for i, c := range n.Cells {
		var id, kind string
		json.Unmarshal(c["id"], &id)
		json.Unmarshal(c["cell_type"], &kind)
		if id == "" {
			id = fmt.Sprintf("cell-%d", i)
		}
		if id != cellID {
			continue
		}
		if kind != "code" {
			return "", errors.New("only code cells can execute")
		}
		var source string
		if json.Unmarshal(c["source"], &source) == nil {
			return source, nil
		}
		var lines []string
		if json.Unmarshal(c["source"], &lines) != nil {
			return "", errors.New("invalid cell source")
		}
		return strings.Join(lines, ""), nil
	}
	return "", errors.New("cell no longer exists")
}
