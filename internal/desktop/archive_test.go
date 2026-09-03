package desktop

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func writeTestZip(t *testing.T, svc *Service, dest string, files map[string][]byte) {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range files {
		writer, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", name, err)
		}
		if _, err := writer.Write(data); err != nil {
			t.Fatalf("write zip entry %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	if err := svc.WriteFileBytes(context.Background(), dest, buf.Bytes(), SourceUser); err != nil {
		t.Fatalf("WriteFileBytes %s: %v", dest, err)
	}
}

func TestNormalizeArchiveEntryNameRejectsTraversal(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"", "../secret.txt", "/etc/passwd", "foo/../../etc/passwd", `C:\windows\x.txt`, "foo/./../bar.txt"} {
		if _, err := NormalizeArchiveEntryName(name); !errors.Is(err, ErrArchiveEntryUnsafe) {
			t.Fatalf("NormalizeArchiveEntryName(%q) = %v, want ErrArchiveEntryUnsafe", name, err)
		}
	}
	got, err := NormalizeArchiveEntryName(`docs\readme.txt`)
	if err != nil {
		t.Fatalf("NormalizeArchiveEntryName backslash: %v", err)
	}
	if got != "docs/readme.txt" {
		t.Fatalf("normalized = %q", got)
	}
}

func TestReadArchiveEntryReturnsPreviewableFile(t *testing.T) {
	svc := testService(t)
	writeTestZip(t, svc, "Documents/sample.zip", map[string][]byte{
		"docs/readme.txt": []byte("hello zipper"),
		"docs/photo.png":  []byte("\x89PNG"),
	})

	entry, err := svc.ReadArchiveEntry(context.Background(), "Documents/sample.zip", "docs/readme.txt")
	if err != nil {
		t.Fatalf("ReadArchiveEntry: %v", err)
	}
	if entry.Name != "readme.txt" || string(entry.Data) != "hello zipper" {
		t.Fatalf("entry = %+v data = %q", entry, entry.Data)
	}
	if !strings.HasPrefix(entry.MIMEType, "text/") && entry.MIMEType != "text/plain" {
		t.Fatalf("mime = %q", entry.MIMEType)
	}
}

func TestReadArchiveEntryRejectsUnsafeMissingAndBlockedTypes(t *testing.T) {
	svc := testService(t)
	writeTestZip(t, svc, "Documents/sample.zip", map[string][]byte{
		"docs/readme.txt":  []byte("hello"),
		"docs/payload.exe": []byte("MZ"),
		"docs/":            []byte{},
	})

	if _, err := svc.ReadArchiveEntry(context.Background(), "Documents/sample.zip", "../readme.txt"); !errors.Is(err, ErrArchiveEntryUnsafe) {
		t.Fatalf("traversal: %v", err)
	}
	if _, err := svc.ReadArchiveEntry(context.Background(), "Documents/sample.zip", "docs/missing.txt"); !errors.Is(err, ErrArchiveEntryNotFound) {
		t.Fatalf("missing: %v", err)
	}
	if _, err := svc.ReadArchiveEntry(context.Background(), "Documents/sample.zip", "docs/payload.exe"); !errors.Is(err, ErrArchiveEntryNotPreviewable) {
		t.Fatalf("exe: %v", err)
	}
	if _, err := svc.ReadArchiveEntry(context.Background(), "Documents/sample.zip", "docs"); !errors.Is(err, ErrArchiveEntryIsDirectory) && !errors.Is(err, ErrArchiveEntryNotPreviewable) {
		t.Fatalf("directory: %v", err)
	}
}

func TestReadArchiveEntryRejectsOversizedMember(t *testing.T) {
	svc := testService(t)
	writeTestZip(t, svc, "Documents/big.zip", map[string][]byte{
		"notes.txt": bytes.Repeat([]byte("a"), 1024*1024+32),
	})
	if _, err := svc.ReadArchiveEntry(context.Background(), "Documents/big.zip", "notes.txt"); !errors.Is(err, ErrArchiveEntryTooLarge) {
		t.Fatalf("oversized: %v", err)
	}
}
