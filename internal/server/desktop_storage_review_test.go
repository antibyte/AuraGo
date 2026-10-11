package server

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestDesktopStorageChangedSourcePreservesReplacementTarget(t *testing.T) {
	for _, sourceProvider := range []string{"local", "webdav", "koofr"} {
		for _, targetProvider := range []string{"local", "webdav", "koofr"} {
			for _, changed := range []string{"short", "much-longer-content", "CHANGED!"} {
				t.Run(sourceProvider+"_to_"+targetProvider+"_"+changed, func(t *testing.T) {
					d, f := newStorageTestSession(t)
					source := storageTestLocation(sourceProvider, "source.bin")
					target := storageTestLocation(targetProvider, "target.bin")
					if _, err := d.put(source, bytes.NewBufferString("original"), ""); err != nil {
						t.Fatal(err)
					}
					_, err := d.put(target, bytes.NewBufferString("valuable target"), "")
					if err != nil {
						t.Fatal(err)
					}
					previous, err := d.stat(target)
					if err != nil {
						t.Fatal(err)
					}
					req := desktopStorageRequest{Location: source, Destination: target, Mode: "copy", Version: previous.Version}
					items, err := d.transferPlan(req)
					if err != nil {
						t.Fatal(err)
					}
					if sourceProvider == "local" {
						err = os.WriteFile(filepath.Join(d.svc.Config().WorkspaceDir, source.Path), []byte(changed), 0600)
						if err != nil {
							t.Fatal(err)
						}
					} else {
						key := "dav/" + source.Path
						if sourceProvider == "koofr" {
							key = "shared/" + source.Path
						}
						f.Mu.Lock()
						f.Files[key] = []byte(changed)
						f.Mu.Unlock()
					}
					if err = d.transfer(req, items, func(int) error { return nil }); err == nil {
						t.Fatal("changed source accepted")
					}
					in, err := d.open(target, "")
					if err != nil {
						t.Fatal(err)
					}
					data, err := io.ReadAll(in)
					in.Close()
					if err != nil || string(data) != "valuable target" {
						t.Fatalf("failed copy overwrote existing target: %q (%v)", data, err)
					}
				})
			}
		}
	}
}

type storageObservedUpload struct {
	io.ReadCloser
	read chan struct{}
	once sync.Once
}

func (r *storageObservedUpload) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	if n > 0 {
		r.once.Do(func() { close(r.read) })
	}
	return n, err
}

func TestDesktopStorageRevocationInterruptsPausedUpload(t *testing.T) {
	d, f := newStorageTestSession(t)
	read, finished := make(chan struct{}), make(chan struct{})
	handler := handleDesktopStorage(d.s)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(finished)
		r.Body = &storageObservedUpload{ReadCloser: r.Body, read: read}
		handler(w, r)
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/api/desktop/storage/upload?provider=webdav&path=paused.bin", reader)
	if err != nil {
		t.Fatal(err)
	}
	clientDone := make(chan struct{})
	go func() {
		defer close(clientDone)
		response, err := server.Client().Do(req)
		if err == nil {
			response.Body.Close()
		}
	}()
	defer func() { cancel(); writer.Close(); <-clientDone }()
	if _, err := writer.Write([]byte("first upload bytes")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-read:
	case <-time.After(3 * time.Second):
		t.Fatal("upload did not start")
	}
	cfg := *d.s.ConfigSnapshot()
	cfg.WebDAV.ShowInFileManager = false
	d.s.CfgMu.Lock()
	d.s.replaceConfigSnapshot(&cfg)
	d.s.CfgMu.Unlock()
	select {
	case <-finished:
	case <-time.After(3 * time.Second):
		t.Fatal("revoked upload remained blocked reading the client")
	}
	f.Mu.Lock()
	defer f.Mu.Unlock()
	if _, exists := f.Files["dav/paused.bin"]; exists {
		t.Fatal("revoked upload published a file")
	}
}

func TestDesktopStorageTruncatedDownloadFails(t *testing.T) {
	d, f := newStorageTestSession(t)
	f.Files["dav/broken.bin"] = []byte("complete content")
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/dav/broken.bin" {
			w.Header().Set("Content-Length", "16")
			_, _ = w.Write([]byte("partial"))
			return
		}
		f.Server.Config.Handler.ServeHTTP(w, r)
	}))
	defer provider.Close()
	d.cloud.WebDAV.URL = provider.URL + "/dav"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		worker := *d
		worker.ctx = r.Context()
		worker.serveContent(w, r, storageTestLocation("webdav", "broken.bin"))
	}))
	defer server.Close()
	response, err := server.Client().Get(server.URL)
	if err != nil {
		return // An aborted response before the headers is also a failed download.
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err == nil {
		t.Fatalf("truncated download reported success: status=%d data=%q", response.StatusCode, data)
	}
}
