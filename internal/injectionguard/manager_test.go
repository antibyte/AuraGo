package injectionguard

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == WorkerFlag {
		os.Exit(RunWorker(os.Args[2:], os.Stdin, os.Stdout, os.Stderr))
	}
	os.Exit(m.Run())
}

func TestManagerDisabledAndUnavailable(t *testing.T) {
	m := NewManager(context.Background(), t.TempDir(), "")
	defer m.Close()
	if m.Status().Enabled || m.Status().Ready {
		t.Fatal("optional model must start disabled and unprepared")
	}
	_, _, enabled, err := m.Evaluate(context.Background(), "hello")
	if enabled || err != nil {
		t.Fatal("disabled classifier must not run")
	}
	m.Configure(true, "quarantine")
	_, _, enabled, err = m.Evaluate(context.Background(), "hello")
	if !enabled || err == nil {
		t.Fatal("enabled missing model must fail closed")
	}
	if m.Status().Ready {
		t.Fatal("checking must not install a model")
	}
}

func TestManagerAdmissionBound(t *testing.T) {
	m := NewManager(context.Background(), t.TempDir(), "")
	defer m.Close()
	if err := m.acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	for range 8 {
		m.admission <- struct{}{}
	}
	if err := m.acquire(context.Background()); err == nil {
		t.Fatal("unbounded queue")
	}
	for range 8 {
		<-m.admission
	}
	m.release()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// A cancelled waiter must not occupy an admission slot.
	m.slot <- struct{}{}
	if err := m.acquire(ctx); err == nil {
		t.Fatal("cancelled waiter admitted")
	}
	<-m.slot
	if len(m.admission) != 0 {
		t.Fatal("leaked queue slot")
	}
}

func TestModelIntegration(t *testing.T) {
	root := os.Getenv("AURAGO_INJECTION_GUARD_CACHE")
	if root == "" || os.Getenv("AURAGO_INJECTION_GUARD_INTEGRATION") != "1" {
		t.Skip("explicit prepared-cache/integration opt-in required")
	}
	m := NewManager(context.Background(), root, "")
	defer m.Close()
	started := time.Now()
	if err := m.Prepare(); err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(5 * time.Minute); ; {
		s := m.Status()
		if s.Ready {
			break
		}
		if s.State == "error" {
			t.Fatalf("prepare: %s", s.Error)
		}
		if time.Now().After(deadline) {
			t.Fatal("prepare timeout")
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Logf("verified installation + cold self-test: %s", time.Since(started))
	if err := m.SelfTest(context.Background()); err != nil {
		t.Fatal(err)
	}
	m.Configure(true, "quarantine")
	for _, tc := range []struct {
		text   string
		attack bool
	}{{safeSample, false}, {attackSample, true}} {
		started = time.Now()
		p, _, enabled, err := m.Evaluate(context.Background(), tc.text)
		if err != nil || !enabled || (p >= Threshold) != tc.attack {
			t.Fatalf("score %.8f, enabled %v, err %v", p, enabled, err)
		}
		t.Logf("attack=%v score=%.8f elapsed=%s", tc.attack, p, time.Since(started))
	}
	m.Configure(false, "quarantine")
	if !m.Status().Ready {
		t.Fatal("disable discarded installation")
	}
}
