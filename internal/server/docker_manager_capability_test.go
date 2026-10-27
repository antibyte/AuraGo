package server

import (
	"testing"

	"aurago/internal/config"
)

// The "docker" desktop capability shows the Docker Manager app exactly while Docker is enabled. Docker's read-only
// switch does not hide it: host management stays allowed, and the app itself hides the container actions.
func TestDesktopCapabilityDockerFollowsDockerEnabled(t *testing.T) {
	s := &Server{Cfg: &config.Config{}}
	caps := desktopCapabilities{s: s}
	if caps.HasCapability("docker") {
		t.Fatal("the docker capability must be off while Docker is disabled")
	}
	s.Cfg.Docker.Enabled = true
	if !caps.HasCapability("docker") {
		t.Fatal("the docker capability must be on while Docker is enabled")
	}
	s.Cfg.Docker.ReadOnly = true
	if !caps.HasCapability("docker") {
		t.Fatal("Docker read-only must not hide the app")
	}
	if (desktopCapabilities{}).HasCapability("docker") {
		t.Fatal("without a server nothing is available")
	}
}
