package server

import "testing"

func TestStoreTerminalExecCommandUsesCatalogMetadata(t *testing.T) {
	t.Parallel()

	got := storeTerminalExecCommand(map[string]string{"terminal_command": "cmd"}, true)
	want := []string{"/bin/bash", "-lc", "exec cmd"}
	if len(got) != len(want) {
		t.Fatalf("exec cmd = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("exec cmd[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestStoreTerminalExecCommandUsesPlainShellForAdditionalSessions(t *testing.T) {
	t.Parallel()

	got := storeTerminalExecCommand(map[string]string{"terminal_command": "cmd"}, false)
	want := defaultContainerTerminalCommand()
	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("exec cmd = %#v, want %#v", got, want)
	}
}

func TestStoreTerminalExecCommandFallsBackToShell(t *testing.T) {
	t.Parallel()

	for _, metadata := range []map[string]string{
		nil,
		{},
		{"terminal_command": ""},
		{"terminal_command": "bad;rm"},
	} {
		got := storeTerminalExecCommand(metadata, true)
		want := defaultContainerTerminalCommand()
		if len(got) != len(want) || got[0] != want[0] {
			t.Fatalf("metadata %#v => %#v, want %#v", metadata, got, want)
		}
	}
}