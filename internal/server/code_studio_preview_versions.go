package server

import (
	"context"
	"strings"
	"sync"
	"time"
)

// Every Code Studio mutation bumps the version of its workspace through notifyWorkspaceChange; live.js in each
// preview long-polls that version and reloads (or swaps stylesheets when only .css files changed). The registry
// lives in memory per server: a restart sends every open preview one full reload.
const codeStudioPreviewHistory = 64

type codeStudioPreviewChange struct {
	version uint64
	paths   []string // workspace-relative; nil means anything may have changed
}

type codeStudioPreviewSpace struct {
	version uint64
	changes []codeStudioPreviewChange
	wake    chan struct{}
}

type codeStudioPreviewVersionRegistry struct {
	mu     sync.Mutex
	spaces map[string]*codeStudioPreviewSpace
}

func newCodeStudioPreviewVersionRegistry() *codeStudioPreviewVersionRegistry {
	return &codeStudioPreviewVersionRegistry{spaces: map[string]*codeStudioPreviewSpace{}}
}

// One registry per *Server without a field on Server (server.go is shared with other sessions).
var codeStudioPreviewRegistries sync.Map

func codeStudioPreviewVersionsFor(s *Server) *codeStudioPreviewVersionRegistry {
	if registry, ok := codeStudioPreviewRegistries.Load(s); ok {
		return registry.(*codeStudioPreviewVersionRegistry)
	}
	fresh := newCodeStudioPreviewVersionRegistry()
	actual, loaded := codeStudioPreviewRegistries.LoadOrStore(s, fresh)
	registry := actual.(*codeStudioPreviewVersionRegistry)
	if !loaded {
		// Plan A's notifyWorkspaceChange (write, move, delete, upload, git discard/switch, init) fans out to the
		// registered hooks; every registry bumps its workspace counter. paths are /workspace virtual paths; none
		// means "anything". The hook is registered once per Server and lives as long as the registry.
		registerCodeStudioWorkspaceChangeHook(func(wsID string, paths []string) { registry.Bump(wsID, paths...) })
	}
	return registry
}

func codeStudioPreviewSpaceKey(workspace string) string {
	if workspace = strings.TrimSpace(workspace); workspace == "" {
		return "local"
	}
	return workspace
}

func (r *codeStudioPreviewVersionRegistry) spaceLocked(workspace string) *codeStudioPreviewSpace {
	key := codeStudioPreviewSpaceKey(workspace)
	space := r.spaces[key]
	if space == nil {
		space = &codeStudioPreviewSpace{wake: make(chan struct{})}
		r.spaces[key] = space
	}
	return space
}

func (r *codeStudioPreviewVersionRegistry) Current(workspace string) uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.spaceLocked(workspace).version
}

func (r *codeStudioPreviewVersionRegistry) Bump(workspace string, paths ...string) uint64 {
	changed := make([]string, 0, len(paths))
	for _, p := range paths {
		rel := strings.Trim(strings.TrimPrefix(p, codeStudioWorkspaceRoot), "/")
		if rel == "" {
			changed = nil
			break
		}
		changed = append(changed, rel)
	}
	if len(changed) == 0 {
		changed = nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	space := r.spaceLocked(workspace)
	space.version++
	space.changes = append(space.changes, codeStudioPreviewChange{version: space.version, paths: changed})
	if extra := len(space.changes) - codeStudioPreviewHistory; extra > 0 {
		space.changes = append([]codeStudioPreviewChange(nil), space.changes[extra:]...)
	}
	close(space.wake)
	space.wake = make(chan struct{})
	return space.version
}

// changesLocked lists the paths changed after since. full is true when the kept history does not reach back that
// far, when one change named no paths, or when the client is ahead of the server (restart).
func (s *codeStudioPreviewSpace) changesLocked(since uint64) ([]string, bool) {
	if since > s.version {
		return nil, true
	}
	if since == s.version {
		return nil, false
	}
	full := len(s.changes) == 0 || s.changes[0].version > since+1
	seen := map[string]bool{}
	var paths []string
	for _, change := range s.changes {
		if change.version <= since {
			continue
		}
		if change.paths == nil {
			full = true
			continue
		}
		for _, p := range change.paths {
			if !seen[p] {
				seen[p] = true
				paths = append(paths, p)
			}
		}
	}
	if full {
		return nil, true
	}
	return paths, false
}

// Wait returns as soon as the version differs from since, or the unchanged version after timeout or cancellation.
func (r *codeStudioPreviewVersionRegistry) Wait(ctx context.Context, workspace string, since uint64, timeout time.Duration) (uint64, []string, bool) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		r.mu.Lock()
		space := r.spaceLocked(workspace)
		version := space.version
		if version != since {
			paths, full := space.changesLocked(since)
			r.mu.Unlock()
			return version, paths, full
		}
		wake := space.wake
		r.mu.Unlock()
		select {
		case <-wake:
		case <-ctx.Done():
			return version, nil, false
		case <-timer.C:
			return version, nil, false
		}
	}
}
