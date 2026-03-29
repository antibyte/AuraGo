package tools

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractZipRejectsSymlinkEntries(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "malicious.zip")

	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	zw := zip.NewWriter(f)
	hdr := &zip.FileHeader{Name: "link-out"}
	hdr.SetMode(os.ModeSymlink | 0o777)
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		t.Fatalf("create header: %v", err)
	}
	if _, err := w.Write([]byte("../../outside")); err != nil {
		t.Fatalf("write header: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close file: %v", err)
	}

	_, err = extractZip(archivePath, filepath.Join(dir, "out"))
	if err == nil || !strings.Contains(err.Error(), "symlink entries") {
		t.Fatalf("expected symlink rejection, got: %v", err)
	}
}

func TestExtractTarGzRejectsSymlinkEntries(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "malicious.tar.gz")

	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create tar.gz: %v", err)
	}
	gzw := gzip.NewWriter(f)
	tw := tar.NewWriter(gzw)
	if err := tw.WriteHeader(&tar.Header{
		Name:     "link-out",
		Typeflag: tar.TypeSymlink,
		Linkname: "../../outside",
		Mode:     0o777,
	}); err != nil {
		t.Fatalf("write tar header: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close file: %v", err)
	}

	_, err = extractTarGz(archivePath, filepath.Join(dir, "out"))
	if err == nil || !strings.Contains(err.Error(), "symlink entries") {
		t.Fatalf("expected symlink rejection, got: %v", err)
	}
}
