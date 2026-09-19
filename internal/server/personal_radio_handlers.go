package server

import (
	"aurago/internal/personalradio"
	"aurago/internal/tools"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func radioJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func radioError(w http.ResponseWriter, err error) {
	code := "radio_request_failed"
	status := 400
	switch {
	case errors.Is(err, personalradio.ErrNotFound):
		status = 404
	case errors.Is(err, personalradio.ErrConflict), errors.Is(err, personalradio.ErrLease):
		status = 409
	case errors.Is(err, personalradio.ErrLimit):
		status = 429
	}
	if strings.HasPrefix(err.Error(), "radio_") && !strings.ContainsAny(err.Error(), " :/\\\n") {
		code = err.Error()
	}
	radioJSON(w, status, map[string]string{"error": code})
}
func radioDecode(w http.ResponseWriter, r *http.Request, v any) error {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("radio_invalid_request")
	}
	return nil
}

type radioPlaybackRequest struct {
	Device   string `json:"device"`
	Epoch    string `json:"epoch"`
	Current  string `json:"current"`
	Position int64  `json:"position"`
	Kind     string `json:"kind"`
	Takeover bool   `json:"takeover"`
}

func (s *Server) handlePersonalRadio(w http.ResponseWriter, r *http.Request) {
	scope := desktopScopeRead
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		scope = desktopScopeWrite
	}
	if !requireDesktopPermission(s, w, r, scope) {
		return
	}
	cfg := s.ConfigSnapshot()
	if cfg == nil || !cfg.VirtualDesktop.Enabled {
		radioJSON(w, 503, map[string]string{"error": "radio_disabled"})
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead && cfg.VirtualDesktop.ReadOnly {
		radioJSON(w, 403, map[string]string{"error": "radio_read_only"})
		return
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/desktop/personal-radio/"), "/")
	parts := strings.Split(path, "/")
	svc := s.PersonalRadio
	if path == "state" && r.Method == http.MethodGet {
		caps := map[string]bool{"music": cfg.MusicConfigured() && s.MediaRegistryDB != nil, "tts": chatVoiceOutputTTSConfigured(cfg), "llm": s.LLMClient != nil, "news": cfg.BraveSearch.Enabled && cfg.BraveSearch.APIKey != "" && cfg.Tools.WebScraper.Enabled && cfg.Agent.AllowNetworkRequests, "read_only": cfg.VirtualDesktop.ReadOnly}
		if svc == nil {
			radioJSON(w, 200, map[string]any{"available": false, "capabilities": caps, "stations": []any{}})
			return
		}
		stations, err := svc.Stations()
		if err != nil {
			radioError(w, err)
			return
		}
		radioJSON(w, 200, map[string]any{"available": true, "capabilities": caps, "stations": stations, "defaults": personalradio.DefaultStation(), "state": svc.Snapshot()})
		return
	}
	if svc == nil {
		radioJSON(w, 503, map[string]string{"error": "radio_unavailable"})
		return
	}
	if len(parts) == 2 && parts[0] == "audio" && r.Method == http.MethodGet {
		offset, _ := strconv.ParseInt(r.URL.Query().Get("offset"), 10, 64)
		length, _ := strconv.ParseInt(r.URL.Query().Get("length"), 10, 64)
		if length == 0 {
			length = 30000
		}
		data, err := svc.AudioWindow(parts[1], offset, length)
		if err != nil {
			radioError(w, err)
			return
		}
		w.Header().Set("Content-Type", "audio/wav")
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		http.ServeContent(w, r, "radio.wav", time.Time{}, bytes.NewReader(data))
		return
	}
	if path == "library" && r.Method == http.MethodGet {
		s.radioLibrary(w, r)
		return
	}
	if path == "files" && r.Method == http.MethodGet {
		s.radioFiles(w, r)
		return
	}
	if path == "unused" && r.Method == http.MethodDelete {
		n, err := svc.DeleteUnused()
		if err != nil {
			radioError(w, err)
			return
		}
		radioJSON(w, 200, map[string]int{"deleted": n})
		return
	}
	if (path == "heartbeat" || path == "playback") && r.Method == http.MethodPost {
		var v radioPlaybackRequest
		if err := radioDecode(w, r, &v); err != nil {
			radioError(w, err)
			return
		}
		var err error
		if path == "heartbeat" {
			err = svc.Heartbeat(v.Device, v.Epoch, v.Current, v.Position)
		} else {
			err = svc.Playback(v.Device, v.Epoch, v.Current, v.Kind)
		}
		if err != nil {
			radioError(w, err)
			return
		}
		radioJSON(w, 200, svc.Snapshot())
		return
	}
	if parts[0] != "stations" {
		http.NotFound(w, r)
		return
	}
	if len(parts) == 1 {
		if r.Method == http.MethodPost {
			p := personalradio.DefaultStation()
			if err := radioDecode(w, r, &p); err != nil {
				radioError(w, err)
				return
			}
			p.ID = ""
			p.Revision = 0
			v, err := svc.SaveStation(p)
			if err != nil {
				radioError(w, err)
				return
			}
			radioJSON(w, 201, v)
			return
		}
		w.WriteHeader(405)
		return
	}
	id := parts[1]
	if len(parts) == 2 {
		switch r.Method {
		case http.MethodPatch:
			p := personalradio.DefaultStation()
			if err := radioDecode(w, r, &p); err != nil {
				radioError(w, err)
				return
			}
			p.ID = id
			v, err := svc.SaveStation(p)
			if err != nil {
				radioError(w, err)
				return
			}
			radioJSON(w, 200, v)
		case http.MethodDelete:
			rev, _ := strconv.Atoi(r.Header.Get("If-Match"))
			if err := svc.DeleteStation(id, rev); err != nil {
				radioError(w, err)
				return
			}
			radioJSON(w, 200, map[string]bool{"deleted": true})
		default:
			w.WriteHeader(405)
		}
		return
	}
	if len(parts) == 3 {
		switch parts[2] {
		case "preview":
			if r.Method != http.MethodPost {
				w.WriteHeader(405)
				return
			}
			audio, err := svc.Preview(r.Context(), id, "Personal Radio.")
			if err != nil {
				radioError(w, err)
				return
			}
			w.Header().Set("Content-Type", "audio/wav")
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Write(audio.Data)
			return
		case "news":
			if r.Method != http.MethodGet {
				w.WriteHeader(405)
				return
			}
			radioJSON(w, 200, map[string]any{"items": svc.News(id)})
			return
		case "start", "pause", "resume", "stop", "skip":
			if r.Method != http.MethodPost {
				w.WriteHeader(405)
				return
			}
			var v radioPlaybackRequest
			if err := radioDecode(w, r, &v); err != nil {
				radioError(w, err)
				return
			}
			var err error
			if parts[2] == "start" {
				_, err = svc.Start(id, v.Device, v.Takeover)
			} else {
				if svc.Snapshot().StationID != id {
					err = personalradio.ErrConflict
				} else {
					err = svc.Control(v.Device, v.Epoch, parts[2], v.Current)
				}
			}
			if err != nil {
				radioError(w, err)
				return
			}
			svc.Tick()
			radioJSON(w, 200, svc.Snapshot())
			return
		case "tracks":
			if r.Method == http.MethodGet {
				v, err := svc.Tracks(id)
				if err != nil {
					radioError(w, err)
					return
				}
				radioJSON(w, 200, map[string]any{"items": v})
				return
			}
		case "imports":
			if r.Method == http.MethodPost {
				s.radioImport(w, r, id)
				return
			}
		case "upload":
			if r.Method == http.MethodPost {
				s.radioUpload(w, r, id)
				return
			}
		}
	}
	if len(parts) == 4 && parts[2] == "tracks" {
		var err error
		switch r.Method {
		case http.MethodPatch:
			var v struct {
				Favorite bool `json:"favorite"`
				Blocked  bool `json:"blocked"`
				Weight   int  `json:"weight"`
			}
			if err = radioDecode(w, r, &v); err == nil {
				err = svc.UpdateTrack(id, parts[3], v.Favorite, v.Blocked, v.Weight)
			}
		case http.MethodDelete:
			err = svc.RemoveTrack(id, parts[3])
		default:
			w.WriteHeader(405)
			return
		}
		if err != nil {
			radioError(w, err)
			return
		}
		radioJSON(w, 200, map[string]bool{"ok": true})
		return
	}
	w.WriteHeader(405)
}

func (s *Server) radioLibrary(w http.ResponseWriter, r *http.Request) {
	if s.MediaRegistryDB == nil {
		radioJSON(w, 200, map[string]any{"items": []any{}})
		return
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	items, total, err := tools.SearchMedia(s.MediaRegistryDB, radioBound(r.URL.Query().Get("q"), 100), "music", nil, 50, max(0, offset))
	if err != nil {
		radioError(w, err)
		return
	}
	out := []map[string]any{}
	for _, x := range items {
		out = append(out, map[string]any{"id": x.ID, "title": x.Description, "filename": x.Filename, "duration_ms": x.DurationMs})
	}
	radioJSON(w, 200, map[string]any{"items": out, "total": total})
}
func (s *Server) radioFiles(w http.ResponseWriter, r *http.Request) {
	svc, _, err := s.getDesktopService(r.Context())
	if err != nil {
		radioError(w, err)
		return
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	files, more, err := svc.ListFilesRecursive(r.Context(), r.URL.Query().Get("path"), max(0, offset), 100)
	if err != nil {
		radioError(w, err)
		return
	}
	out := []string{}
	for _, f := range files {
		ext := strings.ToLower(filepath.Ext(f.Name))
		if f.Type == "file" && (ext == ".mp3" || ext == ".wav") {
			out = append(out, f.Path)
		}
	}
	radioJSON(w, 200, map[string]any{"paths": out, "more": more, "next_offset": max(0, offset) + len(files)})
}

func (s *Server) radioImport(w http.ResponseWriter, r *http.Request, station string) {
	var v struct {
		Path    string `json:"path"`
		MediaID int64  `json:"media_id"`
		Genre   string `json:"genre"`
	}
	if err := radioDecode(w, r, &v); err != nil {
		radioError(w, err)
		return
	}
	if len(v.Genre) > 80 {
		radioError(w, personalradio.ErrLimit)
		return
	}
	var file *os.File
	var title, origin, extension string
	var err error
	origin = "local"
	if v.MediaID > 0 {
		if s.MediaRegistryDB == nil {
			radioError(w, personalradio.ErrNotFound)
			return
		}
		item, e := tools.GetMedia(s.MediaRegistryDB, v.MediaID)
		if e != nil || item.MediaType != "music" {
			radioError(w, personalradio.ErrNotFound)
			return
		}
		cfg := s.ConfigSnapshot()
		abs, e := filepath.Abs(item.FilePath)
		if e != nil {
			radioError(w, e)
			return
		}
		base, e := filepath.Abs(cfg.Directories.DataDir)
		if e != nil {
			radioError(w, e)
			return
		}
		rel, e := filepath.Rel(base, abs)
		if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			radioError(w, personalradio.ErrNotFound)
			return
		}
		root, e := os.OpenRoot(base)
		if e != nil {
			radioError(w, e)
			return
		}
		file, err = root.Open(rel)
		root.Close()
		title = item.Description
		extension = filepath.Ext(item.Filename)
		if item.SourceTool == "generate_music" {
			origin = "generated"
		}
	} else {
		svc, _, e := s.getDesktopService(r.Context())
		if e != nil {
			radioError(w, e)
			return
		}
		var entryName string
		f, entry, _, e := svc.OpenPreviewFile(r.Context(), v.Path)
		file, err = f, e
		entryName = entry.Name
		title = entryName
		extension = filepath.Ext(entryName)
	}
	if err != nil {
		radioError(w, err)
		return
	}
	defer file.Close()
	track, err := s.PersonalRadio.Import(r.Context(), station, file, extension, title, origin, v.Genre, v.MediaID)
	if err != nil {
		radioError(w, err)
		return
	}
	radioJSON(w, 201, track)
}
func (s *Server) radioUpload(w http.ResponseWriter, r *http.Request, station string) {
	name := filepath.Base(r.URL.Query().Get("name"))
	extension := strings.ToLower(filepath.Ext(name))
	if extension != ".mp3" && extension != ".wav" {
		radioError(w, errors.New("radio_format_unsupported"))
		return
	}
	genre := radioBound(r.URL.Query().Get("genre"), 80)
	cfg := s.ConfigSnapshot()
	dir := filepath.Join(cfg.Directories.DataDir, "personal-radio")
	file, err := os.CreateTemp(dir, "import-*")
	if err != nil {
		radioError(w, err)
		return
	}
	defer func() { file.Close(); os.Remove(file.Name()) }()
	r.Body = http.MaxBytesReader(w, r.Body, personalradio.MaxImportBytes)
	if _, err = io.Copy(file, r.Body); err != nil {
		radioError(w, personalradio.ErrLimit)
		return
	}
	track, err := s.PersonalRadio.Import(r.Context(), station, file, extension, name, "local", genre, 0)
	if err != nil {
		radioError(w, err)
		return
	}
	radioJSON(w, 201, track)
}
