package server

import (
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"
)

func TestRuntimePermissionResolverFollowsPublishedSnapshot(t *testing.T) {
	cfg := &config.Config{}
	cfg.Docker.Enabled = true
	cfg.Agent.AllowShell = true
	s := &Server{Cfg: cfg}
	s.initConfigSnapshot()
	t.Cleanup(func() {
		tools.SetRuntimePermissionResolver(nil)
		tools.ClearRuntimePermissionsForTest()
	})
	s.bindRuntimePermissions()

	perms, configured := tools.CurrentRuntimePermissionsForTest()
	if !configured || !perms.DockerEnabled || !perms.AllowShell {
		t.Fatalf("initial process gate = configured:%v %+v, want the published snapshot", configured, perms)
	}

	updated := s.ConfigSnapshot().Clone()
	updated.Docker.Enabled = false
	updated.Agent.AllowShell = false
	s.replaceConfigSnapshot(updated)
	// A run that still carries the startup config writes it process-wide.
	tools.ConfigureRuntimePermissions(tools.RuntimePermissionsFromConfig(cfg))

	perms, configured = tools.CurrentRuntimePermissionsForTest()
	if !configured || perms.DockerEnabled || perms.AllowShell {
		t.Fatalf("process gate = configured:%v %+v, want docker and shell closed by the published snapshot", configured, perms)
	}
}
