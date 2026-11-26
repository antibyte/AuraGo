package desktop

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"path"
	"sync"
	"time"
)

// ---- pending host-key trust ----

const (
	hostKeyPendingTTL = 5 * time.Minute
	hostKeyPendingMax = 256
)

type pendingHostKeyStore struct {
	mu      sync.Mutex
	entries map[string]pendingHostKey // by device id
}

type pendingHostKey struct {
	fingerprint string
	expires     time.Time
}

// hostKeyPending remembers the unknown host key a file operation just showed
// the user, so the trust endpoint can only confirm that exact key.
var hostKeyPending = &pendingHostKeyStore{entries: map[string]pendingHostKey{}}

func (s *pendingHostKeyStore) sweepLocked(now time.Time) {
	for id, e := range s.entries {
		if !now.Before(e.expires) {
			delete(s.entries, id)
		}
	}
}

func (s *pendingHostKeyStore) add(deviceID, fingerprint string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := fileProbeNow()
	s.sweepLocked(now)
	if _, exists := s.entries[deviceID]; !exists && len(s.entries) >= hostKeyPendingMax {
		return // bounded: refuse to grow, the user simply gets a fresh prompt later
	}
	s.entries[deviceID] = pendingHostKey{fingerprint: fingerprint, expires: now.Add(hostKeyPendingTTL)}
}

// get returns the pending fingerprint of a device (empty when none/expired).
func (s *pendingHostKeyStore) get(deviceID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sweepLocked(fileProbeNow())
	return s.entries[deviceID].fingerprint
}

func (s *pendingHostKeyStore) consume(deviceID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, deviceID)
}

// ---- safe writes ----

// abortableWriter is implemented by writers that can discard a partial write
// instead of committing it.
type abortableWriter interface {
	Abort(err error)
}

func tempSiblingPath(dst string) string {
	var r [4]byte
	_, _ = rand.Read(r[:])
	name := "." + path.Base(dst) + ".aurago-" + hex.EncodeToString(r[:]) + ".part"
	dir := path.Dir(dst)
	if dir == "." || dir == "/" {
		return name
	}
	return path.Join(dir, name)
}

// writeRemoteFileSafely streams src into a temporary sibling of dst and renames
// it over dst only after the whole transfer and the writer's Close succeeded.
// On any failure dst stays untouched and the temporary file is removed.
func writeRemoteFileSafely(ctx context.Context, fsys remoteFS, dst string, src io.Reader) error {
	tmp := tempSiblingPath(dst)
	w, err := fsys.Create(ctx, tmp)
	if err != nil {
		return fmt.Errorf("create destination: %w", err)
	}
	cleanup := func() {
		rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = fsys.Remove(rmCtx, tmp, false)
	}
	if _, err := io.Copy(w, src); err != nil {
		if a, ok := w.(abortableWriter); ok {
			a.Abort(err)
		}
		_ = w.Close()
		cleanup()
		return err
	}
	if err := w.Close(); err != nil {
		cleanup()
		return err
	}
	if err := fsys.Replace(ctx, tmp, dst); err != nil {
		cleanup()
		return fmt.Errorf("replace destination: %w", err)
	}
	return nil
}
