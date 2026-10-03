package tools

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

func TestRuntimePermissionResolverIgnoresStaleProcessSnapshot(t *testing.T) {
	// A stale agent-scoped write must not re-open gates the server closed.
	ConfigureRuntimePermissions(RuntimePermissions{
		DockerEnabled:            true,
		AllowShell:               true,
		AllowUnsafeHostExecution: true,
		AllowFilesystemWrite:     true,
	})
	SetRuntimePermissionResolver(func() RuntimePermissions { return RuntimePermissions{} })
	t.Cleanup(func() {
		SetRuntimePermissionResolver(nil)
		ConfigureRuntimePermissions(defaultRuntimePermissionsForTests())
	})

	if err := requireDockerPermission(); err == nil {
		t.Fatal("stale process snapshot re-opened docker")
	}
	if err := requireShellPermission(); err == nil {
		t.Fatal("stale process snapshot re-opened shell execution")
	}
	if err := requireFilesystemWritePermission(); err == nil {
		t.Fatal("stale process snapshot re-opened filesystem writes")
	}
}

func TestRuntimePermissionsContextOnlyNarrowsServerSnapshot(t *testing.T) {
	SetRuntimePermissionResolver(func() RuntimePermissions {
		return RuntimePermissions{AllowShell: true, AllowPython: true, AllowUnsafeHostExecution: true, DockerEnabled: true}
	})
	t.Cleanup(func() {
		SetRuntimePermissionResolver(nil)
		ConfigureRuntimePermissions(defaultRuntimePermissionsForTests())
	})

	narrowed := WithRuntimePermissions(context.Background(), RuntimePermissions{
		AllowPython:              true,
		AllowUnsafeHostExecution: true,
		DockerEnabled:            true,
		DockerReadOnly:           true,
	})
	if err := requireShellPermissionContext(narrowed); err == nil || !strings.Contains(err.Error(), "shell execution is disabled") {
		t.Fatalf("narrowed shell gate error = %v, want shell disabled", err)
	}
	if err := requirePythonPermissionContext(narrowed); err != nil {
		t.Fatalf("narrowed python gate error = %v, want allowed", err)
	}
	if perms, _ := EffectiveRuntimePermissions(narrowed); !perms.DockerReadOnly {
		t.Fatalf("run-scoped read-only restriction was dropped: %+v", perms)
	}

	widened := WithRuntimePermissions(context.Background(), RuntimePermissions{
		AllowShell:               true,
		AllowPython:              true,
		AllowUnsafeHostExecution: true,
		AllowFilesystemWrite:     true,
	})
	perms, configured := EffectiveRuntimePermissions(widened)
	if !configured || perms.AllowFilesystemWrite || perms.DockerEnabled {
		t.Fatalf("run context widened the server snapshot: configured=%v perms=%+v", configured, perms)
	}
}

func TestRuntimePermissionsContextIsAuthoritativeWithoutServerSnapshot(t *testing.T) {
	SetRuntimePermissionResolver(nil)
	ClearRuntimePermissionsForTest()
	t.Cleanup(func() { ConfigureRuntimePermissions(defaultRuntimePermissionsForTests()) })

	ctx := WithRuntimePermissions(context.Background(), RuntimePermissions{AllowPython: true, AllowUnsafeHostExecution: true})
	if err := requirePythonPermissionContext(ctx); err != nil {
		t.Fatalf("standalone run-scoped python gate error = %v, want allowed", err)
	}
	if err := requirePythonPermission(); err == nil {
		t.Fatal("context-free python gate must stay closed without a configured snapshot")
	}
}

// intersectRuntimePermissions builds its result field by field. A new
// RuntimePermissions field that is not added there would silently be zeroed
// for every context-aware gate, so this test pins the field count.
func TestIntersectRuntimePermissionsCoversEveryField(t *testing.T) {
	const handledFields = 19
	if got := reflect.TypeOf(RuntimePermissions{}).NumField(); got != handledFields {
		t.Fatalf("RuntimePermissions has %d fields but intersectRuntimePermissions handles %d; add the new field there and update this count", got, handledFields)
	}
}
