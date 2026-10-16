package decisions

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestDecisionsPublicationHandlesWindowsReaders(t *testing.T) {
	for _, copyRuntime := range []bool{false, true} {
		for _, cancelled := range []bool{false, true} {
			t.Run(fmtPublicationCase(copyRuntime, cancelled), func(t *testing.T) {
				root := t.TempDir()
				source, target := filepath.Join(root, "replacement"), filepath.Join(root, "target")
				for path, value := range map[string]string{source: "new", target: "old"} {
					if err := os.WriteFile(path, []byte(value), 0o700); err != nil {
						t.Fatal(err)
					}
				}
				name, err := windows.UTF16PtrFromString(target)
				if err != nil {
					t.Fatal(err)
				}
				handle, err := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
				if err != nil {
					t.Fatal(err)
				}
				var once sync.Once
				release := func() { once.Do(func() { _ = windows.CloseHandle(handle) }) }
				defer release()
				ctx := t.Context()
				if cancelled {
					var cancel context.CancelFunc
					ctx, cancel = context.WithTimeout(ctx, 40*time.Millisecond)
					defer cancel()
				} else {
					timer := time.AfterFunc(60*time.Millisecond, release)
					defer timer.Stop()
				}
				publish := publishArtifact
				if copyRuntime {
					publish = copyAtomic
				}
				err = publish(ctx, source, target)
				release()
				want := "new"
				if cancelled {
					want = "old"
					if !errors.Is(err, context.DeadlineExceeded) {
						t.Fatalf("publication did not cancel: %v", err)
					}
					if got, readErr := os.ReadFile(source); readErr != nil || string(got) != "new" {
						t.Fatalf("replacement lost: %q, %v", got, readErr)
					}
				} else if err != nil {
					t.Fatal(err)
				}
				if got, err := os.ReadFile(target); err != nil || string(got) != want {
					t.Fatalf("target = %q, %v", got, err)
				}
			})
		}
	}
}

func fmtPublicationCase(copyRuntime, cancelled bool) string {
	name := "model"
	if copyRuntime {
		name = "runtime"
	}
	if cancelled {
		return name + "/cancel"
	}
	return name + "/retry"
}
