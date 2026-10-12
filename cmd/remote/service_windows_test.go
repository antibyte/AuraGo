package main

import (
	"errors"
	"golang.org/x/sys/windows/svc"
	"io"
	"log/slog"
	"syscall"
	"testing"
	"time"
)

func TestWindowsServiceBinPathQuotesExecutableAndArgumentsForSC(t *testing.T) {
	got := windowsServiceBinPath(`C:\Program Files\AuraGo\aurago-remote.exe`)
	want := `"C:\Program Files\AuraGo\aurago-remote.exe" --config ` + syscall.EscapeArg(configPath())
	if got != want {
		t.Fatalf("windowsServiceBinPath() = %q, want %q", got, want)
	}
}

func TestIsRunningAsWindowsServiceUsesServiceProbeResult(t *testing.T) {
	if !isRunningAsWindowsService(func() (bool, error) { return true, nil }) {
		t.Fatal("expected true service probe to report service mode")
	}
	if isRunningAsWindowsService(func() (bool, error) { return false, nil }) {
		t.Fatal("expected false service probe to report non-service mode")
	}
	if isRunningAsWindowsService(func() (bool, error) { return false, errors.New("probe failed") }) {
		t.Fatal("expected service probe errors to report non-service mode")
	}
}

func TestWindowsServiceDispatcherHandlesStartInterrogateAndStop(t *testing.T) {
	client := &Client{logger: slog.New(slog.NewTextHandler(io.Discard, nil)), done: make(chan struct{})}
	service := &remoteWindowsService{client: client}
	requests := make(chan svc.ChangeRequest, 2)
	statuses := make(chan svc.Status, 8)
	done := make(chan struct{})
	go func() { service.Execute(nil, requests, statuses); close(done) }()
	for _, expected := range []svc.State{svc.StartPending, svc.Running} {
		select {
		case state := <-statuses:
			if state.State != expected {
				t.Fatalf("state %v, want %v", state.State, expected)
			}
		case <-time.After(time.Second):
			t.Fatal("service did not start")
		}
	}
	requests <- svc.ChangeRequest{Cmd: svc.Interrogate}
	select {
	case state := <-statuses:
		if state.State != svc.Running {
			t.Fatal("interrogate did not report running")
		}
	case <-time.After(time.Second):
		t.Fatal("interrogate blocked")
	}
	requests <- svc.ChangeRequest{Cmd: svc.Stop}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("service stop blocked")
	}
}
