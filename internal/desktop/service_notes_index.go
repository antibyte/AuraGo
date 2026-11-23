package desktop

import (
	"context"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// NoteIndexLink is one outgoing link of a note; Path is the resolved note or attachment ("" when unresolved).
type NoteIndexLink struct {
	Target string `json:"target"` // raw value
	Path   string `json:"path"`   // resolved path or ""
	Embed  bool   `json:"embed,omitempty"`
	Kind   string `json:"kind"`
}

type NoteIndexEntry struct {
	Path     string          `json:"path"`
	Title    string          `json:"title"`
	Aliases  []string        `json:"aliases"`
	Tags     []string        `json:"tags"`
	Links    []NoteIndexLink `json:"links"`
	Modified time.Time       `json:"modified"`
	Size     int64           `json:"size"`
}

type NotesIndexResult struct {
	Notes       []NoteIndexEntry `json:"notes"`
	Folders     []string         `json:"folders"`
	Attachments []string         `json:"attachments"`
	Skipped     int              `json:"skipped"`
}

// NoteLinkUpdates reports which notes MoveNoteWithLinks rewrote (Updated: referencing notes, Relocated: the moved note)
// or could not rewrite (Failed).
type NoteLinkUpdates struct {
	Updated   []string
	Failed    []string
	Relocated bool // the moved note itself was rewritten so its relative links still point at the same targets
}

type notesIndexCacheEntry struct {
	mod  time.Time
	size int64
	refs NoteRefs
}

// NotesIndex walks Documents/Notes and returns titles, aliases, tags and resolved links.
// Parsed files are cached by path + mtime + size; entries of vanished files are dropped.
func (s *Service) NotesIndex(ctx context.Context) (NotesIndexResult, error) {
	result := NotesIndexResult{Notes: []NoteIndexEntry{}, Folders: []string{}, Attachments: []string{}}
	if err := s.ensureReady(ctx); err != nil {
		return result, err
	}
	abs, err := s.resolveWorkspacePathNoSymlinks(NotesDirectory, true)
	if err != nil {
		return result, err
	}
	s.notesIndexMu.Lock()
	defer s.notesIndexMu.Unlock()
	if s.notesIndexCache == nil {
		s.notesIndexCache = map[string]notesIndexCacheEntry{}
	}
	seen := map[string]bool{}
	type parsed struct {
		entry NoteIndexEntry
		refs  NoteRefs
		name  string
	}
	var notes []parsed
	err = filepath.WalkDir(abs, func(p string, entry fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			if p == abs && os.IsNotExist(walkErr) {
				return nil
			}
			result.Skipped++
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			result.Skipped++
			return nil
		}
		rel := s.relativePath(p)
		underAttachments := inNoteAttachments(rel)
		if entry.IsDir() {
			if !underAttachments {
				result.Folders = append(result.Folders, rel)
			}
			return nil
		}
		// Everything below ".attachments" is an attachment, even Markdown files (SearchNotes skips the folder too).
		if underAttachments || !strings.EqualFold(filepath.Ext(p), ".md") {
			if rel != NotesDirectory+"/notes.meta.json" {
				result.Attachments = append(result.Attachments, rel)
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			result.Skipped++
			return nil
		}
		seen[rel] = true
		cached, ok := s.notesIndexCache[rel]
		if ok && cached.mod.Equal(info.ModTime()) && cached.size == info.Size() {
			s.notesIndexCacheHits++
		} else {
			data, _, err := s.ReadFileBytes(ctx, rel)
			if err != nil || len(data) > MaxNoteBytes || !utf8.Valid(data) {
				result.Skipped++
				delete(s.notesIndexCache, rel)
				seen[rel] = false
				return nil
			}
			refs, err := ExtractNoteRefsContext(ctx, string(data))
			if err != nil {
				return err
			}
			cached = notesIndexCacheEntry{mod: info.ModTime(), size: info.Size(), refs: refs}
			s.notesIndexCache[rel] = cached
		}
		title := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		if cached.refs.TitleSource == "frontmatter" || cached.refs.TitleSource == "heading" {
			title = cached.refs.Title
		}
		notes = append(notes, parsed{
			entry: NoteIndexEntry{Path: rel, Title: title, Aliases: cached.refs.Aliases, Tags: cached.refs.Tags, Links: []NoteIndexLink{}, Modified: info.ModTime(), Size: info.Size()},
			refs:  cached.refs,
		})
		return nil
	})
	if err != nil {
		return result, err
	}
	for key := range s.notesIndexCache {
		if !seen[key] {
			delete(s.notesIndexCache, key)
		}
	}
	sort.Slice(notes, func(i, j int) bool { return notes[i].entry.Path < notes[j].entry.Path })
	sort.Strings(result.Folders)
	sort.Strings(result.Attachments)
	paths := make([]string, len(notes))
	aliases := map[string][]string{}
	for i, n := range notes {
		paths[i] = n.entry.Path
		if len(n.refs.Aliases) > 0 {
			aliases[n.entry.Path] = n.refs.Aliases
		}
	}
	lookup := newNoteLookup(paths, aliases, result.Attachments)
	for _, n := range notes {
		for _, link := range n.refs.Links {
			n.entry.Links = append(n.entry.Links, NoteIndexLink{Target: link.Value, Path: lookup.Resolve(n.entry.Path, link.Kind, link.Value, link.Embed), Embed: link.Embed, Kind: link.Kind})
		}
		result.Notes = append(result.Notes, n.entry)
	}
	return result, nil
}

// inNoteAttachments reports whether a workspace path is the ".attachments" folder or lies inside one.
func inNoteAttachments(rel string) bool {
	for _, segment := range strings.Split(filepath.ToSlash(rel), "/") {
		if segment == ".attachments" {
			return true
		}
	}
	return false
}

// relativeNoteURL builds the URL-escaped relative path from the folder of note "from" to "to".
func relativeNoteURL(from, to string) string {
	a := strings.Split(path.Dir(from), "/")
	b := strings.Split(to, "/")
	common := 0
	for common < len(a) && common < len(b)-1 && a[common] == b[common] {
		common++
	}
	var parts []string
	for i := common; i < len(a); i++ {
		parts = append(parts, "..")
	}
	for _, seg := range b[common:] {
		parts = append(parts, url.PathEscape(seg))
	}
	return strings.Join(parts, "/")
}

// MoveNoteWithLinks moves a note and rewrites wikilinks and Markdown links that pointed to it.
// The move itself is atomic and versioned; each referencing note is rewritten with its own version
// precondition, and notes changed in the meantime are reported in Failed.
func (s *Service) MoveNoteWithLinks(ctx context.Context, from, to, expected, source string) (NoteLinkUpdates, error) {
	var updates NoteLinkUpdates
	from, to = filepath.ToSlash(filepath.Clean(from)), filepath.ToSlash(filepath.Clean(to))
	index, err := s.NotesIndex(ctx)
	if err != nil {
		return updates, err
	}
	var paths, attachments = make([]string, 0, len(index.Notes)), index.Attachments
	aliases := map[string][]string{}
	var referencing []string
	for _, n := range index.Notes {
		paths = append(paths, n.Path)
		if len(n.Aliases) > 0 {
			aliases[n.Path] = n.Aliases
		}
		if strings.EqualFold(n.Path, from) {
			continue
		}
		for _, link := range n.Links {
			if strings.EqualFold(link.Path, from) {
				referencing = append(referencing, n.Path)
				break
			}
		}
	}
	oldLookup := newNoteLookup(paths, aliases, attachments)
	if err := s.MovePathConditional(ctx, from, to, source, CheckNoteVersion(expected, false)); err != nil {
		return updates, err
	}
	var newPaths []string
	newAliases := map[string][]string{}
	for _, p := range paths {
		if strings.EqualFold(p, from) {
			p = to
		}
		newPaths = append(newPaths, p)
	}
	for p, list := range aliases {
		if strings.EqualFold(p, from) {
			p = to
		}
		newAliases[p] = list
	}
	newLookup := newNoteLookup(newPaths, newAliases, attachments)
	linkText := newLookup.LinkText(to)
	// "# | ^ [ ] \" and line breaks cannot be written inside a wikilink target (Obsidian forbids them in names too):
	// such a note is not rewritten but reported in Failed, so its links stay visibly stale instead of silently broken.
	wikiTextOK := !strings.ContainsAny(linkText, "#|^[]\\\r\n")
	sameDir := path.Dir(from) == path.Dir(to)

	// rewrite updates one note: a note that referenced the moved one (base == its own path) or the moved note
	// itself (moved == true, read from "to" but resolved as it was at "from"). A note that cannot be rewritten
	// safely is left untouched and reported in Failed.
	rewrite := func(readPath, base string, moved bool) {
		fail := func() { updates.Failed = append(updates.Failed, readPath) }
		data, _, err := s.ReadFileBytes(ctx, readPath)
		if err != nil || len(data) > MaxNoteBytes || !utf8.Valid(data) {
			fail()
			return
		}
		version := NoteVersion(data)
		content := string(data)
		unsafeName := false
		rewritten := RewriteNoteLinks(content, func(link NoteRefLink) (string, bool) {
			if link.Kind == "markdown" {
				target := to
				if moved {
					target = oldLookup.Resolve(base, "markdown", link.Value, link.Embed)
					if target == "" {
						target = lexicalNoteTarget(base, link.Value)
					}
					if target == "" {
						return "", false
					}
					if strings.EqualFold(target, from) {
						target = to
					} else if sameDir {
						return "", false
					}
				} else if !strings.EqualFold(oldLookup.Resolve(base, "markdown", link.Value, link.Embed), from) {
					return "", false
				}
				suffix := ""
				if i := strings.IndexAny(link.Value, "#?"); i >= 0 {
					suffix = link.Value[i:]
				}
				if replacement := relativeNoteURL(readPath, target) + suffix; replacement != link.Value {
					return replacement, true
				}
				return "", false
			}
			target := normalizeWikiTarget(ParseWikiValue(link.Value).Target)
			if target == "" {
				return "", false
			}
			hasMD := strings.HasSuffix(strings.ToLower(target), ".md")
			name := target
			if hasMD {
				name = target[:len(target)-3]
			}
			// Only links that name the note (not its aliases) are rewritten.
			if !strings.EqualFold(oldLookup.notes.find(base, name), from) {
				return "", false
			}
			if !wikiTextOK {
				unsafeName = true
				return "", false
			}
			text := linkText
			if hasMD {
				text += ".md"
			}
			return ReplaceWikiTarget(link.Value, text), true
		})
		if unsafeName || len(rewritten) > MaxNoteBytes {
			fail()
			return
		}
		if rewritten == content {
			return
		}
		if s.beforeNoteLinkWrite != nil {
			s.beforeNoteLinkWrite(readPath)
		}
		if _, err := s.WriteFileBytesConditional(ctx, readPath, []byte(rewritten), source, CheckNoteVersion(version, false)); err != nil {
			fail()
			return
		}
		if moved {
			updates.Relocated = true
		} else {
			updates.Updated = append(updates.Updated, readPath)
		}
	}

	// The move already happened: a cancelled request must not hide it, so unfinished notes are reported in Failed.
	todo := append([]string{to}, referencing...)
	for i, ref := range todo {
		if ctx.Err() != nil {
			updates.Failed = append(updates.Failed, todo[i:]...)
			break
		}
		if i == 0 {
			rewrite(to, from, true)
		} else {
			rewrite(ref, ref, false)
		}
	}
	sort.Strings(updates.Failed)
	return updates, nil
}
