package server

import (
	"reflect"

	"aurago/internal/config"
	"aurago/internal/tools"
)

func prepareCloudflareConfigSave(s *Server, next *config.Config, explicit bool) (func(bool), error) {
	old := s.ConfigSnapshot()
	if !explicit && reflect.DeepEqual(old.CloudflareTunnel, next.CloudflareTunnel) && old.Docker.Enabled == next.Docker.Enabled && old.Docker.ReadOnly == next.Docker.ReadOnly && old.Docker.Host == next.Docker.Host && old.Homepage.Enabled == next.Homepage.Enabled && old.Homepage.WorkspacePath == next.Homepage.WorkspacePath && old.SQLite.HomepageRegistryPath == next.SQLite.HomepageRegistryPath {
		return func(bool) {}, nil
	}
	revokeDocker := old.Docker.Enabled && (!next.Docker.Enabled || !old.Docker.ReadOnly && next.Docker.ReadOnly)
	return tools.CloudflareTunnelPrepareConfigChange(cloudflareTunnelRuntimeConfig(old), cloudflareTunnelRuntimeConfig(next), s.Registry, s.Logger, revokeDocker)
}
