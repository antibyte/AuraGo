package desktop

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestArchiveExtractionPreflightsAllEntries(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	if err := svc.WriteFileBytes(ctx, "target/keep.txt", []byte("original"), SourceUser); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"../target-sibling/escape.txt", "/absolute.txt", "C:/escape.txt", "a\\b", "a/../b"} {
		var data bytes.Buffer
		writer := zip.NewWriter(&data)
		first, _ := writer.Create("keep.txt")
		first.Write([]byte("changed"))
		last, _ := writer.Create(bad)
		last.Write([]byte("bad"))
		writer.Close()
		if err := svc.WriteFileBytes(ctx, "input.zip", data.Bytes(), SourceUser); err != nil {
			t.Fatal(err)
		}
		if err := svc.ExtractArchive(ctx, "input.zip", "target", SourceUser); err == nil {
			t.Fatalf("accepted bad path %q", bad)
		}
		content, err := os.ReadFile(filepath.Join(svc.Config().WorkspaceDir, "target", "keep.txt"))
		if err != nil || string(content) != "original" {
			t.Fatalf("invalid tail modified destination: %q, %v", content, err)
		}
	}
}

func TestArchiveRoundTripAndReadOnly(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	if err := svc.WriteFileBytes(ctx, "folder/source.txt", []byte("fixture"), SourceUser); err != nil {
		t.Fatal(err)
	}
	if err := svc.CreateArchive(ctx, []string{"folder"}, "output.zip", SourceUser); err != nil {
		t.Fatal(err)
	}
	if err := svc.ExtractArchive(ctx, "output.zip", "unpacked", SourceUser); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(svc.Config().WorkspaceDir, "unpacked", "folder", "source.txt"))
	if err != nil || string(data) != "fixture" {
		t.Fatalf("round trip: %q %v", data, err)
	}
	svc.mu.Lock()
	svc.cfg.ReadOnly = true
	svc.mu.Unlock()
	if err := svc.CreateArchive(ctx, []string{"folder"}, "denied.zip", SourceUser); err == nil {
		t.Fatal("read-only archive permitted")
	}
	if err := svc.ExtractArchive(ctx, "output.zip", "denied", SourceUser); err == nil {
		t.Fatal("read-only extract permitted")
	}
}
