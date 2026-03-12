package config

import (
	"bytes"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"time"
)

// Runtime holds auto-detected environment capabilities.
// All fields are computed at startup — nothing is persisted to YAML.
type Runtime struct {
	IsDocker         bool `json:"is_docker"`
	DockerSocketOK   bool `json:"docker_socket_ok"`
	BroadcastOK      bool `json:"broadcast_ok"`
	FirewallAccessOK bool `json:"firewall_access_ok"`
}

// FeatureAvailability describes whether a config section is usable
// in the current runtime environment.
type FeatureAvailability struct {
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

// DetectRuntime probes the environment once at startup and populates
// the Runtime struct. Designed to be safe and fast (≤ 6 s worst case).
func DetectRuntime(logger *slog.Logger) Runtime {
	rt := Runtime{}
	logger.Info("[Runtime] Detecting environment capabilities …")

	// 1. Docker container detection.
	// /.dockerenv alone is NOT a reliable signal — Proxmox LXC and other
	// container runtimes may also have this file.  We require at least one
	// additional positive signal before setting IsDocker=true.
	rt.IsDocker = probeDockerContainer()
	logger.Info("[Runtime] Container check", "is_docker", rt.IsDocker)

	// 2. Docker socket reachability — try common paths / configured host.
	// We import tools.DockerPing indirectly to avoid a circular import;
	// instead we do a minimal dial to the socket.
	rt.DockerSocketOK = probeDockerSocket()
	logger.Info("[Runtime] Docker socket", "ok", rt.DockerSocketOK)

	// 3. Broadcast network — send a single UDP packet to 255.255.255.255:9.
	//    If the OS returns an immediate error, broadcast is blocked (Docker bridge).
	rt.BroadcastOK = probeBroadcast()
	logger.Info("[Runtime] Broadcast network", "ok", rt.BroadcastOK)

	// 4. Firewall access — try running `iptables -S` to see if we have permission.
	if rt.IsDocker {
		rt.FirewallAccessOK = probeFirewall()
	} else {
		// On bare-metal, iptables may or may not work depending on sudo config.
		// We still probe, but don't gate on IsDocker.
		rt.FirewallAccessOK = probeFirewall()
	}
	logger.Info("[Runtime] Firewall access", "ok", rt.FirewallAccessOK)

	return rt
}

// ComputeFeatureAvailability maps each config section to its runtime availability.
func ComputeFeatureAvailability(rt Runtime) map[string]FeatureAvailability {
	avail := make(map[string]FeatureAvailability)

	if rt.IsDocker {
		// Firewall — never works in Docker
		avail["firewall"] = FeatureAvailability{
			Available: false,
			Reason:    "Firewall management is not available inside a Docker container.",
		}
		// Sudo — not meaningful in Docker
		avail["sudo"] = FeatureAvailability{
			Available: false,
			Reason:    "Sudo commands are not available inside a Docker container.",
		}
	} else {
		avail["firewall"] = FeatureAvailability{Available: rt.FirewallAccessOK, Reason: boolReason(!rt.FirewallAccessOK, "iptables/ufw not accessible (missing sudo?).")}
		avail["sudo"] = FeatureAvailability{Available: true}
	}

	// Docker socket
	// Only report the socket as "unavailable" (with a reason) when we are
	// actually running inside a Docker container — outside Docker, the socket
	// simply being absent is normal (Docker not installed / not needed).
	socketReason := ""
	if rt.IsDocker && !rt.DockerSocketOK {
		socketReason = "Docker socket not detected. Mount /var/run/docker.sock to enable."
	}
	avail["docker"] = FeatureAvailability{Available: rt.DockerSocketOK || !rt.IsDocker, Reason: socketReason}
	avail["sandbox"] = FeatureAvailability{Available: rt.DockerSocketOK || !rt.IsDocker, Reason: func() string {
		if rt.IsDocker && !rt.DockerSocketOK {
			return "Sandbox requires Docker socket. Mount /var/run/docker.sock to enable."
		}
		return ""
	}()}
	avail["homepage_docker"] = FeatureAvailability{Available: rt.DockerSocketOK || !rt.IsDocker, Reason: func() string {
		if rt.IsDocker && !rt.DockerSocketOK {
			return "Docker-based development requires the Docker socket. Local file server still works."
		}
		return ""
	}()}
	avail["invasion_local"] = FeatureAvailability{Available: rt.DockerSocketOK || !rt.IsDocker, Reason: func() string {
		if rt.IsDocker && !rt.DockerSocketOK {
			return "Local Docker deployment requires the Docker socket. SSH deployment still works."
		}
		return ""
	}()}

	// Broadcast network (WOL, Chromecast discovery)
	avail["wol"] = FeatureAvailability{Available: rt.BroadcastOK, Reason: boolReason(!rt.BroadcastOK, "Wake-on-LAN requires broadcast network. Use network_mode: host in Docker.")}
	avail["chromecast_discovery"] = FeatureAvailability{Available: rt.BroadcastOK, Reason: boolReason(!rt.BroadcastOK, "Chromecast discovery requires mDNS/broadcast. Manual IP entry still works.")}

	return avail
}

func boolReason(show bool, reason string) string {
	if show {
		return reason
	}
	return ""
}

// probeDockerContainer returns true only when the process is running inside a
// Docker container.  /.dockerenv alone is not a reliable indicator — Proxmox
// LXC containers and other runtimes create it too.  We require /.dockerenv
// PLUS at least one of:
//   - /proc/1/environ contains "container=docker"  (set by Docker's init)
//   - /proc/self/cgroup contains a path element "docker"
//   - /proc/1/cpuset path starts with "/docker/"
//
// Systemd-only containers (LXC, nspawn, etc.) will fail ALL secondary checks
// even if /.dockerenv happens to exist.
func probeDockerContainer() bool {
	if _, err := os.Stat("/.dockerenv"); err != nil {
		return false // no /.dockerenv — definitely not Docker
	}

	// Check /proc/1/environ for container=docker
	if env, err := os.ReadFile("/proc/1/environ"); err == nil {
		for _, kv := range bytes.Split(env, []byte{0}) {
			if bytes.Equal(kv, []byte("container=docker")) {
				return true
			}
		}
	}

	// Check cgroup for docker path
	if cg, err := os.ReadFile("/proc/self/cgroup"); err == nil {
		if bytes.Contains(cg, []byte("docker")) {
			return true
		}
	}

	// Check cpuset for /docker/ prefix
	if cs, err := os.ReadFile("/proc/1/cpuset"); err == nil {
		line := bytes.TrimSpace(cs)
		if bytes.HasPrefix(line, []byte("/docker/")) {
			return true
		}
	}

	return false
}

// probeDockerSocket tries connecting to common Docker socket paths.
func probeDockerSocket() bool {
	paths := []string{"/var/run/docker.sock", "/run/docker.sock"}
	for _, p := range paths {
		if fi, err := os.Stat(p); err == nil && fi.Mode().Type() == os.ModeSocket {
			conn, err := net.DialTimeout("unix", p, 2*time.Second)
			if err == nil {
				conn.Close()
				return true
			}
		}
	}
	return false
}

// probeBroadcast sends a single UDP datagram to 255.255.255.255:9.
// If the kernel immediately rejects the packet, broadcast is not available.
func probeBroadcast() bool {
	conn, err := net.DialTimeout("udp4", "255.255.255.255:9", 2*time.Second)
	if err != nil {
		return false
	}
	defer conn.Close()
	// Try to actually write a byte
	conn.SetWriteDeadline(time.Now().Add(1 * time.Second))
	_, err = conn.Write([]byte{0x00})
	return err == nil
}

// probeFirewall checks whether iptables is accessible.
func probeFirewall() bool {
	cmd := exec.Command("iptables", "-S")
	if err := cmd.Run(); err != nil {
		// Also try with sudo in case of non-root
		cmd2 := exec.Command("sudo", "-n", "iptables", "-S")
		return cmd2.Run() == nil
	}
	return true
}
