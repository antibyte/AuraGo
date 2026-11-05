package desktop

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func storymakerVideoDir(svc *Service) string {
	return filepath.Join(svc.Config().DataDir, "generated_videos", "Storymaker")
}

func TestWriteStorymakerVideoCreatesTheFolderAndAvoidsCollisions(t *testing.T) {
	svc := testMediaService(t)
	ctx := context.Background()
	dir := storymakerVideoDir(svc)
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("a fresh data dir already has the folder: %v", err)
	}
	for i, want := range []string{"Videos/Storymaker/clip.mp4", "Videos/Storymaker/clip-2.mp4", "Videos/Storymaker/clip-3.mp4"} {
		body := strings.Repeat("v", i+1)
		got, err := svc.WriteStorymakerVideo(ctx, "clip", strings.NewReader(body), int64(len(body)), SourceUser)
		if err != nil || got != want {
			t.Fatalf("copy %d = %q, %v, want %q", i+1, got, err, want)
		}
		data, err := os.ReadFile(filepath.Join(dir, filepath.Base(want)))
		if err != nil || string(data) != body {
			t.Fatalf("copy %d holds %q, %v", i+1, data, err)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 3 {
		t.Fatalf("the folder holds %v, %v, want the three copies and no staging file", entries, err)
	}
}

func TestWriteStorymakerVideoRefusals(t *testing.T) {
	svc := testMediaService(t)
	ctx := context.Background()
	for _, slug := range []string{"", "../clip", "a/b", `a\b`, "a b", "clip.mp4", strings.Repeat("a", 81)} {
		if _, err := svc.WriteStorymakerVideo(ctx, slug, strings.NewReader("v"), 1, SourceUser); err == nil {
			t.Errorf("slug %q was accepted", slug)
		}
	}
	// 2^62 bytes are more than any test box has free, and the limit is reached before a byte is read.
	if _, err := svc.WriteStorymakerVideo(ctx, "clip", strings.NewReader("v"), int64(1)<<62, SourceUser); err == nil || !strings.Contains(err.Error(), "free space") {
		t.Errorf("low disk = %v", err)
	}
	if _, err := svc.WriteStorymakerVideo(ctx, "clip", strings.NewReader("short"), 99, SourceUser); err == nil {
		t.Error("a source shorter than its size was accepted")
	}
	if _, err := svc.WriteStorymakerVideo(ctx, "clip", strings.NewReader("too long"), 3, SourceUser); err == nil {
		t.Error("a source longer than its size was accepted")
	}
	svc.SetReadOnly(true)
	if _, err := svc.WriteStorymakerVideo(ctx, "clip", strings.NewReader("v"), 1, SourceUser); err == nil {
		t.Error("a read-only desktop accepted the copy")
	}
	svc.SetReadOnly(false)
	entries, _ := os.ReadDir(storymakerVideoDir(svc))
	if len(entries) != 0 {
		t.Errorf("refused copies left %v", entries)
	}
}
