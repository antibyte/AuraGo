package tools

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestProcessRegistryTerminateReturnsKillFallbackError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	registry := NewProcessRegistry(logger)
	registry.Register(&ProcessInfo{
		PID:       4242,
		Process:   &os.Process{Pid: 4242},
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

func TestProcessInfoWriteTruncatesWhenBufferExceedsMaxSize(t *testing.T) {
	info := &ProcessInfo{
		PID:       2001,
		StartedAt: time.Now(),
		Alive:     true,
	}

	// First fill the buffer to near capacity
	fillSize := maxOutputSize - 100
	fillData := make([]byte, fillSize)
	for i := range fillData {
		fillData[i] = 'x'
	}
	info.Write(fillData)

	// Now write data that would exceed maxOutputSize
	extraData := make([]byte, 1024)
	for i := range extraData {
		extraData[i] = 'y'
	}

	n, err := info.Write(extraData)
	if err != nil {
		t.Fatalf("Write() returned error: %v", err)
	}
	if n != len(extraData) {
		t.Fatalf("Write() reported n=%d, expected %d", n, len(extraData))
	}

	// Buffer should be truncated to at most maxOutputSize.
	if info.OutputLen() > maxOutputSize {
		t.Fatalf("buffer len=%d, expected <= %d after truncation", info.OutputLen(), maxOutputSize)
	}
}

func TestProcessInfoWriteSystemMessageAppliesTruncation(t *testing.T) {
	info := &ProcessInfo{
		PID:       2002,
		StartedAt: time.Now(),
		Alive:     true,
	}

	// Fill buffer to near capacity
	largeData := make([]byte, maxOutputSize-50)
	for i := range largeData {
		largeData[i] = 'a'
	}
	info.Write(largeData)

	// Write a system message that would exceed maxOutputSize
	longMessage := strings.Repeat("X", 200)
	err := info.WriteSystemMessage(longMessage)
	if err != nil {
		t.Fatalf("WriteSystemMessage() returned error: %v", err)
	}

	// Buffer should be truncated to at most maxOutputSize.
	if info.OutputLen() > maxOutputSize {
		t.Fatalf("buffer len=%d after system message, expected <= %d", info.OutputLen(), maxOutputSize)
	}
}

func TestProcessInfoConcurrentWriteRead(t *testing.T) {
	info := &ProcessInfo{
		PID:       2003,
		StartedAt: time.Now(),
		Alive:     true,
	}

	const goroutines = 10
	const writesPerGoroutine = 100
	var wg sync.WaitGroup
	wg.Add(goroutines * 2) // writers + readers

	// Concurrent writers
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < writesPerGoroutine; j++ {
				msg := fmt.Sprintf("goroutine-%d-write-%d ", id, j)
				info.Write([]byte(msg))
			}
		}(i)
	}

	// Concurrent readers
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < writesPerGoroutine; j++ {
				_ = info.ReadOutput()
			}
		}()
	}

	wg.Wait()

	// Should not panic and output should be readable
	output := info.ReadOutput()
	if output == "" && t.Failed() == false {
		// Empty is acceptable if all data was truncated
	}
}

func TestProcessInfoWriteAndReadOutput(t *testing.T) {
	info := &ProcessInfo{
		PID:       2004,
		StartedAt: time.Now(),
		Alive:     true,
	}

	testData := []byte("hello world")
	n, err := info.Write(testData)
	if err != nil {
		t.Fatalf("Write() returned error: %v", err)
	}
	if n != len(testData) {
		t.Fatalf("Write() reported n=%d, expected %d", n, len(testData))
	}

	output := info.ReadOutput()
	if output != "hello world" {
		t.Fatalf("ReadOutput() returned %q, expected %q", output, "hello world")
	}
}

func TestProcessInfoWriteSystemMessagePrefixesWithNewline(t *testing.T) {
	info := &ProcessInfo{
		PID:       2005,
		StartedAt: time.Now(),
		Alive:     true,
	}

	info.Write([]byte("process output"))
	info.WriteSystemMessage("system message here")

	output := info.ReadOutput()
	if !strings.Contains(output, "\nsystem message here") {
		t.Fatalf("expected system message to be prefixed with newline, got %q", output)
	}
}

func TestProcessInfoWriteSystemMessageBytesNoNewline(t *testing.T) {
	info := &ProcessInfo{
		PID:       2006,
		StartedAt: time.Now(),
		Alive:     true,
	}

	info.Write([]byte("existing"))
	info.WriteSystemMessageBytes([]byte("direct bytes"))

	output := info.ReadOutput()
	if !strings.Contains(output, "direct bytes") {
		t.Fatalf("expected output to contain 'direct bytes', got %q", output)
	}
}

func TestProcessRegistryListDoesNotHoldRegistryLockWhileWaitingOnProcessInfo(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	registry := NewProcessRegistry(logger)
	blocked := &ProcessInfo{PID: 1001, StartedAt: time.Now(), Alive: true}
	other := &ProcessInfo{PID: 1002, StartedAt: time.Now(), Alive: true}
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

func TestProcessRegistryListIncludesCompletionDetails(t *testing.T) {
	registry := NewProcessRegistry(slog.New(slog.NewTextHandler(io.Discard, nil)))
	finished := time.Now().Add(-time.Minute)
	registry.Register(&ProcessInfo{
		PID:          3001,
		StartedAt:    finished.Add(-time.Second),
		Alive:        false,
		State:        ProcessStateCrashed,
		ExitCode:     9,
		TerminatedAt: finished,
		ErrorReason:  "exit status 9",
	})

	list := registry.List()
	if len(list) != 1 {
		t.Fatalf("list length = %d, want 1", len(list))
	}
	if list[0]["state"] != "crashed" || list[0]["exit_code"] != 9 {
		t.Fatalf("missing completion details: %#v", list[0])
	}
	if list[0]["finished_at"] == "" || list[0]["error_reason"] != "exit status 9" {
		t.Fatalf("missing finish metadata: %#v", list[0])
	}
}

func TestProcessRegistryPrunesExpiredAndExcessCompletedProcesses(t *testing.T) {
	registry := NewProcessRegistry(slog.New(slog.NewTextHandler(io.Discard, nil)))
	registry.completedRetention = time.Minute
	registry.maxCompleted = 2
	now := time.Now()

	registry.Register(&ProcessInfo{PID: 4000, StartedAt: now.Add(-3 * time.Minute), Alive: false, State: ProcessStateExited, TerminatedAt: now.Add(-2 * time.Minute)})
	for i := 1; i <= 3; i++ {
		registry.Register(&ProcessInfo{PID: 4000 + i, StartedAt: now.Add(time.Duration(i) * time.Second), Alive: false, State: ProcessStateExited, TerminatedAt: now.Add(time.Duration(i) * time.Second)})
	}

	list := registry.List()
	if len(list) != 2 {
		t.Fatalf("list length = %d, want newest 2 completed processes: %#v", len(list), list)
	}
	if _, ok := registry.Get(4000); ok {
		t.Fatal("expired completed process was not pruned")
	}
	if _, ok := registry.Get(4001); ok {
		t.Fatal("oldest excess completed process was not pruned")
	}
}
