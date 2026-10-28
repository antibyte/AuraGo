package injectionguard

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type blockedScorer struct {
	entered, closed chan struct{}
	once            sync.Once
	fail            bool
}

func (w *blockedScorer) Score(ctx context.Context, _ string) (float64, error) {
	select {
	case w.entered <- struct{}{}:
	default:
	}
	if w.fail {
		return 0, errors.New("worker crashed")
	}
	select {
	case <-w.closed:
		return .01, nil
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}
func (w *blockedScorer) Close() { w.once.Do(func() { close(w.closed) }) }
func fakeReadyManager(t *testing.T) (*Manager, *blockedScorer) {
	t.Helper()
	m := NewManager(context.Background(), t.TempDir(), "")
	m.Configure(true, "quarantine")
	w := &blockedScorer{entered: make(chan struct{}, 1), closed: make(chan struct{})}
	m.worker = w
	m.status.Ready = true
	t.Cleanup(m.Close)
	return m, w
}

func TestManagerRevocationInvalidatesLateScore(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		m, w := fakeReadyManager(t)
		done := make(chan error, 1)
		go func() { _, _, _, err := m.Evaluate(context.Background(), "ordinary news"); done <- err }()
		<-w.entered
		m.Configure(enabled, "datamark")
		if err := <-done; err == nil {
			t.Fatal("revoked decision was accepted")
		}
		if m.Status().State != "ready" {
			t.Fatal("late failure overwrote new configuration status")
		}
	}
}

func TestManagerTimeoutCrashAndQueueRelease(t *testing.T) {
	for _, crash := range []bool{false, true} {
		m, w := fakeReadyManager(t)
		w.fail = crash
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		_, _, enabled, err := m.Evaluate(ctx, "ordinary news")
		cancel()
		if !enabled || err == nil {
			t.Fatal("failure allowed content")
		}
		if len(m.admission) != 0 || len(m.slot) != 0 || m.worker != nil {
			t.Fatal("failed worker or queue leaked")
		}
		select {
		case <-w.closed:
		default:
			t.Fatal("worker not stopped")
		}
	}
}

func TestManagerStartupAndCancelDoNotInstall(t *testing.T) {
	m := NewManager(context.Background(), t.TempDir(), "")
	defer m.Close()
	m.Inspect()
	if _, err := os.Stat(m.modelDir); !os.IsNotExist(err) {
		t.Fatal("startup created installation")
	}
	// Hold execution admission so cancelling installation cannot make a network call.
	if err := m.acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := m.Prepare(); err != nil {
		t.Fatal(err)
	}
	m.CancelPrepare()
	m.mu.Lock()
	done := m.jobDone
	m.mu.Unlock()
	<-done
	m.release()
	if m.Status().Ready || m.Status().Error != "cancelled" {
		t.Fatalf("%+v", m.Status())
	}
	if _, err := os.Stat(m.modelDir); !os.IsNotExist(err) {
		t.Fatal("cancelled queued download created files")
	}
}

func TestWorkerRejectsDamagedArtifactsBeforeNativeLoad(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ModelAsset.FileName), []byte("corrupt model"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := openEngine("", dir); err == nil {
		t.Fatal("corrupt model accepted")
	}
	var out strings.Builder
	if RunWorker([]string{"--runtime", "missing", "--models", dir}, strings.NewReader(""), &out, &out) == 0 {
		t.Fatal("broken worker became ready")
	}
	if strings.Contains(out.String(), "corrupt model") {
		t.Fatal("worker exposed content")
	}
}
