package chatrt

import (
	"context"
	"errors"
	"fmt"
	"syscall"
	"testing"
	"time"
)

func TestManagerRefusesBeforeInstall(t *testing.T) {
	ctx := context.Background()

	t.Run("disabled", func(t *testing.T) {
		fx := newFixture(t, newFakeLauncher(true))
		fx.setConfig(func(c *Config) { c.Enabled = false })
		if st := fx.m.Status(ctx); st.State != "disabled" {
			t.Fatalf("state %q", st.State)
		}
		if _, err := fx.m.Acquire(ctx, fx.model("a")); !errors.Is(err, ErrDisabled) {
			t.Fatalf("Acquire: %v", err)
		}
		if _, err := fx.m.BeginInstall(ctx); !errors.Is(err, ErrDisabled) {
			t.Fatalf("BeginInstall: %v", err)
		}
	})

	t.Run("unsupported", func(t *testing.T) {
		l := newFakeLauncher(true)
		l.goos = "freebsd"
		fx := newFixture(t, l)
		if st := fx.m.Status(ctx); st.State != "unsupported" {
			t.Fatalf("state %q", st.State)
		}
		if _, err := fx.m.Acquire(ctx, fx.model("a")); !errors.Is(err, ErrUnsupported) {
			t.Fatalf("Acquire: %v", err)
		}
		if err := fx.m.Install(ctx); !errors.Is(err, ErrUnsupported) {
			t.Fatalf("Install: %v", err)
		}
		if err := fx.m.Admit("a"); !errors.Is(err, ErrUnsupported) {
			t.Fatalf("Admit: %v", err)
		}
	})

	t.Run("not installed", func(t *testing.T) {
		l := newFakeLauncher(false)
		fx := newFixture(t, l)
		st := fx.m.Status(ctx)
		if st.State != "not_installed" || st.Download.Bytes <= 0 || st.Download.Host != "github.com" {
			t.Fatalf("status %+v", st)
		}
		if _, err := fx.m.Acquire(ctx, fx.model("a")); !errors.Is(err, ErrNotInstalled) {
			t.Fatalf("Acquire: %v", err)
		}
		if err := fx.m.Admit("a"); !errors.Is(err, ErrNotInstalled) {
			t.Fatalf("Admit: %v", err)
		}
		if l.installs() != 0 {
			t.Fatalf("Acquire or Status downloaded: %d installs", l.installs())
		}
	})
}

func TestManagerInstallProgressAndJoin(t *testing.T) {
	l := newFakeLauncher(false)
	gate := make(chan struct{})
	progressed := make(chan struct{})
	l.installFn = func(ctx context.Context, progress func(string, int64)) error {
		progress("fake", 1000)
		close(progressed)
		select {
		case <-gate:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	fx := newFixture(t, l)
	ctx := context.Background()

	first, err := fx.m.BeginInstall(ctx)
	if err != nil {
		t.Fatal(err)
	}
	waitClosed(t, progressed, "install progress")
	st := fx.m.Status(ctx)
	if st.State != "installing" || st.Download.DoneBytes != 1000 {
		t.Fatalf("status %+v", st)
	}
	second, err := fx.m.BeginInstall(ctx)
	if err != nil || second != first {
		t.Fatalf("second BeginInstall: %v (same channel: %v)", err, second == first)
	}
	if _, err := fx.m.Acquire(ctx, fx.model("a")); !errors.Is(err, ErrInstalling) {
		t.Fatalf("Acquire during install: %v", err)
	}
	if err := fx.m.Admit("a"); !errors.Is(err, ErrInstalling) {
		t.Fatalf("Admit during install: %v", err)
	}
	close(gate)
	waitClosed(t, first, "install end")
	if st := fx.m.Status(ctx); st.State != "ready" {
		t.Fatalf("state after install %q", st.State)
	}
	if l.installs() != 1 {
		t.Fatalf("Install ran %d times", l.installs())
	}
	if err := fx.m.Install(ctx); err != nil {
		t.Fatalf("Install on an installed runtime: %v", err)
	}
	if l.installs() != 1 {
		t.Fatalf("Install ran again on an installed runtime")
	}
}

func TestManagerInstallErrorCodes(t *testing.T) {
	cases := []struct {
		name string
		err  error
		code string
	}{
		{"checksum", errors.New("install llama.cpp runtime x: SHA-256 mismatch"), "checksum_mismatch"},
		{"disk", fmt.Errorf("write archive: %w", syscall.ENOSPC), "disk_full"},
		{"plain", errors.New("connection reset"), "download_failed"},
	}
	l := newFakeLauncher(false)
	var next error
	l.installFn = func(context.Context, func(string, int64)) error { return next }
	fx := newFixture(t, l)
	ctx := context.Background()
	for _, tc := range cases {
		next = tc.err
		if err := fx.m.Install(ctx); err == nil {
			t.Fatalf("%s: Install succeeded", tc.name)
		}
		st := fx.m.Status(ctx)
		if st.State != "failed" || st.Download.ErrorCode != tc.code {
			t.Fatalf("%s: status %q code %q", tc.name, st.State, st.Download.ErrorCode)
		}
	}

	// The next BeginInstall clears the code while it runs.
	gate := make(chan struct{})
	l.installFn = func(ctx context.Context, _ func(string, int64)) error {
		select {
		case <-gate:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	done, err := fx.m.BeginInstall(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st := fx.m.Status(ctx); st.State != "installing" || st.Download.ErrorCode != "" {
		t.Fatalf("status while retrying %+v", st)
	}
	close(gate)
	waitClosed(t, done, "retry")
	if st := fx.m.Status(ctx); st.State != "ready" {
		t.Fatalf("state after retry %q", st.State)
	}
}

func TestManagerInstallDiskRule(t *testing.T) {
	l := newFakeLauncher(false)
	fx := newFixture(t, l)
	fx.m.freeDisk = func(string) (int64, error) { return 1 << 20, nil }
	err := fx.m.Install(context.Background())
	if err == nil {
		t.Fatal("Install succeeded with a full disk")
	}
	st := fx.m.Status(context.Background())
	if st.State != "failed" || st.Download.ErrorCode != "disk_full" {
		t.Fatalf("status %q code %q", st.State, st.Download.ErrorCode)
	}
	if l.installs() != 0 {
		t.Fatal("launcher.Install ran despite the disk rule")
	}
}

func TestManagerCloseCancelsInstall(t *testing.T) {
	l := newFakeLauncher(false)
	started := make(chan struct{})
	l.installFn = func(ctx context.Context, _ func(string, int64)) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	}
	fx := newFixture(t, l)
	done, err := fx.m.BeginInstall(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	waitClosed(t, started, "install start")
	fx.m.Close()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("install did not end after Close")
	}
	if _, err := fx.m.BeginInstall(context.Background()); !errors.Is(err, ErrClosed) {
		t.Fatalf("BeginInstall after Close: %v", err)
	}
}
