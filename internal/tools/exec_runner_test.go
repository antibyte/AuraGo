package tools

import (
	"log/slog"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestForegroundRunner_BasicExecution(t *testing.T) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", "Write-Output 'hello from runner'")
	} else {
		cmd = exec.Command("/bin/sh", "-c", "echo 'hello from runner'")
	}

	runner := NewForegroundRunner(cmd, ForegroundOptions{
		Timeout:  5 * time.Second,
		Graceful: runtime.GOOS != "windows",
		KillWait: 2 * time.Second,
	})

	stdout, stderr, err := runner.Run()
	if err != nil {
		t.Fatalf("ForegroundRunner.Run() error = %v, stderr = %s", err, stderr)
	}
	if !strings.Contains(stdout, "hello from runner") {
		t.Errorf("expected stdout to contain 'hello from runner', got: %s", stdout)
	}
}

func TestForegroundRunner_TimeoutKillsProcess(t *testing.T) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", "Start-Sleep -Seconds 10")
	} else {
		cmd = exec.Command("/bin/sh", "-c", "sleep 10")
	}

	runner := NewForegroundRunner(cmd, ForegroundOptions{
		Timeout:  500 * time.Millisecond,
		Graceful: runtime.GOOS != "windows",
		KillWait: 2 * time.Second,
	})

	stdout, stderr, err := runner.Run()
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "TIMEOUT") {
		t.Errorf("expected error to contain 'TIMEOUT', got: %v", err)
	}
	// Stdout/stderr should be empty or minimal since process was killed quickly
	_ = stdout
	_ = stderr
}

func TestForegroundRunner_CommandError(t *testing.T) {
	cmd := exec.Command("nonexistent-command-12345")

	runner := NewForegroundRunner(cmd, ForegroundOptions{
		Timeout:  5 * time.Second,
		Graceful: false,
		KillWait: 2 * time.Second,
	})

	_, _, err := runner.Run()
	if err == nil {
		t.Fatal("expected error for nonexistent command, got nil")
	}
}

func TestForegroundRunner_DefaultTimeout(t *testing.T) {
	// Ensure default timeout is applied when Timeout is 0
	originalTimeout := GetForegroundTimeout()
	SetForegroundTimeout(10 * time.Second)
	defer SetForegroundTimeout(originalTimeout)

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", "Write-Output 'test'")
	} else {
		cmd = exec.Command("/bin/sh", "-c", "echo 'test'")
	}

	runner := NewForegroundRunner(cmd, ForegroundOptions{
		Timeout: 0, // Should use default
	})

	if runner.opts.Timeout != 10*time.Second {
		t.Errorf("expected default timeout of 10s, got %v", runner.opts.Timeout)
	}
}

func TestForegroundRunner_DefaultKillWait(t *testing.T) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", "echo test")
	} else {
		cmd = exec.Command("/bin/sh", "-c", "echo test")
	}

	runner := NewForegroundRunner(cmd, ForegroundOptions{
		Timeout:  5 * time.Second,
		KillWait: 0, // Should default to 10s
	})

	if runner.opts.KillWait != 10*time.Second {
		t.Errorf("expected default kill wait of 10s, got %v", runner.opts.KillWait)
	}
}

func TestForegroundRunner_CustomErrMsg(t *testing.T) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", "Start-Sleep -Seconds 10")
	} else {
		cmd = exec.Command("/bin/sh", "-c", "sleep 10")
	}

	runner := NewForegroundRunner(cmd, ForegroundOptions{
		Timeout:  500 * time.Millisecond,
		Graceful: runtime.GOOS != "windows",
		KillWait: 2 * time.Second,
		ErrMsg:   "CUSTOM TIMEOUT: process ran too long (%s)",
	})

	_, _, err := runner.Run()
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "CUSTOM TIMEOUT") {
		t.Errorf("expected error to contain 'CUSTOM TIMEOUT', got: %v", err)
	}
}

func TestBackgroundRunner_BasicExecution(t *testing.T) {
	registry := NewProcessRegistry(slog.Default())

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", "Start-Sleep -Milliseconds 100; Write-Output 'done'")
	} else {
		cmd = exec.Command("/bin/sh", "-c", "sleep 0.1; echo 'done'")
	}

	runner := NewBackgroundRunner(cmd, BackgroundOptions{
		Registry: registry,
	})

	pid, err := runner.Run()
	if err != nil {
		t.Fatalf("BackgroundRunner.Run() error = %v", err)
	}
	if pid <= 0 {
		t.Errorf("expected positive PID, got %d", pid)
	}

	// Give process time to start
	time.Sleep(100 * time.Millisecond)

	// Check process is in registry (may have been cleaned up by supervisor by now)
	info, ok := registry.Get(pid)
	if ok {
		t.Logf("process %d is in registry with state %v", pid, info.State)
	} else {
		t.Logf("process %d has already been cleaned up by supervisor", pid)
	}
}
