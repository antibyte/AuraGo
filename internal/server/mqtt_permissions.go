package server

import "aurago/internal/tools"

// bindMQTTPermissions makes direct device publishing independent from the last
// agent's scoped permissions. Agent dispatch still enforces its own narrower
// run authorization before entering the bridge.
func (s *Server) bindMQTTPermissions() {
	tools.SetMQTTPermissionResolver(func() (bool, bool) {
		cfg := s.ConfigSnapshot()
		if cfg == nil || cfg.EggMode.Enabled {
			return false, true
		}
		return cfg.MQTT.Enabled, cfg.MQTT.ReadOnly
	})
}
