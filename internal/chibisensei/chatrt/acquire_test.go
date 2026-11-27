package chatrt

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestInstanceThreads(t *testing.T) {
	for logical, want := range map[int]int{1: 1, 2: 1, 4: 1, 8: 2, 12: 3, 16: 4, 32: 8, 64: 8} {
		if got := instanceThreads(logical); got != want {
			t.Errorf("instanceThreads(%d) = %d, want %d", logical, got, want)
		}
	}
}

// TestAcquireChecksModelFileWithoutFollowingLinks covers S1: the file is checked again right before a start.
func TestAcquireChecksModelFileWithoutFollowingLinks(t *testing.T) {
	l := newFakeLauncher(true)
	fx := newFixture(t, l)
	good := fx.model("tuned:ok")
	acquire := func(ref ModelRef) error {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		lease, err := fx.m.Acquire(ctx, ref)
		if err == nil {
			lease.Release()
		}
		return err
	}

	link := filepath.Join(t.TempDir(), "link.gguf")
	if err := os.Symlink(good.Path, link); err != nil {
		t.Logf("symbolic link not possible here: %v", err)
	} else if err := acquire(ModelRef{Key: "tuned:link", Path: link, Bytes: good.Bytes}); !errors.Is(err, ErrModelMissing) {
		t.Fatalf("symbolic link: err = %v, want ErrModelMissing", err)
	}
	if err := acquire(ModelRef{Key: "tuned:dir", Path: t.TempDir(), Bytes: 0}); !errors.Is(err, ErrModelMissing) {
		t.Fatalf("directory: err = %v, want ErrModelMissing", err)
	}
	wrong := good
	wrong.Key, wrong.Bytes = "tuned:size", good.Bytes+1
	if err := acquire(wrong); !errors.Is(err, ErrModelMissing) {
		t.Fatalf("size mismatch: err = %v, want ErrModelMissing", err)
	}
	if n := len(l.launchSpecs()); n != 0 {
		t.Fatalf("a refused model started %d processes", n)
	}
	unsized := good
	unsized.Key, unsized.Bytes = "tuned:unsized", 0 // no expected size: any size passes
	if err := acquire(unsized); err != nil {
		t.Fatalf("regular file without expected size: %v", err)
	}
	if err := acquire(good); err != nil {
		t.Fatalf("regular file: %v", err)
	}
	if n := len(l.launchSpecs()); n != 2 {
		t.Fatalf("launches = %d, want 2", n)
	}
}
