package webassets

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func testArchive(t *testing.T, mutate func(*tar.Header, *[]byte)) ([]byte, Pin) {
	t.Helper()
	b := []byte("<!doctype html><title>local</title>")
	m, _ := json.Marshal(Manifest{Version: 1, Files: []Entry{{Path: "ui/index.html", Size: int64(len(b)), SHA256: Digest(b)}}})
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: ManifestName, Mode: 0644, Typeflag: tar.TypeReg, Size: int64(len(m))}); err != nil {
		t.Fatal(err)
	}
	tw.Write(m)
	h := &tar.Header{Name: "ui/index.html", Mode: 0644, Typeflag: tar.TypeReg, Size: int64(len(b))}
	if mutate != nil {
		mutate(h, &b)
	}
	if err := tw.WriteHeader(h); err != nil {
		t.Fatal(err)
	}
	tw.Write(b)
	tw.Close()
	gz.Close()
	data := buf.Bytes()
	return data, Pin{ID: Digest(m), SHA256: Digest(data), Bytes: int64(len(data))}
}

func TestAssetInstallIntegrityAndLifecycle(t *testing.T) {
	data, pin := testArchive(t, nil)
	root := t.TempDir()
	s := Open(root, pin)
	if s.Ready() {
		t.Fatal("missing set ready")
	}
	if err := s.Install(context.Background(), bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}
	if s.Ready() {
		t.Fatal("installation activated captured filesystem before restart")
	}
	restarted := Open(root, pin)
	defer restarted.Close()
	if !restarted.Ready() {
		t.Fatal(restarted.Error)
	}
	if _, err := fs.ReadFile(restarted.Namespace("ui"), "index.html"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"../index.html", "ui/../../secret", "manifest.json", "ui\\index.html"} {
		if _, err := restarted.Open(name); err == nil {
			t.Fatalf("opened %q", name)
		}
	}
	var wg sync.WaitGroup
	for range 3 {
		wg.Go(func() {
			if err := s.Install(context.Background(), bytes.NewReader(data)); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	restarted.Close()
	if err := os.WriteFile(filepath.Join(root, pin.ID, "ui/index.html"), []byte("tampered"), 0644); err != nil {
		t.Fatal(err)
	}
	bad := Open(root, pin)
	if bad.Ready() {
		t.Fatal("accepted tampering")
	}
	if err := bad.Install(context.Background(), bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}
	fixed := Open(root, pin)
	defer fixed.Close()
	if !fixed.Ready() {
		t.Fatal(fixed.Error)
	}
}

func TestAssetArchiveRejection(t *testing.T) {
	for name, mutate := range map[string]func(*tar.Header, *[]byte){
		"traversal": func(h *tar.Header, b *[]byte) { h.Name = "../escape" },
		"symlink": func(h *tar.Header, b *[]byte) {
			h.Typeflag = tar.TypeSymlink
			h.Linkname = "../../outside"
			h.Size = 0
			*b = nil
		},
		"hardlink": func(h *tar.Header, b *[]byte) {
			h.Typeflag = tar.TypeLink
			h.Linkname = "ui/index.html"
			h.Size = 0
			*b = nil
		},
		"executable":    func(h *tar.Header, b *[]byte) { h.Mode = 0755 },
		"wrong content": func(h *tar.Header, b *[]byte) { (*b)[0] = 'x' },
		"wrong size":    func(h *tar.Header, b *[]byte) { h.Size = 1; *b = (*b)[:1] },
	} {
		t.Run(name, func(t *testing.T) {
			data, pin := testArchive(t, mutate)
			s := Open(t.TempDir(), pin)
			if err := s.Install(context.Background(), bytes.NewReader(data)); err == nil {
				t.Fatal("accepted invalid archive")
			}
			if _, err := os.Stat(filepath.Join(s.Dir, pin.ID)); !os.IsNotExist(err) {
				t.Fatal("partial publication")
			}
		})
	}
	data, pin := testArchive(t, nil)
	for _, b := range [][]byte{data[:len(data)-1], append(append([]byte{}, data...), 0), bytes.Repeat([]byte{'x'}, len(data))} {
		s := Open(t.TempDir(), pin)
		if err := s.Install(context.Background(), bytes.NewReader(b)); err == nil {
			t.Fatal("accepted corrupt download")
		}
	}
	s := Open(t.TempDir(), pin)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.Install(ctx, bytes.NewReader(data)); err == nil {
		t.Fatal("accepted cancelled install")
	}
	for _, p := range []string{"../x", "/x", "x\\y", "x:y", "CON.txt", "ui/aux", "ui/x.", "ui/.hidden"} {
		if ValidPath(p) {
			t.Errorf("accepted %q", p)
		}
	}
}

func TestAssetSymlinkAndDownloadBoundary(t *testing.T) {
	data, pin := testArchive(t, nil)
	dir := t.TempDir()
	s := Open(dir, pin)
	if err := s.Install(context.Background(), bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, pin.ID, "ui/index.html")
	original, _ := os.ReadFile(file)
	outside := filepath.Join(t.TempDir(), "outside")
	os.WriteFile(outside, original, 0644)
	os.Remove(file)
	if err := os.Symlink(outside, file); err == nil {
		linked := Open(dir, pin)
		if linked.Ready() {
			linked.Close()
			t.Fatal("accepted asset symlink")
		}
	}
	for _, url := range []string{"http://github.com/antibyte/AuraGo/releases/x", "https://example.com/x", "file:///x", "https://github.com:443/x", "https://user@github.com/x"} {
		s.Pin.URL = url
		if err := s.Download(context.Background()); err == nil {
			t.Fatalf("accepted %s", url)
		}
	}
}

func TestAssetDirectoryImportAndRollback(t *testing.T) {
	data, pin := testArchive(t, nil)
	source := Open(t.TempDir(), pin)
	if err := source.Install(context.Background(), bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}
	dest := Open(t.TempDir(), pin)
	if err := dest.Import(context.Background(), source.Dir); err != nil {
		t.Fatal(err)
	}
	active := Open(dest.Dir, pin)
	if !active.Ready() {
		t.Fatal(active.Error)
	}
	active.Close()
	other := Pin{ID: Digest([]byte("other revision"))}
	wrong := Open(dest.Dir, other)
	if err := wrong.Import(context.Background(), source.Dir); err == nil {
		t.Fatal("accepted wrong set")
	}
	rollback := Open(dest.Dir, pin)
	defer rollback.Close()
	if !rollback.Ready() {
		t.Fatal("previous set lost")
	}
}
