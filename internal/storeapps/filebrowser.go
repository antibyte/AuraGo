package storeapps

import (
	"context"
	"errors"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
)

type filePermissions struct {
	View     bool `json:"view"`
	Download bool `json:"download"`
	Modify   bool `json:"modify"`
	Create   bool `json:"create"`
	Delete   bool `json:"delete"`
}
type FileBrowserEntry struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Type     string `json:"type"`
	Size     int64  `json:"size"`
	Modified string `json:"modified"`
	Source   string `json:"source,omitempty"`
}

func (c Connection) FileBrowserProbe(ctx context.Context) error {
	if c.Source == "" || strings.ContainsAny(c.Source, ",:\\/?#\x00\r\n") {
		return errors.New("Select the FileBrowser source backed by the Store /folder volume")
	}
	var user struct {
		Username    string `json:"username"`
		Permissions struct {
			Admin bool `json:"admin"`
			API   bool `json:"api"`
			Share bool `json:"share"`
		} `json:"permissions"`
		Scopes []struct {
			Name        string           `json:"name"`
			Permissions *filePermissions `json:"permissions"`
		} `json:"scopes"`
	}
	if err := c.request(ctx, "GET", "/api/users?username=self", nil, false, &user); err != nil {
		return err
	}
	if user.Username == "" || (c.Username != "" && user.Username != c.Username) || user.Permissions.Admin || user.Permissions.Share || !user.Permissions.API || len(user.Scopes) != 1 || user.Scopes[0].Name != c.Source {
		return errors.New("Use a non-admin FileBrowser API user limited to the selected Store source, without sharing")
	}
	p := user.Scopes[0].Permissions
	if p == nil || !p.View || !p.Download || p.Modify || p.Create || p.Delete {
		return errors.New("The selected FileBrowser source must grant only View and Download; update the app if source permissions are unavailable")
	}
	return nil
}

func fileBrowserPath(value string) (string, error) {
	if len(value) > 2048 || strings.ContainsAny(value, "\\\x00\r\n:") {
		return "", errors.New("Invalid FileBrowser path")
	}
	for _, part := range strings.Split(value, "/") {
		if part == ".." {
			return "", errors.New("FileBrowser traversal is not allowed")
		}
	}
	return path.Clean("/" + value), nil
}

func (c Connection) fileBrowserInfo(ctx context.Context, name string) (FileBrowserEntry, []FileBrowserEntry, error) {
	var result struct {
		FileBrowserEntry
		Files   []FileBrowserEntry `json:"files"`
		Folders []FileBrowserEntry `json:"folders"`
	}
	q := url.Values{"source": {c.Source}, "path": {name}, "content": {"false"}, "metadata": {"false"}, "skipExtendedAttrs": {"true"}}
	err := c.request(ctx, "GET", "/api/resources?"+q.Encode(), nil, false, &result)
	return result.FileBrowserEntry, append(result.Folders, result.Files...), err
}

func (c Connection) FileBrowserList(ctx context.Context, name string, offset, limit int) (any, error) {
	if err := c.FileBrowserProbe(ctx); err != nil {
		return nil, err
	}
	name, err := fileBrowserPath(name)
	if err != nil {
		return nil, err
	}
	info, items, err := c.fileBrowserInfo(ctx, name)
	if err != nil {
		return nil, err
	}
	if info.Type != "directory" {
		return nil, errors.New("Select a FileBrowser directory")
	}
	for i := range items {
		if path.Base(items[i].Name) != items[i].Name || items[i].Name == ".." {
			return nil, errors.New("Invalid FileBrowser listing")
		}
		items[i].Path = path.Join(name, items[i].Name)
		items[i].Source = c.Source
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Path < items[j].Path })
	start := min(offset, len(items))
	end := min(start+limit, len(items))
	return map[string]any{"entries": items[start:end], "has_more": end < len(items), "next_offset": end}, nil
}

func (c Connection) FileBrowserSearch(ctx context.Context, query string, limit int) (any, error) {
	if err := c.FileBrowserProbe(ctx); err != nil {
		return nil, err
	}
	if query == "" || len(query) > 256 {
		return nil, errors.New("Enter a search term of at most 256 bytes")
	}
	q := url.Values{"terms": {query}, "sources": {c.Source}, "scope": {c.Source + ":/"}, "useWildcard": {"false"}, "limit": {strconv.Itoa(limit)}}
	var items []FileBrowserEntry
	if err := c.request(ctx, "GET", "/api/tools/search?"+q.Encode(), nil, false, &items); err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].Source != c.Source {
			return nil, errors.New("FileBrowser returned a different source")
		}
		p, e := fileBrowserPath(items[i].Path)
		if e != nil {
			return nil, e
		}
		items[i].Path = p
	}
	return map[string]any{"entries": items[:min(limit, len(items))], "limit": limit}, nil
}

func (c Connection) FileBrowserRead(ctx context.Context, name string, maxBytes int64) ([]byte, string, error) {
	if err := c.FileBrowserProbe(ctx); err != nil {
		return nil, "", err
	}
	name, err := fileBrowserPath(name)
	if err != nil {
		return nil, "", err
	}
	info, _, err := c.fileBrowserInfo(ctx, name)
	if err != nil {
		return nil, "", err
	}
	if info.Type == "directory" || info.Size < 0 || info.Size > maxBytes {
		return nil, "", errors.New("FileBrowser file exceeds the size limit or is a directory")
	}
	q := url.Values{"source": {c.Source}, "file": {name}}
	data, err := c.exchange(ctx, "GET", "/api/resources/download?"+q.Encode(), nil, "", "Authorization", "Bearer "+c.Credential, false, maxBytes)
	return data, path.Base(name), err
}
