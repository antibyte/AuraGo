package server

import "aurago/internal/tools"

// bindRuntimePermissions makes every direct tool gate follow the server's
// published config snapshot. Agent runs narrow a single dispatch through
// tools.WithRuntimePermissions and never write the process-wide gate.
func (s *Server) bindRuntimePermissions() {
	tools.SetRuntimePermissionResolver(func() tools.RuntimePermissions {
		return tools.RuntimePermissionsFromConfig(s.ConfigSnapshot())
	})
}
