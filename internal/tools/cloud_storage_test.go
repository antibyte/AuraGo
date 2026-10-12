package tools

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aurago/internal/testutil"
)

func TestCloudStorageBinaryOperations(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")
	f := testutil.NewStorageFixture(t)
	client := CloudStorage{WebDAV: WebDAVConfig{URL: f.Server.URL + "/dav"}, Koofr: KoofrConfig{BaseURL: f.Server.URL}}
	ctx := context.Background()
	mounts, err := client.Mounts(ctx)
	if err != nil || len(mounts) != 4 || mounts[2].Write || *mounts[3].Online {
		t.Fatalf("mounts: %#v %v", mounts, err)
	}
	for _, provider := range []string{"webdav", "koofr"} {
		t.Run(provider, func(t *testing.T) {
			loc := StorageLocation{Provider: provider}
			if provider == "koofr" {
				loc.MountID = "shared"
			}
			folder := loc
			folder.Path = "空 ü #&%"
			if err := client.Mkdir(ctx, folder); err != nil {
				t.Fatal(err)
			}
			entries, err := client.List(ctx, folder)
			if err != nil || len(entries) != 0 {
				t.Fatalf("empty: %v %v", entries, err)
			}
			file := folder
			file.Path += "/a ?.bin"
			binary := bytes.Repeat([]byte{0, 255, 13, 10, 128, 1}, 5000)
			entry, err := client.Put(ctx, file, bytes.NewReader(binary), "")
			if err != nil {
				t.Fatal(err)
			}
			if entry.Size != int64(len(binary)) || entry.Version == "" {
				t.Fatalf("metadata: %#v", entry)
			}
			entries, err = client.List(ctx, folder)
			if err != nil || len(entries) != 1 || entries[0].Path != file.Path || entries[0].Name != "a ?.bin" {
				t.Fatalf("href/name: %#v %v", entries, err)
			}
			reader, err := client.Open(ctx, file, entry.Version)
			if err != nil {
				t.Fatal(err)
			}
			got, err := io.ReadAll(reader)
			reader.Close()
			if err != nil || !bytes.Equal(got, binary) {
				t.Fatal("binary content altered")
			}
			if _, err = client.Put(ctx, file, strings.NewReader("bad"), ""); !errors.Is(err, ErrStorageConflict) {
				t.Fatalf("overwrite: %v", err)
			}
			if _, err = client.Put(ctx, file, strings.NewReader("new"), entry.Version); err != nil {
				t.Fatal(err)
			}
			if err = client.Remove(ctx, file, entry.Version, false); !errors.Is(err, ErrStorageConflict) {
				t.Fatalf("stale delete: %v", err)
			}
			entry, _ = client.Stat(ctx, file)
			next := file
			next.Path = folder.Path + "/renamed.bin"
			if err = client.Move(ctx, file, next, entry.Version); err != nil {
				t.Fatal(err)
			}
			if err = client.Remove(ctx, next, "", false); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCloudStorageRejectsPathsAndUnsafeHrefs(t *testing.T) {
	for _, p := range []string{"../a", "a/../b", "a//b", "a\\b", "/a", "a/", "C:foo", "a\x00b"} {
		if err := ValidateStorageLocation(StorageLocation{Provider: "webdav", Path: p}); err == nil {
			t.Errorf("accepted %q", p)
		}
	}
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")
	for _, href := range []string{"https://other.test/dav/a", "/outside/a", "/dav/../outside/a", "/dav/a?secret=x", "/dav/a%2Fb", ""} {
		t.Run(href, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(207)
				io.WriteString(w, `<multistatus xmlns="DAV:"><response><href>`+href+`</href><propstat><status>HTTP/1.1 200 OK</status><prop><getcontentlength>1</getcontentlength></prop></propstat></response></multistatus>`)
			}))
			defer server.Close()
			_, err := (CloudStorage{WebDAV: WebDAVConfig{URL: server.URL + "/dav"}}).List(context.Background(), StorageLocation{Provider: "webdav"})
			if err == nil {
				t.Fatalf("accepted href %q", href)
			}
		})
	}
}

func TestCloudStorageCancellationAndRedirectIsolation(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")
	called := false
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
	defer other.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL, http.StatusTemporaryRedirect)
	}))
	defer redirect.Close()
	c := CloudStorage{WebDAV: WebDAVConfig{URL: redirect.URL, Password: "fixture-only"}, Koofr: KoofrConfig{BaseURL: redirect.URL, AppPassword: "fixture-only"}}
	if _, err := c.List(context.Background(), StorageLocation{Provider: "webdav"}); err == nil {
		t.Fatal("redirect accepted")
	}
	if _, err := c.Mounts(context.Background()); err == nil {
		t.Fatal("Koofr redirect accepted")
	}
	if called {
		t.Fatal("cross-origin request sent")
	}
	started := make(chan struct{})
	blocked := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		close(started)
		<-r.Context().Done()
	}))
	defer blocked.Close()
	c.WebDAV.URL = blocked.URL
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := c.List(ctx, StorageLocation{Provider: "webdav"}); done <- err }()
	select {
	case <-started:
	case err := <-done:
		t.Fatalf("request did not start: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("request did not reach fixture")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation: %v", err)
		}
	case <-time.After(time.Second * 3):
		t.Fatal("request did not cancel")
	}
}
