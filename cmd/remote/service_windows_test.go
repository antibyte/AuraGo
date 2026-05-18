package main

import (
	"errors"
	"testing"
)

func TestWindowsServiceBinPathQuotesExecutableAndArgumentsForSC(t *testing.T) {
	got := windowsServiceBinPath(`C:\Program Files\AuraGo\aurago-remote.exe`)
	want := `""C:\Program Files\AuraGo\aurago-remote.exe" --foreground"`
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
