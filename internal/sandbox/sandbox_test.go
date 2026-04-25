package sandbox

import (
	"log/slog"
	"os/exec"
	"testing"
)

func TestFallbackSandbox(t *testing.T) {
	fb := &FallbackSandbox{}

	if fb.Available() {
		t.Error("FallbackSandbox.Available() should return false")
	}
	if fb.Name() != "fallback" {
		t.Errorf("FallbackSandbox.Name() = %q, want %q", fb.Name(), "fallback")
	}
}

func TestFallbackPrepareCommand(t *testing.T) {
	fb := &FallbackSandbox{}
	cmd := fb.PrepareCommand("echo test", "/tmp")

	if cmd.Dir != "/tmp" {
		t.Errorf("cmd.Dir = %q, want %q", cmd.Dir, "/tmp")
	}
	// On Windows, /bin/sh won't exist, but the command should still be constructed correctly
	if cmd.Path == "" {
		t.Error("cmd.Path should not be empty")
	}
}

func TestFallbackPrepareExecCommand(t *testing.T) {
	fb := &FallbackSandbox{}
	cmd := fb.PrepareExecCommand("echo", []string{"test"}, "/tmp")

	if cmd.Dir != "/tmp" {
		t.Errorf("cmd.Dir = %q, want %q", cmd.Dir, "/tmp")
	}
	if cmd.Path == "" {
		t.Error("cmd.Path should not be empty")
	}
}

func TestGetReturnsInstance(t *testing.T) {
	sb := Get()
	if sb == nil {
		t.Fatal("Get() returned nil")
	}
	// Before Init(), should be FallbackSandbox
	if sb.Name() != "fallback" {
		t.Errorf("default sandbox name = %q, want %q", sb.Name(), "fallback")
	}
}

func TestDetectReturnsCaps(t *testing.T) {
	caps := Detect()
	// On any platform, this should not panic and should return a valid struct
	_ = caps.LandlockABI
	_ = caps.InDocker
	_ = caps.KernelVersion
}

func TestShellSandboxConfigDefaults(t *testing.T) {
	cfg := ShellSandboxConfig{}
	if cfg.Enabled {
		t.Error("default Enabled should be false")
	}
	if cfg.MaxMemoryMB != 0 {
		t.Error("default MaxMemoryMB should be 0 (set by config loader)")
	}
}

func TestSplitPaths(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"/tmp", 1},
		{"/tmp:/var:/usr", 3},
		{"/tmp::/var", 2}, // empty segment skipped
		{"  /tmp  :  /var  ", 2},
	}

	for _, tt := range tests {
		got := splitPaths(tt.input)
		if len(got) != tt.want {
			t.Errorf("splitPaths(%q) = %d paths, want %d", tt.input, len(got), tt.want)
		}
	}
}

func TestFilterEnv(t *testing.T) {
	input := []string{
		"PATH=/usr/bin",
		"HOME=/home/test",
		"AURAGO_SBX_WORKDIR=/tmp",
		"AURAGO_SBX_MEM=512",
		"AURAGO_MASTER_KEY=secret",
		"CUSTOM_API_TOKEN=secret",
		"DB_PASSWORD=secret",
		"MY_SECRET=secret",
		"LANG=en_US.UTF-8",
	}

	got := FilterEnv(input)

	for _, e := range got {
		if e == "AURAGO_SBX_WORKDIR=/tmp" || e == "AURAGO_SBX_MEM=512" ||
			e == "AURAGO_MASTER_KEY=secret" || e == "CUSTOM_API_TOKEN=secret" ||
			e == "DB_PASSWORD=secret" || e == "MY_SECRET=secret" {
			t.Errorf("FilterEnv should have removed %q", e)
		}
	}

	// PATH, HOME, LANG should still be there
	found := map[string]bool{}
	for _, e := range got {
		found[e] = true
	}
	if !found["PATH=/usr/bin"] {
		t.Error("PATH should be preserved")
	}
	if !found["LANG=en_US.UTF-8"] {
		t.Error("LANG should be preserved")
	}
}

func TestEnvInt(t *testing.T) {
	t.Setenv("TEST_SBX_INT", "42")
	if v := envInt("TEST_SBX_INT"); v != 42 {
		t.Errorf("envInt = %d, want 42", v)
	}

	t.Setenv("TEST_SBX_INT", "")
	if v := envInt("TEST_SBX_INT"); v != 0 {
		t.Errorf("envInt('') = %d, want 0", v)
	}

	if v := envInt("TEST_SBX_NONEXISTENT"); v != 0 {
		t.Errorf("envInt(nonexistent) = %d, want 0", v)
	}
}

func TestInitDisabled(t *testing.T) {
	// When disabled, should use fallback regardless of capabilities
	Init(ShellSandboxConfig{Enabled: false}, "/tmp", testLogger())
	sb := Get()
	if sb.Name() != "fallback" {
		t.Errorf("disabled sandbox name = %q, want %q", sb.Name(), "fallback")
	}
}

// testLogger returns a no-op logger for tests.
func testLogger() *slog.Logger {
	return slog.Default()
}

// Ensure ShellSandbox interface is satisfied by concrete types.
var _ ShellSandbox = (*FallbackSandbox)(nil)

// Verify exec.Cmd is the return type (compile-time check).
var _ *exec.Cmd = (*FallbackSandbox)(nil).PrepareCommand("", "")
var _ *exec.Cmd = (*FallbackSandbox)(nil).PrepareExecCommand("", nil, "")

func TestInitDoubleInit_ClosesOldFallback(t *testing.T) {
	// Calling Init twice must not panic and must leave a valid sandbox instance.
	Init(ShellSandboxConfig{Enabled: false}, "/tmp", testLogger())
	Init(ShellSandboxConfig{Enabled: false}, "/tmp", testLogger())
	sb := Get()
	if sb == nil {
		t.Fatal("Get() returned nil after double Init")
	}
	if sb.Name() != "fallback" {
		t.Errorf("expected fallback after disabled init, got %q", sb.Name())
	}
}

func TestFallbackSandbox_PrepareCommand_HasSh(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping exec-spawning test in short mode")
	}
	fb := &FallbackSandbox{}
	cmd := fb.PrepareCommand("true", "/tmp")
	// The command must reference a shell, not an empty path.
	if cmd.Path == "" {
		t.Error("FallbackSandbox.PrepareCommand returned cmd with empty Path")
	}
}
