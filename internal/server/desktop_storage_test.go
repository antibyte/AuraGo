package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/desktop"
	"aurago/internal/testutil"
	"aurago/internal/tools"
)

func newStorageTestSession(t *testing.T) (*desktopStorageSession, *testutil.StorageFixture) {
	t.Helper()
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")
	f := testutil.NewStorageFixture(t)
	s := newDesktopFilesystemTestServer(t)
	s.Cfg.WebDAV.Enabled = true
	s.Cfg.WebDAV.ShowInFileManager = true
	s.Cfg.WebDAV.URL = f.Server.URL + "/dav"
	s.Cfg.Koofr.Enabled = true
	s.Cfg.Koofr.ShowInFileManager = true
	s.Cfg.Koofr.BaseURL = f.Server.URL
	s.Cfg.VirtualDesktop.MaxFileSizeMB = 1
	svc, _, err := s.getDesktopService(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return &desktopStorageSession{s: s, svc: svc, cloud: desktopStorageCloud(s.Cfg), maxBytes: 1 << 20, ctx: context.Background()}, f
}

func storageTestLocation(provider, p string) tools.StorageLocation {
	if provider == "koofr-primary" {
		return tools.StorageLocation{Provider: "koofr", MountID: "primary", Path: p}
	}
	loc := tools.StorageLocation{Provider: provider, Path: p}
	if provider == "koofr" {
		loc.MountID = "shared"
	}
	return loc
}

func seedStorageTree(t *testing.T, d *desktopStorageSession, f *testutil.StorageFixture, loc tools.StorageLocation, data []byte) {
	t.Helper()
	if loc.Provider == "local" {
		root := filepath.Join(d.svc.Config().WorkspaceDir, filepath.FromSlash(loc.Path))
		if err := os.MkdirAll(filepath.Join(root, "empty"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "bin ü #%.dat"), data, 0600); err != nil {
			t.Fatal(err)
		}
		return
	}
	prefix := "dav/"
	if loc.Provider == "koofr" {
		prefix = loc.MountID + "/"
	}
	f.Mu.Lock()
	defer f.Mu.Unlock()
	f.Files[prefix+loc.Path] = nil
	f.Files[prefix+loc.Path+"/empty"] = nil
	f.Files[prefix+loc.Path+"/bin ü #%.dat"] = data
}

func TestDesktopStorageTransfersAllDirections(t *testing.T) {
	for _, mode := range []string{"copy", "move"} {
		for _, from := range []string{"local", "webdav", "koofr", "koofr-primary"} {
			for _, to := range []string{"local", "webdav", "koofr", "koofr-primary"} {
				t.Run(mode+"_"+from+"_to_"+to, func(t *testing.T) {
					d, f := newStorageTestSession(t)
					data := bytes.Repeat([]byte{0, 255, 14, 129}, 10000)
					source := storageTestLocation(from, "source")
					dest := storageTestLocation(to, "destination")
					seedStorageTree(t, d, f, source, data)
					req := desktopStorageRequest{Location: source, Destination: dest, Mode: mode}
					items, err := d.transferPlan(req)
					if err != nil {
						t.Fatal(err)
					}
					if len(items) != 3 {
						t.Fatalf("plan has %d entries", len(items))
					}
					err = d.transfer(req, items, func(int) error { return nil })
					retainsDirectories := mode == "move" && from == "webdav" && from != to
					if retainsDirectories {
						if !errors.Is(err, tools.ErrStorageSourceRetained) {
							t.Fatalf("unsafe directory cleanup: %v", err)
						}
					} else if err != nil {
						t.Fatal(err)
					}
					file := dest
					file.Path += "/bin ü #%.dat"
					in, err := d.open(file, "")
					if err != nil {
						t.Fatal(err)
					}
					got, err := io.ReadAll(in)
					in.Close()
					if err != nil || !bytes.Equal(got, data) {
						t.Fatal("binary mismatch")
					}
					empty := dest
					empty.Path += "/empty"
					children, err := d.list(empty)
					if err != nil || len(children) != 0 {
						t.Fatalf("empty directory lost: %v %v", children, err)
					}
					if mode == "copy" {
						if _, err = d.transferPlan(req); !errors.Is(err, tools.ErrStorageConflict) {
							t.Fatalf("existing directory was not a conflict: %v", err)
						}
					} else {
						file = source
						file.Path += "/bin ü #%.dat"
						if _, err = d.stat(file); !errors.Is(err, tools.ErrStorageNotFound) && !errors.Is(err, os.ErrNotExist) {
							t.Fatalf("copied source file retained: %v", err)
						}
						if _, err = d.stat(source); !retainsDirectories && !errors.Is(err, tools.ErrStorageNotFound) && !errors.Is(err, os.ErrNotExist) {
							t.Fatalf("copied source directory retained: %v", err)
						}
					}
				})
			}
		}
	}
}

func TestDesktopStorageBoundedPlainTextPreview(t *testing.T) {
	d, f := newStorageTestSession(t)
	data := bytes.Repeat([]byte("<script>active()</script>"), 10000)
	f.Files["dav/page.html"] = data
	loc := storageTestLocation("webdav", "page.html")
	for _, query := range []string{"", "?preview=text"} {
		rec := httptest.NewRecorder()
		d.serveContent(rec, httptest.NewRequest("GET", "/api/desktop/storage/content"+query, nil), loc)
		if rec.Code != 200 || !strings.Contains(rec.Header().Get("Content-Security-Policy"), "sandbox") {
			t.Fatal(rec.Code, rec.Header())
		}
		if query == "" {
			if !strings.HasPrefix(rec.Header().Get("Content-Disposition"), "attachment") || !bytes.Equal(rec.Body.Bytes(), data) {
				t.Fatal("active download rendered inline or truncated")
			}
		} else if rec.Body.Len() != 64<<10 || rec.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
			t.Fatal("preview must be bounded plain text")
		}
	}
}

func TestDesktopStorageMoveRetainsSourcesOnFailure(t *testing.T) {
	for _, failure := range []string{"write", "read", "changed", "corrupt", "cancel"} {
		t.Run(failure, func(t *testing.T) {
			d, f := newStorageTestSession(t)
			source := storageTestLocation("koofr", "source")
			dest := storageTestLocation("webdav", "destination")
			seedStorageTree(t, d, f, source, []byte("original"))
			req := desktopStorageRequest{Location: source, Destination: dest, Mode: "move"}
			items, err := d.transferPlan(req)
			if err != nil {
				t.Fatal(err)
			}
			switch failure {
			case "write":
				f.FailWrite = "dav/destination/bin ü #%.dat"
			case "read":
				f.FailRead = "shared/source/bin ü #%.dat"
			case "corrupt":
				f.CorruptWrite = true
			case "changed":
				f.AfterWrite = func(files map[string][]byte) { files["shared/source/bin ü #%.dat"] = []byte("changed") }
			case "cancel":
				ctx, cancel := context.WithCancel(d.ctx)
				cancel()
				d.ctx = ctx
			}
			if err = d.transfer(req, items, func(int) error { return nil }); err == nil {
				t.Fatal("failed transfer reported success")
			}
			f.Mu.Lock()
			defer f.Mu.Unlock()
			if _, ok := f.Files["shared/source/bin ü #%.dat"]; !ok {
				t.Fatal("failed copy deleted source")
			}
			for _, r := range f.Requests {
				if strings.HasPrefix(r, "DELETE ") {
					t.Fatalf("failure triggered deletion: %s", r)
				}
			}
		})
	}
}

func TestDesktopStorageMovesAndVersionedConflicts(t *testing.T) {
	d, f := newStorageTestSession(t)
	for _, provider := range []string{"local", "webdav", "koofr"} {
		source := storageTestLocation(provider, "source-"+provider)
		seedStorageTree(t, d, f, source, []byte("one"))
		dest := source
		dest.Path = "renamed-" + provider
		req := desktopStorageRequest{Location: source, Destination: dest, Mode: "move"}
		items, err := d.transferPlan(req)
		if err != nil {
			t.Fatal(err)
		}
		if err = d.transfer(req, items, func(int) error { return nil }); err != nil {
			t.Fatal(err)
		}
		if _, err = d.stat(source); !errors.Is(err, os.ErrNotExist) && !errors.Is(err, tools.ErrStorageNotFound) {
			t.Fatalf("old source: %v", err)
		}
	}
	from := storageTestLocation("koofr", "renamed-koofr/bin ü #%.dat")
	to := storageTestLocation("local", "moved.dat")
	req := desktopStorageRequest{Location: from, Destination: to, Mode: "move"}
	items, err := d.transferPlan(req)
	if err != nil {
		t.Fatal(err)
	}
	if err = d.transfer(req, items, func(int) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if _, err = d.stat(from); !errors.Is(err, tools.ErrStorageNotFound) {
		t.Fatalf("Koofr source not removed: %v", err)
	}
	entry, err := d.stat(to)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = d.put(to, strings.NewReader("two"), "*"); !errors.Is(err, tools.ErrStorageConflict) {
		t.Fatalf("wildcard overwrite: %v", err)
	}
	if _, err = d.put(to, strings.NewReader("two"), entry.Version); err != nil {
		t.Fatal(err)
	}
	if _, err = d.put(to, strings.NewReader("three"), entry.Version); !errors.Is(err, tools.ErrStorageConflict) {
		t.Fatalf("stale overwrite: %v", err)
	}
}

func TestDesktopStorageLimitsAndRights(t *testing.T) {
	d, f := newStorageTestSession(t)
	source := storageTestLocation("webdav", "source")
	seedStorageTree(t, d, f, source, []byte("oversize"))
	dest := storageTestLocation("koofr", "destination")
	d.maxBytes = 3
	if _, err := d.transferPlan(desktopStorageRequest{Location: source, Destination: dest, Mode: "copy"}); err == nil {
		t.Fatal("file limit ignored")
	}
	d.maxBytes = 1 << 20
	for _, p := range []string{"", "../escape", "source/child"} {
		target := source
		target.Path = p
		if _, err := d.transferPlan(desktopStorageRequest{Location: source, Destination: target, Mode: "copy"}); err == nil {
			t.Fatalf("unsafe target %q", p)
		}
	}
	for _, mount := range []string{"readonly", "offline", "denied", "unknown"} {
		loc := storageTestLocation("koofr", "a")
		loc.MountID = mount
		if err := d.authorize(loc, true); !errors.Is(err, tools.ErrStorageReadOnly) {
			t.Fatalf("mount %s: %v", mount, err)
		}
	}
	d.s.Cfg.WebDAV.ReadOnly = true
	if err := d.authorize(source, true); !errors.Is(err, tools.ErrStorageReadOnly) {
		t.Fatal("readonly ignored")
	}
	if err := d.authorize(source, false); err != nil {
		t.Fatal(err)
	}
	d.s.Cfg.WebDAV.ShowInFileManager = false
	if err := d.authorize(source, false); !errors.Is(err, tools.ErrStorageReadOnly) {
		t.Fatal("hidden source accessible")
	}
	d.s.Cfg.VirtualDesktop.ReadOnly = true
	if err := d.authorize(storageTestLocation("local", "a"), true); !errors.Is(err, tools.ErrStorageReadOnly) {
		t.Fatal("desktop readonly ignored")
	}
}

func TestDesktopStorageTreeBudgets(t *testing.T) {
	for _, kind := range []string{"entries", "depth", "bytes"} {
		t.Run(kind, func(t *testing.T) {
			d, f := newStorageTestSession(t)
			source := storageTestLocation("webdav", "source")
			dest := storageTestLocation("koofr", "dest")
			f.Files["dav/source"] = nil
			switch kind {
			case "entries":
				for i := 0; i < 10001; i++ {
					f.Files[fmt.Sprintf("dav/source/f%d", i)] = []byte{}
				}
			case "depth":
				p := "dav/source"
				for i := 0; i < 66; i++ {
					p += "/d"
					f.Files[p] = nil
				}
			case "bytes":
				d.maxBytes = 600 << 20
				f.Files["dav/source/a"] = []byte{}
				f.Files["dav/source/b"] = []byte{}
				f.Sizes = map[string]int64{"dav/source/a": 256 << 20, "dav/source/b": (256 << 20) + 1}
			}
			if _, err := d.transferPlan(desktopStorageRequest{Location: source, Destination: dest, Mode: "copy"}); err == nil {
				t.Fatalf("%s budget ignored", kind)
			}
			for _, request := range f.Requests {
				if strings.HasPrefix(request, "PUT ") || strings.HasPrefix(request, "POST ") || strings.HasPrefix(request, "DELETE ") {
					t.Fatalf("mutation during failed preflight: %s", request)
				}
			}
		})
	}
}

func TestDesktopStorageMoveKeepsUnsafeCleanup(t *testing.T) {
	d, f := newStorageTestSession(t)
	source := storageTestLocation("webdav", "source")
	dest := storageTestLocation("koofr", "dest")
	seedStorageTree(t, d, f, source, []byte("copied"))
	req := desktopStorageRequest{Location: source, Destination: dest, Mode: "move"}
	items, err := d.transferPlan(req)
	if err != nil {
		t.Fatal(err)
	}
	if err = d.transfer(req, items, func(int) error { return nil }); !errors.Is(err, tools.ErrStorageSourceRetained) {
		t.Fatalf("empty WebDAV directory cleanup: %v", err)
	}
	if _, err = d.stat(source); err != nil {
		t.Fatal("unsafe directory deleted")
	}
	// Providers without a strong file version must retain the copied source.
	file := storageTestLocation("koofr", "unversioned.bin")
	f.Files["shared/unversioned.bin"] = []byte("copied")
	f.NoVersion = true
	target := storageTestLocation("local", "unversioned.bin")
	entry, err := d.stat(file)
	if err != nil {
		t.Fatal(err)
	}
	err = d.transfer(desktopStorageRequest{Location: file, Destination: target, Mode: "move"}, []storageTransferItem{{from: file, to: target, entry: entry}}, func(int) error { return nil })
	if !errors.Is(err, tools.ErrStorageSourceRetained) {
		t.Fatalf("unversioned move: %v", err)
	}
	if _, err = d.stat(file); err != nil {
		t.Fatal("unversioned source deleted")
	}
}

func TestDesktopStorageHTTPAndLiveRevocation(t *testing.T) {
	d, f := newStorageTestSession(t)
	s := d.s
	handler := handleDesktopStorage(s)
	call := func(method, endpoint string, body io.Reader) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		handler(rec, httptest.NewRequest(method, "/api/desktop/storage/"+endpoint, body))
		return rec
	}
	if rec := call("GET", "sources", nil); rec.Code != 200 || len(f.Requests) != 0 {
		t.Fatalf("sources must be passive: %d %v", rec.Code, f.Requests)
	}
	if rec := call("GET", "mounts", nil); rec.Code != 200 || !strings.Contains(rec.Body.String(), "Connected Drive") {
		t.Fatal(rec.Body.String())
	}
	loc := storageTestLocation("webdav", "a #.dat")
	q := url.Values{"provider": {"webdav"}, "path": {loc.Path}}.Encode()
	if rec := call("POST", "upload?"+q, bytes.NewReader([]byte{0, 255, 1})); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	if rec := call("POST", "upload?"+q, strings.NewReader("overwrite")); rec.Code != 409 || !strings.Contains(rec.Body.String(), "version") {
		t.Fatalf("conflict: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call("GET", "content?"+q, nil); rec.Code != 200 || !bytes.Equal(rec.Body.Bytes(), []byte{0, 255, 1}) || rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("content contract")
	}
	body, _ := json.Marshal(desktopStorageRequest{Location: loc, Destination: storageTestLocation("koofr", "copied.dat"), Mode: "copy"})
	if rec := call("POST", "transfer", bytes.NewReader(body)); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"status":"done"`) {
		t.Fatalf("transfer response: %d %s", rec.Code, rec.Body.String())
	}
	ctx, cancel := s.storageContext(context.Background())
	defer cancel()
	cfg := *s.Cfg
	cfg.WebDAV.ShowInFileManager = false
	s.CfgMu.Lock()
	s.replaceConfigSnapshot(&cfg)
	s.CfgMu.Unlock()
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("live config did not cancel requests")
	}
	if rec := call("GET", "content?"+q, nil); rec.Code != 403 {
		t.Fatalf("revoked access: %d", rec.Code)
	}
	s.Cfg.Auth.Enabled = true
	if rec := call("GET", "sources", nil); rec.Code != 401 && rec.Code != 403 {
		t.Fatalf("admin guard: %d", rec.Code)
	}
}

func TestDesktopStorageStreamFailureRetainsSource(t *testing.T) {
	d, f := newStorageTestSession(t)
	seedStorageTree(t, d, f, storageTestLocation("koofr", "source"), []byte("data"))
	f.FailWrite = "dav/dest/bin ü #%.dat"
	req := desktopStorageRequest{Location: storageTestLocation("koofr", "source"), Destination: storageTestLocation("webdav", "dest"), Mode: "move"}
	body, _ := json.Marshal(req)
	rec := httptest.NewRecorder()
	handleDesktopStorage(d.s)(rec, httptest.NewRequest("POST", "/api/desktop/storage/transfer", bytes.NewReader(body)))
	if !strings.Contains(rec.Body.String(), `"partial":true`) || strings.Contains(rec.Body.String(), `"status":"done"`) {
		t.Fatal(rec.Body.String())
	}
	if _, err := d.stat(req.Location); err != nil {
		t.Fatalf("source lost: %v", err)
	}
}

func TestDesktopStorageProtectedLocalSources(t *testing.T) {
	d, _ := newStorageTestSession(t)
	p := "file.dat"
	if err := d.svc.WriteFileBytes(d.ctx, p, []byte("old"), desktop.SourceUser); err != nil {
		t.Fatal(err)
	}
	entry, err := d.stat(storageTestLocation("local", p))
	if err != nil {
		t.Fatal(err)
	}
	if err = d.svc.WriteFileBytes(d.ctx, p, []byte("new"), desktop.SourceUser); err != nil {
		t.Fatal(err)
	}
	if err = d.svc.RemoveStorageSource(d.ctx, p, entry.Version, false); err == nil {
		t.Fatal("changed local source deleted")
	}
	if _, err = os.Stat(filepath.Join(d.svc.Config().WorkspaceDir, p)); err != nil {
		t.Fatal(err)
	}
	if err = d.svc.RemoveStorageSource(d.ctx, "", "", true); err == nil {
		t.Fatal("root removed")
	}
}
