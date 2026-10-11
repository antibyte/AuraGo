package testutil

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path"
	"sort"
	"strings"
	"sync"
	"testing"
)

// StorageFixture implements the wire contracts used by the Desktop adapters.
// Map keys are "dav/path" or "mount-id/path"; directories contain nil bytes.
type StorageFixture struct {
	Server              *httptest.Server
	Mu                  sync.Mutex
	Files               map[string][]byte
	Sizes               map[string]int64
	Requests            []string
	FailWrite, FailRead string
	CorruptWrite        bool
	NoVersion           bool
	AfterWrite          func(map[string][]byte)
}

func NewStorageFixture(t *testing.T) *StorageFixture {
	t.Helper()
	f := &StorageFixture{Files: map[string][]byte{"dav/": nil, "primary/": nil, "shared/": nil, "readonly/": nil}}
	f.Server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.Server.Close)
	return f
}

func storageHash(data []byte, dav bool) string {
	h := fmt.Sprintf("%x", sha256.Sum256(data))
	if dav {
		return `"` + h + `"`
	}
	return h
}

func (f *StorageFixture) serve(w http.ResponseWriter, r *http.Request) {
	f.Mu.Lock()
	defer f.Mu.Unlock()
	f.Requests = append(f.Requests, r.Method+" "+r.URL.String())
	size := func(key string, data []byte) int64 {
		if n, ok := f.Sizes[key]; ok {
			return n
		}
		return int64(len(data))
	}
	if r.URL.Path == "/api/v2/mounts" {
		io.WriteString(w, `{"mounts":[{"id":"primary","name":"Koofr","online":true,"permissions":{"READ":true,"WRITE":true}},{"id":"shared","name":"Connected Drive","online":true,"permissions":{"READ":true,"WRITE":true}},{"id":"readonly","name":"Shared photos","online":true,"permissions":{"READ":true,"WRITE":false}},{"id":"offline","name":"Offline","online":false,"permissions":{"READ":true,"WRITE":true}},{"id":"denied","name":"Private","permissions":{"READ":false,"WRITE":true}}]}`)
		return
	}
	dav := strings.HasPrefix(r.URL.Path, "/dav")
	mount, rel, op := "dav", strings.TrimPrefix(r.URL.Path, "/dav"), ""
	if !dav {
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/content"), "/")
		if len(parts) != 7 || parts[1] != "api" || parts[3] != "mounts" || parts[5] != "files" {
			http.Error(w, "route", 404)
			return
		}
		mount, rel, op = parts[4], r.URL.Query().Get("path"), parts[6]
		if !strings.HasPrefix(rel, "/") {
			http.Error(w, "absolute Koofr path required", 400)
			return
		}
	}
	rel = strings.Trim(rel, "/")
	key := mount + "/" + rel
	data, exists := f.Files[key]
	version := storageHash(data, dav)
	metadata := func(name string, data []byte) map[string]interface{} {
		kind := "file"
		if data == nil {
			kind = "dir"
		}
		hash := storageHash(data, false)
		if f.NoVersion {
			hash = ""
		}
		return map[string]interface{}{"name": path.Base(name), "type": kind, "size": size(name, data), "modified": 1234567890000, "hash": hash, "contentType": "application/octet-stream"}
	}
	switch {
	case r.Method == "PROPFIND":
		if !exists {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(207)
		io.WriteString(w, `<d:multistatus xmlns:d="DAV:">`)
		keys := []string{key}
		if r.Header.Get("Depth") == "1" {
			for k := range f.Files {
				if k != key && strings.TrimSuffix(path.Dir(k), "/") == strings.TrimSuffix(key, "/") {
					keys = append(keys, k)
				}
			}
		}
		sort.Strings(keys)
		for _, k := range keys {
			content := f.Files[k]
			resource := ""
			if content == nil {
				resource = "<d:collection/>"
			}
			href := (&url.URL{Path: "/" + k}).EscapedPath()
			fmt.Fprintf(w, `<d:response><d:href>%s</d:href><d:propstat><d:status>HTTP/1.1 200 OK</d:status><d:prop><d:displayname>display-only</d:displayname><d:getcontentlength>%d</d:getcontentlength><d:resourcetype>%s</d:resourcetype><d:getetag>%s</d:getetag><d:getlastmodified>Fri, 13 Feb 2009 23:31:30 GMT</d:getlastmodified></d:prop></d:propstat></d:response>`, html.EscapeString(href), size(k, content), resource, html.EscapeString(storageHash(content, true)))
		}
		io.WriteString(w, `</d:multistatus>`)
	case op == "list":
		if !exists {
			http.NotFound(w, r)
			return
		}
		files := []interface{}{}
		for k, v := range f.Files {
			if k != key && path.Dir(k) == strings.TrimSuffix(key, "/") {
				files = append(files, metadata(k, v))
			}
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"files": files})
	case op == "info":
		if !exists {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(metadata(key, data))
	case r.Method == "GET":
		if key == f.FailRead {
			http.Error(w, "network failure", 502)
			return
		}
		if !exists {
			http.NotFound(w, r)
			return
		}
		if v := r.Header.Get("If-Match"); v != "" && v != version {
			w.WriteHeader(412)
			return
		}
		w.Write(data)
	case r.Method == "MKCOL" || op == "folder":
		if !dav {
			var body struct{ Name string }
			json.NewDecoder(r.Body).Decode(&body)
			key = path.Join(key, body.Name)
			_, exists = f.Files[key]
		}
		if exists {
			w.WriteHeader(409)
			return
		}
		f.Files[key] = nil
		w.WriteHeader(201)
	case r.Method == "PUT" && dav || op == "put":
		var source io.Reader = r.Body
		if !dav {
			if r.URL.Query().Get("autorename") != "false" {
				http.Error(w, "autorename must be disabled", 400)
				return
			}
			key = path.Join(key, r.URL.Query().Get("filename"))
			data, exists = f.Files[key]
			version = storageHash(data, false)
			mr, err := r.MultipartReader()
			if err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			part, err := mr.NextPart()
			if err != nil || part.FormName() != "file" {
				http.Error(w, "file part", 400)
				return
			}
			source = part
		}
		if key == f.FailWrite {
			http.Error(w, "network failure", 502)
			return
		}
		condition := r.Header.Get("If-Match")
		if !dav {
			condition = r.URL.Query().Get("overwriteIfHash")
		}
		if exists && (condition == "" || condition != version) || !exists && condition != "" {
			w.WriteHeader(412)
			return
		}
		if dav && condition == "" && r.Header.Get("If-None-Match") != "*" {
			http.Error(w, "create precondition required", 400)
			return
		}
		body, err := io.ReadAll(source)
		if err != nil {
			return
		}
		if f.CorruptWrite {
			body = append([]byte("corrupt"), body...)
		}
		f.Files[key] = append([]byte{}, body...)
		if f.AfterWrite != nil {
			f.AfterWrite(f.Files)
		}
		json.NewEncoder(w).Encode(metadata(key, body))
	case r.Method == "DELETE":
		if !exists {
			http.NotFound(w, r)
			return
		}
		condition := r.Header.Get("If-Match")
		if !dav {
			condition = r.URL.Query().Get("removeIfHash")
		}
		if condition != "" && condition != version {
			w.WriteHeader(412)
			return
		}
		_, emptyOnly := r.URL.Query()["removeIfEmpty"]
		for k := range f.Files {
			if strings.HasPrefix(k, key+"/") && emptyOnly {
				w.WriteHeader(409)
				return
			}
		}
		for k := range f.Files {
			if k == key || strings.HasPrefix(k, key+"/") {
				delete(f.Files, k)
			}
		}
		w.WriteHeader(204)
	case r.Method == "MOVE" || op == "move":
		if !exists {
			http.NotFound(w, r)
			return
		}
		var dest string
		if dav {
			u, _ := url.Parse(r.Header.Get("Destination"))
			dest = strings.TrimPrefix(u.Path, "/")
			if r.Header.Get("Overwrite") != "F" {
				http.Error(w, "overwrite forbidden", 400)
				return
			}
		} else {
			var body struct {
				ToMountID string `json:"toMountId"`
				ToPath    string `json:"toPath"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			dest = body.ToMountID + "/" + strings.TrimPrefix(body.ToPath, "/")
		}
		if _, ok := f.Files[dest]; ok {
			w.WriteHeader(409)
			return
		}
		replacements := map[string][]byte{}
		for k, v := range f.Files {
			if k == key || strings.HasPrefix(k, key+"/") {
				replacements[dest+strings.TrimPrefix(k, key)] = v
				delete(f.Files, k)
			}
		}
		for k, v := range replacements {
			f.Files[k] = v
		}
		w.WriteHeader(200)
	default:
		http.Error(w, "unsupported storage operation", 400)
	}
}
