package main

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestShutdownRequestIsIdempotent(t *testing.T) {
	done, requestShutdown := setupGracefulShutdown(slog.Default())
	defer requestShutdown()
	var requests sync.WaitGroup
	for range 16 {
		requests.Go(requestShutdown)
	}
	requests.Wait()
	select {
	case <-done:
	default:
		t.Fatal("shutdown request did not notify the server")
	}
}

func TestShutdownSignalNotifiesServer(t *testing.T) {
	if os.Getenv("AURAGO_TEST_SHUTDOWN_SIGNAL") == "1" {
		done, requestShutdown := setupGracefulShutdown(slog.Default())
		defer requestShutdown()
		_, _ = io.WriteString(os.Stdout, "shutdown-ready\n")
		<-done
		return
	}
	if runtime.GOOS == "windows" {
		t.Skip("sending SIGTERM requires Unix")
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, executable, "-test.run=^TestShutdownSignalNotifiesServer$")
	command.Env = append(os.Environ(), "AURAGO_TEST_SHUTDOWN_SIGNAL=1")
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	command.Stderr = os.Stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = command.Process.Kill() })
	ready := bufio.NewScanner(stdout)
	if !ready.Scan() || ready.Text() != "shutdown-ready" {
		t.Fatalf("shutdown helper did not become ready: %v", ready.Err())
	}
	if err := command.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	if err := command.Wait(); err != nil {
		t.Fatalf("SIGTERM did not produce a clean shutdown: %v", err)
	}
}
