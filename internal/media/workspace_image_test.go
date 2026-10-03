package media

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestWorkspaceImageSnapshotRejectsEscapesAndKeepsOwnBytes(t *testing.T) {
	root := t.TempDir()
	work := filepath.Join(root, "work")
	if err := os.MkdirAll(work, 0o700); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(filepath.Join(work, "one.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	file.Close()
	stage, err := StageWorkspaceImage(work, "/files/one.png")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(stage)
	if err := os.WriteFile(filepath.Join(work, "one.png"), []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	staged, err := os.Open(stage)
	if err != nil {
		t.Fatal(err)
	}
	defer staged.Close()
	if _, _, err := image.DecodeConfig(staged); err != nil {
		t.Fatalf("stage was not immutable: %v", err)
	}
	for _, raw := range []string{"/files/../secret.png", "/files/..\\secret.png", "https://foreign.example/files/one.png", "/files/one.png"} {
		if _, err := StageWorkspaceImage(work, raw); err == nil {
			t.Fatalf("accepted unsafe/invalid input %q", raw)
		}
	}
	if err := os.Symlink(filepath.Join(root, "secret.png"), filepath.Join(work, "link.png")); err == nil {
		if _, err := StageWorkspaceImage(work, "/files/link.png"); err == nil {
			t.Fatal("symlink accepted")
		}
	}
}
