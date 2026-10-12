package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"aurago/internal/retrogames"
	"aurago/internal/retrogames/retrogamestest"
)

// waitForRetroGamesWriteLock polls until another holder (the sync) owns the
// store's write lock: a prepare that waits until its deadline proves it.
func waitForRetroGamesWriteLock(t *testing.T, svc *retrogames.Service) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		pctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		_, release, err := svc.PrepareLibraryChange(pctx, retrogames.LibraryFavorite, "arcade_tron", true)
		cancel()
		if errors.Is(err, context.DeadlineExceeded) {
			return
		}
		if err != nil {
			t.Fatalf("probe prepare: %v", err)
		}
		release()
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("the sync never took the store's write lock")
}

// TestRetroGamesLibraryWriteWaitsOutsideThePublicationGate: a favorite PUT
// that arrives while the sync holds the store's write lock waits for that lock
// before it enters the desktop publication gate. Other desktop publications
// and a config revocation (both need desktopRuns.mu) go ahead meanwhile, the
// revoked PUT answers 403 instead of 500, and a PUT that waits the sync out is
// stored once the sync committed.
func TestRetroGamesLibraryWriteWaitsOutsideThePublicationGate(t *testing.T) {
	h := newRetroGamesHarness(t)
	dir := t.TempDir()
	svc := retrogames.NewService(retrogames.Deps{
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		Fetcher: &retrogamestest.Fetcher{},
		Sleep:   func(ctx context.Context, _ time.Duration) error { return ctx.Err() },
	})
	svc.Configure(retrogames.Settings{Enabled: true, DataDir: dir})
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = svc.Shutdown(ctx)
	})
	h.s.RetroGames = svc
	ctx := context.Background()

	// Open the index without syncing, then hold SQLite's write lock from a
	// second connection: the sync's first ReplaceSource takes the store's
	// in-process write lock and then waits inside SQLite.
	if _, err := svc.Library(ctx); err != nil {
		t.Fatal(err)
	}
	outside, err := sql.Open("sqlite", "file:"+filepath.ToSlash(filepath.Join(dir, "retrogames.db")))
	if err != nil {
		t.Fatal(err)
	}
	defer outside.Close()
	conn, err := outside.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		t.Fatalf("outside write transaction: %v", err)
	}
	unblocked := false
	unblock := func() {
		if !unblocked {
			unblocked = true
			_, _ = conn.ExecContext(ctx, "ROLLBACK")
		}
	}
	defer unblock()
	if err := svc.StartSync(); err != nil {
		t.Fatal(err)
	}
	waitForRetroGamesWriteLock(t, svc)

	put := func() <-chan *httptest.ResponseRecorder {
		out := make(chan *httptest.ResponseRecorder, 1)
		go func() {
			r := httptest.NewRequest(http.MethodPut, retroGamesPrefix+"favorites/arcade_tron", nil)
			r.Header.Set("Authorization", "Bearer "+h.writeToken)
			w := httptest.NewRecorder()
			h.s.handleRetroGames(w, r)
			out <- w
		}()
		return out
	}
	code := func(w *httptest.ResponseRecorder) string {
		var body map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		c, _ := body["code"].(string)
		return c
	}

	first := put()
	time.Sleep(200 * time.Millisecond)
	select {
	case w := <-first:
		t.Fatalf("PUT finished while the sync held the write lock: %d %s", w.Code, w.Body.String())
	default:
	}

	// Another desktop app's run and publication (both take desktopRuns.mu)
	// are not held up by the waiting PUT.
	published := make(chan error, 1)
	go func() {
		otherCtx, otherCancel, err := h.s.beginDesktopRun(ctx)
		if err != nil {
			published <- err
			return
		}
		defer otherCancel()
		published <- publishDesktopResult(otherCtx, func() error { return nil })
	}()
	select {
	case err := <-published:
		if err != nil {
			t.Fatalf("other publication: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("a desktop publication waited for the Retro Games write")
	}

	// Neither is a config revocation; it ends the waiting PUT as a revoked
	// desktop action.
	revoked := make(chan struct{})
	go func() {
		h.s.revokeDesktopRuns()
		close(revoked)
	}()
	select {
	case <-revoked:
	case <-time.After(2 * time.Second):
		t.Fatal("revokeDesktopRuns waited for the Retro Games write")
	}
	select {
	case w := <-first:
		if w.Code != http.StatusForbidden || code(w) != "desktop_readonly" {
			t.Fatalf("revoked PUT: %d %s", w.Code, w.Body.String())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("a revoked PUT kept waiting for the write lock")
	}

	// A PUT that waits the sync out is stored once the sync committed.
	second := put()
	time.Sleep(100 * time.Millisecond)
	select {
	case w := <-second:
		t.Fatalf("second PUT finished while the sync held the write lock: %d %s", w.Code, w.Body.String())
	default:
	}
	unblock()
	select {
	case w := <-second:
		if w.Code != http.StatusOK {
			t.Fatalf("PUT after the sync: %d %s", w.Code, w.Body.String())
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the PUT did not finish after the sync committed")
	}
	h.waitIdle()
	lib, err := svc.Library(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(lib.Favorites) != 1 || lib.Favorites[0].Identifier != "arcade_tron" {
		t.Fatalf("favorites = %+v", lib.Favorites)
	}
}
