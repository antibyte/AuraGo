package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"
	"sync"

	"aurago/internal/config"
	"aurago/internal/gog"
)

// GogStorage keeps request-local aliases only. Google IDs, never display names,
// identify existing objects; aliases connect newly created transfer directories.
type GogStorage struct {
	Service *gog.Service
	run     func(context.Context, string, gog.Request, *gog.Files) gog.Result
	mu      sync.Mutex
	aliases map[string]StorageLocation
}

func (g *GogStorage) remember(loc StorageLocation) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.aliases == nil {
		g.aliases = map[string]StorageLocation{}
	}
	g.aliases[loc.AccountID+"/"+loc.Path] = loc
}
func (g *GogStorage) cached(loc StorageLocation) StorageLocation {
	g.mu.Lock()
	defer g.mu.Unlock()
	if v, ok := g.aliases[loc.AccountID+"/"+loc.Path]; ok && loc.ItemID == "" {
		return v
	}
	return loc
}
func (g *GogStorage) call(ctx context.Context, loc StorageLocation, op string, args map[string]any, files *gog.Files) (json.RawMessage, error) {
	if g == nil || g.Service == nil {
		return nil, ErrStorageReadOnly
	}
	run := g.run
	if run == nil {
		run = g.Service.Execute
	}
	r := run(ctx, "desktop", gog.Request{AccountID: loc.AccountID, Service: "drive", Operation: op, Args: args}, files)
	if r.Status == "external_status_unknown" {
		return nil, fmt.Errorf("storage_outcome_unknown: inspect Drive before another write")
	}
	if r.Status != "ok" {
		if r.Code == "not_found" {
			return nil, ErrStorageNotFound
		}
		if r.Code == "permission_denied" || r.Code == "readonly" {
			return nil, ErrStorageReadOnly
		}
		return nil, fmt.Errorf("gog: %s", r.Code)
	}
	return r.Data, nil
}

type gogDriveFile struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	MimeType string      `json:"mimeType"`
	Size     json.Number `json:"size"`
	Version  json.Number `json:"version"`
	Modified string      `json:"modifiedTime"`
	Parents  []string    `json:"parents"`
}

func (g *GogStorage) entry(loc StorageLocation, f gogDriveFile) StorageEntry {
	size, _ := strconv.ParseInt(string(f.Size), 10, 64)
	kind := "file"
	if f.MimeType == "application/vnd.google-apps.folder" {
		kind = "directory"
	}
	parent := ""
	if len(f.Parents) > 0 {
		parent = f.Parents[0]
	}
	version := ""
	if f.Version != "" {
		version = "gog-" + string(f.Version)
	}
	a, _ := g.Service.Config().Gog.Account(loc.AccountID)
	grants := a.Services["drive"]
	e := StorageEntry{Name: f.Name, Path: loc.Path, ItemID: f.ID, ParentID: parent, Type: kind, Size: size, ContentType: f.MimeType, Modified: f.Modified, Version: version, ReadOnly: g.Service.Config().Gog.ReadOnly || !grants.Update && !grants.Create && !grants.Delete}
	loc.ItemID = f.ID
	loc.ParentID = parent
	loc.Name = f.Name
	g.remember(loc)
	return e
}
func decodeGogFile(data []byte) (gogDriveFile, error) {
	var f gogDriveFile
	var wrap struct {
		File json.RawMessage `json:"file"`
	}
	_ = json.Unmarshal(data, &wrap)
	if len(wrap.File) > 0 {
		data = wrap.File
	}
	e := json.Unmarshal(data, &f)
	if e == nil && !validGogObjectID(f.ID) {
		e = fmt.Errorf("invalid Google file ID")
	}
	return f, e
}
func validGogObjectID(id string) bool {
	return id != "" && len(id) <= 1024 && !strings.ContainsAny(id, "/\\\x00\r\n") && id != "." && id != ".."
}

func (g *GogStorage) resolve(ctx context.Context, loc StorageLocation) (StorageLocation, error) {
	if g == nil || g.Service == nil {
		return loc, ErrStorageReadOnly
	}
	loc = g.cached(loc)
	if loc.ItemID != "" {
		return loc, nil
	}
	if loc.Path == "" {
		loc.ItemID = "root"
		return loc, nil
	}
	parent, e := g.parent(ctx, loc)
	if e != nil {
		return loc, e
	}
	entries, e := g.List(ctx, parent)
	if e != nil {
		return loc, e
	}
	name := loc.Name
	if name == "" {
		name = path.Base(loc.Path)
	}
	var found *StorageEntry
	for i := range entries {
		if entries[i].Name == name {
			if found != nil {
				return loc, ErrStorageConflict
			}
			found = &entries[i]
		}
	}
	if found == nil {
		return loc, ErrStorageNotFound
	}
	loc.ItemID = found.ItemID
	loc.ParentID = parent.ItemID
	loc.Name = found.Name
	g.remember(loc)
	return loc, nil
}
func (g *GogStorage) parent(ctx context.Context, loc StorageLocation) (StorageLocation, error) {
	p := loc
	p.Path = path.Dir(loc.Path)
	if p.Path == "." {
		p.Path = ""
	}
	p.ItemID = loc.ParentID
	p.ParentID = ""
	p.Name = ""
	return g.resolve(ctx, p)
}
func (g *GogStorage) List(ctx context.Context, loc StorageLocation) ([]StorageEntry, error) {
	loc, e := g.resolve(ctx, loc)
	if e != nil {
		return nil, e
	}
	out := []StorageEntry{}
	page := ""
	seen := map[string]bool{}
	for {
		args := map[string]any{"parent": loc.ItemID, "max": 100}
		if page != "" {
			args["page"] = page
		}
		data, e := g.call(ctx, loc, "list", args, nil)
		if e != nil {
			return nil, e
		}
		var list struct {
			Files []gogDriveFile `json:"files"`
			Next  string         `json:"nextPageToken"`
		}
		if e = json.Unmarshal(data, &list); e != nil {
			return nil, e
		}
		for _, file := range list.Files {
			if !validGogObjectID(file.ID) || seen[file.ID] {
				return nil, fmt.Errorf("invalid Drive listing")
			}
			seen[file.ID] = true
			child := loc
			child.Path = path.Join(loc.Path, file.ID)
			child.ItemID = file.ID
			child.ParentID = loc.ItemID
			out = append(out, g.entry(child, file))
		}
		if len(out) > 10000 {
			return nil, fmt.Errorf("storage listing exceeds 10000 entries")
		}
		if list.Next == "" {
			return out, nil
		}
		if list.Next == page {
			return nil, fmt.Errorf("repeated Drive page")
		}
		page = list.Next
	}
}
func (g *GogStorage) Stat(ctx context.Context, loc StorageLocation) (StorageEntry, error) {
	loc, e := g.resolve(ctx, loc)
	if e != nil {
		return StorageEntry{}, e
	}
	data, e := g.call(ctx, loc, "get", map[string]any{"file_id": loc.ItemID}, nil)
	if e != nil {
		return StorageEntry{}, e
	}
	f, e := decodeGogFile(data)
	if e != nil {
		return StorageEntry{}, e
	}
	return g.entry(loc, f), nil
}
func (g *GogStorage) Open(ctx context.Context, loc StorageLocation, version string) (io.ReadCloser, error) {
	e, err := g.Stat(ctx, loc)
	if err != nil {
		return nil, err
	}
	if version != "" && e.Version != version {
		return nil, ErrStorageConflict
	}
	if strings.HasPrefix(e.ContentType, "application/vnd.google-apps.") {
		return nil, fmt.Errorf("gog_office_import_required")
	}
	var buf bytes.Buffer
	_, err = g.call(ctx, loc, "download", map[string]any{"file_id": e.ItemID}, &gog.Files{Output: &buf})
	if err != nil {
		return nil, err
	}
	after, err := g.Stat(ctx, loc)
	if err != nil || after.Version != e.Version || int64(buf.Len()) != e.Size {
		return nil, ErrStorageConflict
	}
	return io.NopCloser(&buf), nil
}
func (g *GogStorage) Put(ctx context.Context, loc StorageLocation, body io.Reader, version string) (StorageEntry, error) {
	args := map[string]any{}
	operation := "upload"
	if version != "" {
		e, err := g.Stat(ctx, loc)
		if err != nil {
			return StorageEntry{}, err
		}
		if e.Version != version || strings.HasPrefix(e.ContentType, "application/vnd.google-apps.") {
			return StorageEntry{}, ErrStorageConflict
		}
		v, err := strconv.ParseInt(strings.TrimPrefix(version, "gog-"), 10, 64)
		if err != nil {
			return StorageEntry{}, ErrStorageConflict
		}
		operation = "replace"
		args["replace"] = e.ItemID
		args["if_version"] = json.Number(strconv.FormatInt(v, 10))
	} else {
		if _, err := g.Stat(ctx, loc); err != ErrStorageNotFound {
			if err == nil {
				err = ErrStorageConflict
			}
			return StorageEntry{}, err
		}
		parent, err := g.parent(ctx, loc)
		if err != nil {
			return StorageEntry{}, err
		}
		args["parent"] = parent.ItemID
		args["name"] = path.Base(loc.Path)
	}
	data, err := g.call(ctx, loc, operation, args, &gog.Files{Input: body, Name: path.Base(loc.Path)})
	if err != nil {
		return StorageEntry{}, err
	}
	f, err := decodeGogFile(data)
	if err != nil {
		return StorageEntry{}, err
	}
	loc.ItemID = f.ID
	g.remember(loc)
	return g.Stat(ctx, loc)
}
func (g *GogStorage) Mkdir(ctx context.Context, loc StorageLocation) error {
	p, e := g.parent(ctx, loc)
	if e != nil {
		return e
	}
	data, e := g.call(ctx, loc, "mkdir", map[string]any{"parent": p.ItemID, "name": path.Base(loc.Path)}, nil)
	if e != nil {
		return e
	}
	f, e := decodeGogFile(data)
	if e == nil {
		loc.ItemID = f.ID
		g.remember(loc)
	}
	return e
}
func (g *GogStorage) Trash(ctx context.Context, loc StorageLocation) error {
	loc, e := g.resolve(ctx, loc)
	if e != nil {
		return e
	}
	_, e = g.call(ctx, loc, "trash", map[string]any{"file_id": loc.ItemID}, nil)
	return e
}

// Copy creates a distinct Drive object. No source is removed on a failed check.
func (g *GogStorage) Copy(ctx context.Context, from, to StorageLocation, version string) error {
	source, err := g.Stat(ctx, from)
	if err != nil {
		return err
	}
	if source.Version != version {
		return ErrStorageConflict
	}
	if _, err = g.Stat(ctx, to); err != ErrStorageNotFound {
		if err == nil {
			return ErrStorageConflict
		}
		return err
	}
	parent, err := g.parent(ctx, to)
	if err != nil {
		return err
	}
	data, err := g.call(ctx, from, "copy", map[string]any{"file_id": source.ItemID, "name": path.Base(to.Path), "parent": parent.ItemID}, nil)
	if err != nil {
		return err
	}
	created, err := decodeGogFile(data)
	if err != nil {
		return err
	}
	to.ItemID = created.ID
	g.remember(to)
	after, err := g.Stat(ctx, from)
	if err != nil || after.Version != source.Version {
		return fmt.Errorf("storage_outcome_unknown: source changed while Google copied it")
	}
	_, err = g.Stat(ctx, to)
	return err
}
func (g *GogStorage) Move(ctx context.Context, from, to StorageLocation, version string) error {
	f, e := g.Stat(ctx, from)
	if e != nil {
		return e
	}
	if f.Version != version {
		return ErrStorageConflict
	}
	p, e := g.parent(ctx, to)
	if e != nil {
		return e
	}
	if f.ParentID != p.ItemID {
		if _, e = g.call(ctx, from, "move", map[string]any{"file_id": f.ItemID, "parent": p.ItemID}, nil); e != nil {
			return e
		}
	}
	if f.Name != path.Base(to.Path) {
		_, e = g.call(ctx, from, "rename", map[string]any{"file_id": f.ItemID, "new_name": path.Base(to.Path)}, nil)
	}
	return e
}
func GogStorageAccounts(cfg *config.Config) []map[string]any {
	out := []map[string]any{}
	if !cfg.Gog.Enabled {
		return out
	}
	for _, a := range cfg.Gog.Accounts {
		g := a.Services["drive"]
		if a.Enabled && a.DesktopAccess && g.Read {
			out = append(out, map[string]any{"id": a.ID, "name": a.Name, "write": !cfg.Gog.ReadOnly && (g.Create || g.Update || g.Delete)})
		}
	}
	return out
}
