package agent

import (
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"
)

// useRuntimePermissionsForTest binds the process-wide tool gates to cfg for one
// test, the way the server binds them to its published snapshot. It mutates
// package-global state: never call it from a test that uses t.Parallel.
func useRuntimePermissionsForTest(t *testing.T, cfg *config.Config) {
	t.Helper()
	tools.SetRuntimePermissionResolver(func() tools.RuntimePermissions {
		return tools.RuntimePermissionsFromConfig(cfg)
	})
	t.Cleanup(func() { tools.SetRuntimePermissionResolver(nil) })
}
