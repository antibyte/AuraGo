package tools

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"
)

func TestProcessRegistryTerminateReturnsKillFallbackError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	registry := NewProcessRegistry(logger)
	registry.Register(&ProcessInfo{
		PID:       4242,
		Process:   &os.Process{Pid: 4242},
		Output:    &bytes.Buffer{},
		StartedAt: time.Now(),
		Alive:     true,
	})

	originalSignal := signalProcess
	originalKill := killProcess
	defer func() {
		signalProcess = originalSignal
		killProcess = originalKill
	}()

	signalProcess = func(process *os.Process, sig os.Signal) error {
		return errors.New("signal failed")
	}
	killProcess = func(process *os.Process) error {
		return errors.New("kill failed")
	}

	err := registry.Terminate(4242)
	if err == nil {
		t.Fatal("expected terminate error when signal and kill both fail")
	}
	if !strings.Contains(err.Error(), "kill fallback failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProcessRegistryKillAllLogsKillFailures(t *testing.T) {
	var logBuf strings.Builder
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	registry := NewProcessRegistry(logger)
	registry.Register(&ProcessInfo{
		PID:       5151,
		Process:   &os.Process{Pid: 5151},
		Output:    &bytes.Buffer{},
		StartedAt: time.Now(),
		Alive:     true,
	})

	originalKill := killProcess
	defer func() { killProcess = originalKill }()
	killProcess = func(process *os.Process) error {
		return errors.New("kill failed")
	}

	registry.KillAll()
	if !strings.Contains(logBuf.String(), "Failed to kill orphaned background process") {
		t.Fatalf("expected kill failure warning, got %q", logBuf.String())
	}
}

func TestProcessRegistryListDoesNotHoldRegistryLockWhileWaitingOnProcessInfo(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	registry := NewProcessRegistry(logger)
	blocked := &ProcessInfo{PID: 1001, Output: &bytes.Buffer{}, StartedAt: time.Now(), Alive: true}
	other := &ProcessInfo{PID: 1002, Output: &bytes.Buffer{}, StartedAt: time.Now(), Alive: true}
	registry.Register(blocked)
	registry.Register(other)

	blocked.mu.Lock()
	released := false
	defer func() {
		if !released {
			blocked.mu.Unlock()
		}
	}()

	listStarted := make(chan struct{})
	listDone := make(chan struct{})
	go func() {
		close(listStarted)
		_ = registry.List()
		close(listDone)
	}()
	<-listStarted
	time.Sleep(20 * time.Millisecond)

	removeDone := make(chan struct{})
	go func() {
		registry.Remove(other.PID)
		close(removeDone)
	}()

	select {
	case <-removeDone:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected Remove to proceed without waiting for List to release registry lock")
	}

	blocked.mu.Unlock()
	released = true

	select {
	case <-listDone:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected List to finish after blocked process mutex was released")
	}
}
