package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"aurago/internal/desktop"
	"github.com/google/uuid"
)

func notesEditablePath(path string) bool {
	p := filepath.ToSlash(filepath.Clean(strings.ReplaceAll(path, "\\", "/")))
	return p == desktop.NotesDirectory+"/notes.meta.json" || strings.HasPrefix(p, desktop.NotesDirectory+"/") && strings.EqualFold(filepath.Ext(p), ".md")
}

// notesFolderPath validates a new folder: strictly inside Documents/Notes, no "..", no .attachments.
func notesFolderPath(raw string) (string, bool) {
	raw = strings.ReplaceAll(raw, "\\", "/")
	for _, seg := range strings.Split(raw, "/") {
		if seg == ".." || seg == "." || strings.EqualFold(seg, ".attachments") {
			return "", false
		}
	}
	p := filepath.ToSlash(filepath.Clean(raw))
	if !strings.HasPrefix(p, desktop.NotesDirectory+"/") || len(p) > 500 {
		return "", false
	}
	return p, true
}

func handleDesktopNotes(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopMethodScope(r.Method)) {
			return
		}
		svc, hub, err := s.getDesktopService(r.Context())
		if err != nil {
			jsonError(w, err.Error(), 503)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		fail := func(err error) {
			code := 400
			if errors.Is(err, desktop.ErrNoteConflict) {
				code = 412
			}
			if errors.Is(err, os.ErrNotExist) {
				code = 404
			}
			jsonError(w, err.Error(), code)
		}
		publish := func(operation, path, old string) {
			broadcastDesktopEvent(s, hub, desktop.Event{Type: "desktop_changed", Payload: map[string]interface{}{"operation": operation, "path": path, "old_path": old, "new_path": path}, CreatedAt: time.Now().UTC()})
		}
		path := r.URL.Query().Get("path")
		if r.Method == http.MethodGet {
			if r.URL.Query().Get("index") == "1" {
				index, err := svc.NotesIndex(r.Context())
				if err != nil {
					fail(err)
					return
				}
				json.NewEncoder(w).Encode(index)
				return
			}
			if path != "" {
				if path == desktop.NotesDirectory+"/notes.meta.json" {
					data, _, err := svc.ReadFileBytes(r.Context(), path)
					if err != nil {
						fail(err)
						return
					}
					w.Header().Set("ETag", desktop.NoteVersion(data))
					json.NewEncoder(w).Encode(map[string]interface{}{"content": string(data), "version": desktop.NoteVersion(data)})
					return
				}
				note, err := svc.ReadNote(r.Context(), path)
				if err != nil {
					fail(err)
					return
				}
				w.Header().Set("ETag", note.Version)
				json.NewEncoder(w).Encode(note)
				return
			}
			limit, offset := 0, 0
			for key, target := range map[string]*int{"limit": &limit, "offset": &offset} {
				if raw := r.URL.Query().Get(key); raw != "" {
					n, err := strconv.Atoi(raw)
					if err != nil {
						jsonError(w, "Invalid note pagination", 400)
						return
					}
					*target = n
				}
			}
			result, err := svc.SearchNotes(r.Context(), desktop.NotesQuery{Query: r.URL.Query().Get("q"), Folder: r.URL.Query().Get("folder"), Tag: r.URL.Query().Get("tag"), Limit: limit, Offset: offset, Trash: r.URL.Query().Get("trash") == "true"})
			if err != nil {
				fail(err)
				return
			}
			json.NewEncoder(w).Encode(result)
			return
		}
		var body struct {
			Path      string `json:"path"`
			Title     string `json:"title"`
			Content   string `json:"content"`
			Folder    string `json:"folder"`
			NewPath   string `json:"new_path"`
			Operation string `json:"operation"`
			// UpdateLinks rewrites wikilinks and Markdown links of other notes on "move".
			UpdateLinks bool `json:"update_links"`
		}
		if err := decodeDesktopJSON(w, r, &body, 6<<20); err != nil {
			jsonError(w, "Invalid note request", 400)
			return
		}
		if body.Path != "" {
			path = body.Path
		}
		if r.Method == http.MethodPost {
			if body.Operation == "mkdir" {
				folder, ok := notesFolderPath(body.Folder)
				if !ok {
					jsonError(w, "Invalid note folder", 400)
					return
				}
				if err := svc.CreateDirectory(r.Context(), folder, desktop.SourceUser); err != nil {
					fail(err)
					return
				}
				publish("create_directory", folder, "")
				json.NewEncoder(w).Encode(map[string]string{"path": folder})
				return
			}
			note, err := svc.CreateNote(r.Context(), body.Title, body.Content, body.Folder, desktop.SourceUser)
			if err != nil {
				fail(err)
				return
			}
			publish("write_file", note.Path, "")
			w.Header().Set("ETag", note.Version)
			json.NewEncoder(w).Encode(note)
			return
		}
		expected, create := r.Header.Get("If-Match"), r.Header.Get("If-None-Match") == "*"
		if expected == "" && !create {
			jsonError(w, "A note version is required", 428)
			return
		}
		if expected != "" && create {
			jsonError(w, "Conflicting preconditions", 400)
			return
		}
		if r.Method == http.MethodPut {
			if !notesEditablePath(path) || len(body.Content) > desktop.MaxNoteBytes || !utf8.ValidString(body.Content) {
				jsonError(w, "Invalid note path or content", 400)
				return
			}
			if path == desktop.NotesDirectory+"/notes.meta.json" && !json.Valid([]byte(body.Content)) {
				jsonError(w, "Invalid note metadata", 400)
				return
			}
			entry, err := svc.WriteFileBytesConditional(r.Context(), path, []byte(body.Content), desktop.SourceUser, desktop.CheckNoteVersion(expected, create))
			if err != nil {
				fail(err)
				return
			}
			publish("write_file", entry.Path, "")
			version := desktop.NoteVersion([]byte(body.Content))
			w.Header().Set("ETag", version)
			json.NewEncoder(w).Encode(desktop.Note{Path: entry.Path, Title: strings.TrimSuffix(entry.Name, filepath.Ext(entry.Name)), Content: body.Content, Version: version, Modified: entry.ModTime, Size: entry.Size})
			return
		}
		if r.Method == http.MethodPatch {
			if create {
				jsonError(w, "Moving notes requires a source version", 400)
				return
			}
			if _, err := svc.ReadNote(r.Context(), path); err != nil {
				fail(err)
				return
			}
			target := body.NewPath
			switch body.Operation {
			case "trash":
				if !notesEditablePath(path) {
					jsonError(w, "Invalid note path", 400)
					return
				}
				target = desktop.NotesTrashDirectory + "/" + uuid.NewString() + "/" + strings.TrimPrefix(path, desktop.NotesDirectory+"/")
			case "restore":
				if !strings.HasPrefix(path, desktop.NotesTrashDirectory+"/") {
					jsonError(w, "Note is not in the trash", 400)
					return
				}
				if target == "" {
					parts := strings.SplitN(strings.TrimPrefix(path, desktop.NotesTrashDirectory+"/"), "/", 2)
					if len(parts) != 2 {
						jsonError(w, "Invalid trashed note path", 400)
						return
					}
					target = desktop.NotesDirectory + "/" + parts[1]
				}
			case "move":
			default:
				jsonError(w, "Unknown note operation", 400)
				return
			}
			if body.Operation != "trash" && !notesEditablePath(target) {
				jsonError(w, "Invalid note destination", 400)
				return
			}
			if strings.HasSuffix(target, "notes.meta.json") {
				jsonError(w, "Invalid note destination", 400)
				return
			}
			var updates desktop.NoteLinkUpdates
			if body.UpdateLinks && body.Operation == "move" {
				updates, err = svc.MoveNoteWithLinks(r.Context(), path, target, expected, desktop.SourceUser)
				if err != nil {
					fail(err)
					return
				}
			} else if err := svc.MovePathConditional(r.Context(), path, target, desktop.SourceUser, desktop.CheckNoteVersion(expected, false)); err != nil {
				fail(err)
				return
			}
			publish("move_path", target, path)
			for _, updated := range updates.Updated {
				publish("write_file", updated, "")
			}
			note, err := svc.ReadNote(r.Context(), target)
			if err != nil {
				fail(err)
				return
			}
			w.Header().Set("ETag", note.Version)
			json.NewEncoder(w).Encode(struct {
				desktop.Note
				UpdatedLinks     int      `json:"updated_links"`
				LinkUpdateFailed []string `json:"link_update_failed,omitempty"`
			}{note, len(updates.Updated), updates.Failed})
			return
		}
		jsonError(w, "Method not allowed", 405)
	}
}
