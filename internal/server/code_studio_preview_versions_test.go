package server

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestCodeStudioPreviewVersionRegistryLongPoll(t *testing.T) {
	ctx := context.Background()
	r := newCodeStudioPreviewVersionRegistry()
	if v := r.Current("local"); v != 0 {
		t.Fatalf("fresh version = %d, want 0", v)
	}
	r.Bump("local", "/workspace/a.css")
	if v, paths, full := r.Wait(ctx, "local", 0, time.Second); v != 1 || strings.Join(paths, ",") != "a.css" || full {
		t.Fatalf("behind client = %d %v %v, want 1 [a.css] false", v, paths, full)
	}
	type result struct {
		version uint64
		paths   []string
		full    bool
	}
	done := make(chan result, 1)
	go func() {
		v, p, f := r.Wait(ctx, "local", 1, 2*time.Second)
		done <- result{v, p, f}
	}()
	time.Sleep(30 * time.Millisecond)
	r.Bump("local", "/workspace/b.html", "/workspace/c.css")
	if got := <-done; got.version != 2 || strings.Join(got.paths, ",") != "b.html,c.css" || got.full {
		t.Fatalf("woken wait = %+v, want 2 [b.html c.css] false", got)
	}
	r.Bump("") // "" is the local workspace; no paths means anything may have changed
	if v, _, full := r.Wait(ctx, "local", 2, time.Second); v != 3 || !full {
		t.Fatalf("pathless bump = %d full=%v, want 3 full", v, full)
	}
	if r.Current("ssh-1") != 0 {
		t.Fatal("workspaces must count independently")
	}
	start := time.Now()
	if v, paths, _ := r.Wait(ctx, "local", 3, 50*time.Millisecond); v != 3 || paths != nil || time.Since(start) < 40*time.Millisecond {
		t.Fatalf("timeout = %d %v after %v, want the unchanged version after the timeout", v, paths, time.Since(start))
	}
	for i := 0; i < 70; i++ {
		r.Bump("local", "/workspace/x.css")
	}
	if _, _, full := r.Wait(ctx, "local", 3, time.Second); !full {
		t.Fatal("a client behind the kept history must get a full reload")
	}
	start = time.Now()
	if _, _, full := r.Wait(ctx, "local", 10_000, time.Second); !full || time.Since(start) > 200*time.Millisecond {
		t.Fatal("a client ahead of the server (restart) must get an immediate full answer")
	}
}
