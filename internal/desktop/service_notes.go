package desktop

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

const NotesDirectory = "Documents/Notes"
const NotesTrashDirectory = "Trash/Notes"
const MaxNoteBytes = 2 << 20

var ErrNoteConflict = errors.New("note changed; reload or save a copy")
var ErrAgentNoteMutation = errors.New("agents may read, search and create notes, but cannot change, replace, move or delete existing notes")

// NotesPath includes ancestors: deleting Documents must not delete its notes.
func NotesPath(raw string, includeParents bool) bool {
	p := strings.ToLower(filepath.ToSlash(filepath.Clean(strings.ReplaceAll(raw, "\\", "/"))))
	p = strings.TrimPrefix(p, "./")
	for _, root := range []string{"documents/notes", "trash/notes"} {
		if p == root || strings.HasPrefix(p, root+"/") || (includeParents && (p == "." || p == "" || strings.HasPrefix(root, p+"/"))) {
			return true
		}
	}
	return false
}

func (s *Service) guardNoteMutation(path, source string) error {
	if source != SourceUser && NotesPath(s.relativePath(path), true) {
		return ErrAgentNoteMutation
	}
	return nil
}

// Must run under desktopMutationMu, after canonical path/symlink validation.
func (s *Service) guardNoteWrite(path, source string, content []byte) error {
	if source == SourceUser || !NotesPath(s.relativePath(path), true) {
		return nil
	}
	rel := strings.ToLower(s.relativePath(path))
	if !strings.HasPrefix(rel, "documents/notes/") || !strings.EqualFold(filepath.Ext(path), ".md") || len(content) > MaxNoteBytes || !utf8.Valid(content) {
		return ErrAgentNoteMutation
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		return ErrAgentNoteMutation
	}
	return nil
}

type Note struct {
	Path     string    `json:"path"`
	Title    string    `json:"title"`
	Tags     []string  `json:"tags"`
	Snippet  string    `json:"snippet"`
	Modified time.Time `json:"modified"`
	Size     int64     `json:"size"`
	Version  string    `json:"version"`
	Content  string    `json:"content,omitempty"`
}

type NotesQuery struct {
	Query  string
	Folder string
	Tag    string
	Offset int
	Limit  int
	Trash  bool
}

type NotesResult struct {
	Notes   []Note   `json:"notes"`
	Total   int      `json:"total"`
	Folders []string `json:"folders"`
	Skipped int      `json:"skipped"`
}

func NoteVersion(data []byte) string {
	sum := sha256.Sum256(data)
	return `"` + hex.EncodeToString(sum[:]) + `"`
}

func CheckNoteVersion(expected string, create bool) FileWritePrecondition {
	return func(current FileWriteState) error {
		if create && !current.Exists {
			return nil
		}
		if !create && expected != "" && current.Exists && expected == NoteVersion(current.Data) {
			return nil
		}
		return ErrNoteConflict
	}
}

func validNotePath(path string, trash bool) bool {
	p := filepath.ToSlash(filepath.Clean(strings.ReplaceAll(path, "\\", "/")))
	root := NotesDirectory
	if trash {
		root = NotesTrashDirectory
	}
	return strings.HasPrefix(strings.ToLower(p), strings.ToLower(root)+"/") && strings.EqualFold(filepath.Ext(p), ".md")
}

func (s *Service) ReadNote(ctx context.Context, path string) (Note, error) {
	if !validNotePath(path, false) && !validNotePath(path, true) {
		return Note{}, fmt.Errorf("a Markdown note path is required")
	}
	data, entry, err := s.ReadFileBytes(ctx, path)
	if err != nil {
		return Note{}, err
	}
	if len(data) > MaxNoteBytes || !utf8.Valid(data) {
		return Note{}, fmt.Errorf("note is not UTF-8 or exceeds 2 MiB")
	}
	note := noteSummary(data, entry)
	note.Content = string(data)
	return note, nil
}

func noteSummary(data []byte, entry FileEntry) Note {
	text := string(data)
	note := Note{Path: entry.Path, Title: strings.TrimSuffix(entry.Name, filepath.Ext(entry.Name)), Tags: []string{}, Size: entry.Size, Modified: entry.ModTime, Version: NoteVersion(data)}
	body := text
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if len(lines) > 1 && strings.TrimSpace(lines[0]) == "---" {
		for i := 1; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) != "---" {
				continue
			}
			var meta struct {
				Title string      `yaml:"title"`
				Tags  interface{} `yaml:"tags"`
			}
			if yaml.Unmarshal([]byte(strings.Join(lines[1:i], "\n")), &meta) == nil {
				if meta.Title != "" {
					note.Title = meta.Title
				}
				switch tags := meta.Tags.(type) {
				case []interface{}:
					for _, tag := range tags {
						if value, ok := tag.(string); ok {
							note.Tags = append(note.Tags, value)
						}
					}
				case string:
					for _, tag := range strings.Split(tags, ",") {
						if tag = strings.TrimSpace(tag); tag != "" {
							note.Tags = append(note.Tags, tag)
						}
					}
				}
				body = strings.Join(lines[i+1:], "\n")
			}
			break
		}
	}
	if note.Title == strings.TrimSuffix(entry.Name, filepath.Ext(entry.Name)) {
		for _, line := range strings.Split(body, "\n") {
			if strings.HasPrefix(line, "# ") {
				note.Title = strings.TrimSpace(line[2:])
				break
			}
		}
	}
	note.Snippet = stringLimit(strings.Join(strings.Fields(body), " "), 180)
	note.Title = stringLimit(note.Title, 160)
	return note
}

func stringLimit(value string, count int) string {
	runes := []rune(value)
	if len(runes) > count {
		return string(runes[:count])
	}
	return value
}

// SearchNotes reads the actual Markdown corpus, shared by the app and agent.
// ponytail: one cancellable scan; add a derived FTS index if measured corpus latency requires it.
func (s *Service) SearchNotes(ctx context.Context, query NotesQuery) (NotesResult, error) {
	result := NotesResult{Notes: []Note{}, Folders: []string{}}
	if err := s.ensureReady(ctx); err != nil {
		return result, err
	}
	if len(query.Query) > 1024 || query.Offset < 0 || query.Offset > 100000 || query.Limit < 0 || query.Limit > 200 {
		return result, fmt.Errorf("invalid note search bounds")
	}
	if query.Limit == 0 {
		query.Limit = 50
	}
	root := NotesDirectory
	if query.Trash {
		root = NotesTrashDirectory
	}
	abs, err := s.resolveWorkspacePathNoSymlinks(root, true)
	if err != nil {
		return result, err
	}
	words := strings.Fields(strings.ToLower(query.Query))
	err = filepath.WalkDir(abs, func(path string, entry fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			if path == abs && os.IsNotExist(walkErr) {
				return nil
			}
			result.Skipped++
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			result.Skipped++
			return nil
		}
		rel := s.relativePath(path)
		if entry.IsDir() {
			if entry.Name() == ".attachments" {
				return filepath.SkipDir
			}
			result.Folders = append(result.Folders, rel)
			return nil
		}
		if !strings.EqualFold(filepath.Ext(path), ".md") || (query.Folder != "" && !strings.EqualFold(filepath.ToSlash(filepath.Dir(rel)), query.Folder)) {
			return nil
		}
		note, err := s.ReadNote(ctx, rel)
		if err != nil {
			result.Skipped++
			return nil
		}
		if query.Tag != "" {
			found := false
			for _, tag := range note.Tags {
				found = found || strings.EqualFold(tag, query.Tag)
			}
			if !found {
				return nil
			}
		}
		search := strings.ToLower(note.Title + " " + rel + " " + strings.Join(note.Tags, " ") + " " + note.Content)
		for _, word := range words {
			if !strings.Contains(search, word) {
				return nil
			}
		}
		if len(words) > 0 {
			lower := strings.ToLower(note.Content)
			if i := strings.Index(lower, words[0]); i >= 0 {
				// Byte offsets in Unicode lowercase may differ; only slice the lowered search text.
				note.Snippet = stringLimit(strings.Join(strings.Fields(lower[i:]), " "), 180)
			}
		}
		note.Content = ""
		result.Notes = append(result.Notes, note)
		return nil
	})
	if err != nil {
		return result, err
	}
	sort.Slice(result.Notes, func(i, j int) bool {
		if result.Notes[i].Modified.Equal(result.Notes[j].Modified) {
			return result.Notes[i].Path < result.Notes[j].Path
		}
		return result.Notes[i].Modified.After(result.Notes[j].Modified)
	})
	result.Total = len(result.Notes)
	start := min(query.Offset, result.Total)
	result.Notes = result.Notes[start:min(start+query.Limit, result.Total)]
	return result, nil
}

func (s *Service) CreateNote(ctx context.Context, title, content, folder, source string) (Note, error) {
	if len(content) > MaxNoteBytes || !utf8.ValidString(content) || utf8.RuneCountInString(title) > 160 {
		return Note{}, fmt.Errorf("invalid note title or content")
	}
	if folder == "" {
		folder = NotesDirectory
	}
	if !validNotePath(folder+"/note.md", false) {
		return Note{}, fmt.Errorf("notes must be created in Documents/Notes")
	}
	var slug strings.Builder
	for _, char := range strings.ToLower(title) {
		if unicode.IsLetter(char) || unicode.IsDigit(char) || char == '-' || char == '_' {
			slug.WriteRune(char)
		} else if char == ' ' {
			slug.WriteByte('-')
		}
	}
	name := strings.Trim(stringLimit(slug.String(), 60), "-_")
	if name == "" {
		name = "note"
	}
	for attempt := 0; attempt < 5; attempt++ {
		fileName := name
		if attempt > 0 {
			fileName += "-" + uuid.NewString()[:8]
		}
		path := filepath.ToSlash(filepath.Join(folder, fileName+".md"))
		_, err := s.WriteFileBytesConditional(ctx, path, []byte(content), source, CheckNoteVersion("", true))
		if errors.Is(err, ErrNoteConflict) || (attempt == 0 && errors.Is(err, ErrAgentNoteMutation)) {
			continue
		}
		if err != nil {
			return Note{}, err
		}
		return s.ReadNote(ctx, path)
	}
	return Note{}, ErrNoteConflict
}
