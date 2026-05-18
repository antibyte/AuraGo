package main

import "testing"

func TestIsRunningAsLinuxServiceUsesSystemdEnvironment(t *testing.T) {
	if !isRunningAsLinuxService("abc", "", func(int) string { return "" }, 123) {
		t.Fatal("INVOCATION_ID should indicate service mode")
	}
	if !isRunningAsLinuxService("", "/run/systemd/notify", func(int) string { return "" }, 123) {
		t.Fatal("NOTIFY_SOCKET should indicate service mode")
	}
}

func TestIsRunningAsLinuxServiceUsesParentProcessName(t *testing.T) {
	if !isRunningAsLinuxService("", "", func(pid int) string {
		if pid != 123 {
			t.Fatalf("parent lookup pid = %d, want 123", pid)
		}
		return "systemd"
	}, 123) {
		t.Fatal("systemd parent should indicate service mode")
	}
	if isRunningAsLinuxService("", "", func(int) string { return "bash" }, 123) {
		t.Fatal("non-systemd parent should not indicate service mode")
	}
}
