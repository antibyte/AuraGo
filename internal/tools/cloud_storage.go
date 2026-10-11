package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
)

// StorageLocation never treats a remote path as a local filename or a URL.
type StorageLocation struct {
	AccountID string `json:"account_id,omitempty"`
	ItemID    string `json:"item_id,omitempty"`
	ParentID  string `json:"parent_id,omitempty"`
	Name      string `json:"name,omitempty"`
	Provider  string `json:"provider"`
	MountID   string `json:"mount_id,omitempty"`
	Path      string `json:"path"`
}

type StorageEntry struct {
	ItemID      string `json:"item_id,omitempty"`
	ParentID    string `json:"parent_id,omitempty"`
	Name        string `json:"name"`
	Path        string `json:"path"`
	Type        string `json:"type"`
	Size        int64  `json:"size"`
	Modified    string `json:"modified,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	Version     string `json:"version,omitempty"`
	ReadOnly    bool   `json:"readonly"`
}

type StorageMount struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Online *bool  `json:"online,omitempty"`
	Read   bool   `json:"read"`
	Write  bool   `json:"write"`
}

var ErrStorageConflict = errors.New("storage_conflict")
var ErrStorageReadOnly = errors.New("storage_readonly")
var ErrStorageNotFound = errors.New("storage_not_found")
var ErrStorageSourceRetained = errors.New("storage_source_retained")

// CloudStorage supplies typed, bounded binary operations alongside the agent tools.
// The server owns configuration, mount authorization and live revocation.
type CloudStorage struct {
	Gog    *GogStorage
	WebDAV WebDAVConfig
	Koofr  KoofrConfig
}

func ValidateStorageLocation(loc StorageLocation) error {
	if loc.Provider == "gog" {
		if !config.ValidGogAccountID(loc.AccountID) || loc.MountID != "" || loc.ItemID != "" && !validGogObjectID(loc.ItemID) || loc.ParentID != "" && !validGogObjectID(loc.ParentID) {
			return fmt.Errorf("invalid Google storage reference")
		}
		if strings.HasPrefix(loc.Path, "/") || strings.Contains(loc.Path, ":") {
			return fmt.Errorf("invalid Google storage path")
		}
		_, err := webdavPathSuffix(loc.Path)
		return err
	}
	if loc.AccountID != "" || loc.ItemID != "" || loc.ParentID != "" {
		return fmt.Errorf("unexpected Google storage identity")
	}
	if loc.Provider != "local" && loc.Provider != "webdav" && loc.Provider != "koofr" {
		return fmt.Errorf("invalid storage provider")
	}
	if loc.Provider == "koofr" && (loc.MountID == "" || loc.MountID == "." || loc.MountID == ".." || strings.ContainsAny(loc.MountID, "/\\\x00\r\n") || len(loc.MountID) > 512) {
		return fmt.Errorf("invalid storage mount")
	}
	if loc.Provider != "koofr" && loc.MountID != "" {
		return fmt.Errorf("unexpected storage mount")
	}
	if strings.HasPrefix(loc.Path, "/") || strings.HasSuffix(loc.Path, "/") || strings.Contains(loc.Path, ":") {
		return fmt.Errorf("storage path must be relative")
	}
	_, err := webdavPathSuffix(loc.Path)
	return err
}

func storageHTTPError(code int) error {
	switch code {
	case 404:
		return ErrStorageNotFound
	case 409, 412:
		return ErrStorageConflict
	case 401, 403:
		return ErrStorageReadOnly
	default:
		return fmt.Errorf("storage provider returned HTTP %d", code)
	}
}

func (c CloudStorage) request(ctx context.Context, loc StorageLocation, method, operation string, body io.Reader, headers map[string]string) (*http.Response, error) {
	if err := ValidateStorageLocation(loc); err != nil {
		return nil, err
	}
	if loc.Provider == "webdav" {
		if c.WebDAV.ReadOnly && method != "GET" && method != "HEAD" && method != "PROPFIND" {
			return nil, ErrStorageReadOnly
		}
		u, err := webdavURL(c.WebDAV, loc.Path)
		if err != nil {
			return nil, err
		}
		if err := security.ValidateHTTPBaseURL(c.WebDAV.URL); err != nil {
			return nil, err
		}
		resp, err := webdavRequestContext(ctx, c.WebDAV, method, u, body, headers)
		return storageDefiniteResponse(resp, err, method)
	}
	if loc.Provider != "koofr" {
		return nil, fmt.Errorf("not a cloud location")
	}
	if c.Koofr.ReadOnly && method != "GET" && method != "HEAD" {
		return nil, ErrStorageReadOnly
	}
	base := strings.TrimRight(c.Koofr.BaseURL, "/")
	if base == "" {
		base = "https://app.koofr.net"
	}
	if err := security.ValidateHTTPBaseURL(base); err != nil {
		return nil, err
	}
	u := base + operation
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return nil, fmt.Errorf("create storage request: %w", err)
	}
	security.RegisterSensitive(c.Koofr.AppPassword)
	req.SetBasicAuth(c.Koofr.Username, c.Koofr.AppPassword)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := koofrHTTPClient.Do(req)
	return storageDefiniteResponse(resp, err, method)
}

func storageDefiniteResponse(resp *http.Response, err error, method string) (*http.Response, error) {
	if err == nil && (resp.StatusCode == http.StatusAccepted || resp.StatusCode == http.StatusMultiStatus && method != "PROPFIND") {
		resp.Body.Close()
		return nil, fmt.Errorf("storage provider returned an uncertain or partial result; source retained")
	}
	return resp, err
}

func koofrStorageRoute(loc StorageLocation, operation string, content bool) string {
	prefix := ""
	if content {
		prefix = "/content"
	}
	return prefix + "/api/v2/mounts/" + url.PathEscape(loc.MountID) + "/files/" + operation + "?path=" + url.QueryEscape("/"+loc.Path)
}

func readStorageResponse(resp *http.Response, err error) ([]byte, error) {
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, storageHTTPError(resp.StatusCode)
	}
	return readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
}

func (c CloudStorage) Mounts(ctx context.Context) ([]StorageMount, error) {
	loc := StorageLocation{Provider: "koofr", MountID: "mounts"}
	data, err := readStorageResponse(c.request(ctx, loc, "GET", "/api/v2/mounts", nil, nil))
	if err != nil {
		return nil, err
	}
	type mount struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Online      *bool  `json:"online"`
		Permissions struct {
			Read  bool `json:"READ"`
			Write bool `json:"WRITE"`
		} `json:"permissions"`
	}
	var envelope struct {
		Mounts []mount `json:"mounts"`
	}
	if err = json.Unmarshal(data, &envelope); err != nil {
		if err = json.Unmarshal(data, &envelope.Mounts); err != nil {
			return nil, fmt.Errorf("decode storage mounts: %w", err)
		}
	}
	result := make([]StorageMount, 0, len(envelope.Mounts))
	seen := make(map[string]bool)
	for _, m := range envelope.Mounts {
		if m.ID == "" || !m.Permissions.Read {
			continue
		}
		if err := ValidateStorageLocation(StorageLocation{Provider: "koofr", MountID: m.ID}); err != nil {
			return nil, err
		}
		if seen[m.ID] {
			return nil, fmt.Errorf("duplicate storage mount")
		}
		seen[m.ID] = true
		result = append(result, StorageMount{ID: m.ID, Name: m.Name, Online: m.Online, Read: true, Write: m.Permissions.Write && !c.Koofr.ReadOnly})
	}
	return result, nil
}

func (c CloudStorage) List(ctx context.Context, loc StorageLocation) ([]StorageEntry, error) {
	if loc.Provider == "gog" {
		return c.Gog.List(ctx, loc)
	}
	if loc.Provider == "webdav" {
		return c.davEntries(ctx, loc, "1")
	}
	data, err := readStorageResponse(c.request(ctx, loc, "GET", koofrStorageRoute(loc, "list", false), nil, nil))
	if err != nil {
		return nil, err
	}
	var result struct {
		Files []koofrStorageEntry `json:"files"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decode storage listing: %w", err)
	}
	entries := make([]StorageEntry, 0, len(result.Files))
	seen := make(map[string]bool)
	if len(result.Files) > 10000 {
		return nil, fmt.Errorf("storage listing exceeds 10000 entries")
	}
	for _, f := range result.Files {
		if err := f.validate(); err != nil {
			return nil, err
		}
		if seen[f.Name] {
			return nil, fmt.Errorf("duplicate provider filename")
		}
		seen[f.Name] = true
		if f.Name == "" || strings.ContainsAny(f.Name, "/\\") || f.Name == "." || f.Name == ".." {
			return nil, fmt.Errorf("invalid provider filename")
		}
		e := f.entry(path.Join(loc.Path, f.Name), c.Koofr.ReadOnly)
		if err := ValidateStorageLocation(StorageLocation{Provider: loc.Provider, MountID: loc.MountID, Path: e.Path}); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func (c CloudStorage) Stat(ctx context.Context, loc StorageLocation) (StorageEntry, error) {
	if loc.Provider == "gog" {
		return c.Gog.Stat(ctx, loc)
	}
	if loc.Provider == "webdav" {
		entries, err := c.davEntries(ctx, loc, "0")
		if err != nil {
			return StorageEntry{}, err
		}
		if len(entries) != 1 {
			return StorageEntry{}, fmt.Errorf("invalid WebDAV file metadata")
		}
		return entries[0], nil
	}
	data, err := readStorageResponse(c.request(ctx, loc, "GET", koofrStorageRoute(loc, "info", false), nil, nil))
	if err != nil {
		return StorageEntry{}, err
	}
	var f koofrStorageEntry
	if err = json.Unmarshal(data, &f); err != nil {
		return StorageEntry{}, fmt.Errorf("decode storage metadata: %w", err)
	}
	if err = f.validate(); err != nil {
		return StorageEntry{}, err
	}
	return f.entry(loc.Path, c.Koofr.ReadOnly), nil
}

type koofrStorageEntry struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Size        int64  `json:"size"`
	Modified    int64  `json:"modified"`
	Hash        string `json:"hash"`
	ContentType string `json:"contentType"`
}

func (f koofrStorageEntry) validate() error {
	if f.Type != "file" && f.Type != "dir" && f.Type != "directory" || f.Size < 0 {
		return fmt.Errorf("invalid provider file metadata")
	}
	return nil
}

func (f koofrStorageEntry) entry(p string, readonly bool) StorageEntry {
	kind := "file"
	if f.Type == "dir" || f.Type == "directory" {
		kind = "directory"
	}
	version := ""
	if f.Hash != "" {
		version = f.Hash
	}
	return StorageEntry{Name: path.Base(p), Path: p, Type: kind, Size: f.Size, Modified: time.UnixMilli(f.Modified).UTC().Format(time.RFC3339), ContentType: f.ContentType, Version: version, ReadOnly: readonly}
}

func (c CloudStorage) Open(ctx context.Context, loc StorageLocation, version string) (io.ReadCloser, error) {
	if loc.Provider == "gog" {
		return c.Gog.Open(ctx, loc, version)
	}
	headers := map[string]string{}
	if loc.Provider == "webdav" && version != "" {
		headers["If-Match"] = version
	}
	resp, err := c.request(ctx, loc, "GET", koofrStorageRoute(loc, "get", true), nil, headers)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, storageHTTPError(resp.StatusCode)
	}
	return resp.Body, nil
}
