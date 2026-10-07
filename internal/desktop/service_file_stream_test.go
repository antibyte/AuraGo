package desktop

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/fileutil"
)

type desktopStreamZeroReader struct{}

func (desktopStreamZeroReader) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

func TestDesktopFileStreamLargeConditionalPublication(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	const size = 51 << 20
	entry, err := svc.WriteFileStreamConditional(ctx, "Documents/Video Studio/clip.mp4", io.LimitReader(desktopStreamZeroReader{}, size), size, SourceUser, nil)
	if err != nil || entry.Size != size {
		t.Fatalf("streamed file: size=%d err=%v", entry.Size, err)
	}
	if _, err := svc.WriteFileStreamConditional(ctx, entry.Path, strings.NewReader("replacement"), size, SourceUser, nil); err == nil {
		t.Fatal("create-only write replaced an existing video")
	}
	_, err = svc.WriteFileStreamConditional(ctx, entry.Path, strings.NewReader("replacement"), size, SourceUser, func(state FileWriteState) error {
		if !state.Exists || len(state.Version) != 66 || state.Data != nil {
			t.Fatalf("stream precondition buffered data or lost version: %+v", state.Entry)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(svc.Config().WorkspaceDir, filepath.FromSlash(entry.Path)))
	if err != nil || string(content) != "replacement" {
		t.Fatalf("published content: %q %v", content, err)
	}
}

func TestDesktopFileStreamFailuresPreserveDestination(t *testing.T) {
	for _, scenario := range []string{"oversize", "cancel", "revoked", "readonly", "stale"} {
		t.Run(scenario, func(t *testing.T) {
			svc := testService(t)
			ctx := context.Background()
			const target = "Documents/video.mp4"
			if err := svc.WriteFile(ctx, target, "original", SourceUser); err != nil {
				t.Fatal(err)
			}
			limit := int64(1024)
			check := FileWritePrecondition(func(state FileWriteState) error {
				if state.Version != NoteVersion([]byte("original")) {
					t.Fatal("wrong content version")
				}
				return nil
			})
			switch scenario {
			case "oversize":
				limit = 3
			case "cancel":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "revoked":
				ctx = fileutil.WithPublicationGate(ctx, func(func() error) error { return context.Canceled })
			case "readonly":
				svc.SetReadOnly(true)
			case "stale":
				check = func(FileWriteState) error { return errors.New("conflict") }
			}
			if _, err := svc.WriteFileStreamConditional(ctx, target, strings.NewReader("replacement"), limit, SourceUser, check); err == nil {
				t.Fatal("expected rejection")
			}
			content, err := os.ReadFile(filepath.Join(svc.Config().WorkspaceDir, target))
			if err != nil || string(content) != "original" {
				t.Fatalf("lost original: %q %v", content, err)
			}
			matches, _ := filepath.Glob(filepath.Join(svc.Config().WorkspaceDir, "Documents", ".aurago-write-*"))
			if len(matches) != 0 {
				t.Fatalf("leaked staging files: %v", matches)
			}
		})
	}
}

func TestDesktopFileStreamProtectedPaths(t *testing.T) {
	svc := testService(t)
	for _, path := range []string{"../escaped.mp4", "Documents/Notes/test.md", "Trash/Notes/test.md", "Widgets/test.html"} {
		if _, err := svc.WriteFileStreamConditional(context.Background(), path, strings.NewReader("data"), 10, SourceAgent, nil); err == nil {
			t.Errorf("allowed protected path %q", path)
		}
	}
}
