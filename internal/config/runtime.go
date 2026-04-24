package config

import (
	"bytes"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// Runtime holds auto-detected environment capabilities.
// All fields are computed at startup — nothing is persisted to YAML.
type Runtime struct {
	IsDocker         bool `json:"is_docker"`
	DockerSocketOK   bool `json:"docker_socket_ok"`
	BroadcastOK      bool `json:"broadcast_ok"`
	FirewallAccessOK bool `json:"firewall_access_ok"`
	// NoNewPrivileges is true when the kernel flag PR_SET_NO_NEW_PRIVS is active.
	// This prevents sudo (setuid escalation) from working regardless of config.
	NoNewPrivileges     bool `json:"no_new_privileges"`
	ProtectSystemStrict bool `json:"protect_system_strict"`
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

	// 5. No-new-privileges kernel flag — if set, sudo cannot escalate privileges.
	rt.NoNewPrivileges = probeNoNewPrivileges()
	if rt.NoNewPrivileges {
		logger.Warn("[Runtime] No-new-privileges flag is SET — sudo cannot escalate. If using the systemd service, comment out 'NoNewPrivileges=true' in the unit file and reload systemd.")
	} else {
		logger.Info("[Runtime] No-new-privileges flag", "set", false)
	}

	return rt
}

// ComputeFeatureAvailability maps each config section to its runtime availability.
// sudoEnabled should be cfg.Agent.SudoEnabled — when true, firewall access is
// considered available even if passwordless sudo is not configured, because the
// agent can supply the vault-stored password via sudo -S.
func ComputeFeatureAvailability(rt Runtime, sudoEnabled bool) map[string]FeatureAvailability {
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
	} else if rt.NoNewPrivileges {
		// PR_SET_NO_NEW_PRIVS is active — sudo's setuid escalation is blocked at kernel level.
		avail["sudo"] = FeatureAvailability{
			Available: false,
			Reason:    "The \"no new privileges\" flag is set — sudo cannot escalate. If using the systemd service, comment out 'NoNewPrivileges=true' in the unit file and run: sudo systemctl daemon-reload && sudo systemctl restart aurago",
		}
		avail["firewall"] = FeatureAvailability{
			Available: rt.FirewallAccessOK, // only available if process already has direct iptables access
			Reason:    boolReason(!rt.FirewallAccessOK, "iptables/ufw not accessible and sudo is blocked by the \"no new privileges\" flag. Comment out 'NoNewPrivileges=true' in the systemd unit file."),
		}
	} else {
		firewallAvail := rt.FirewallAccessOK || sudoEnabled
		avail["firewall"] = FeatureAvailability{Available: firewallAvail, Reason: boolReason(!firewallAvail, "iptables/ufw not accessible. Run as root, add NOPASSWD sudo for iptables, or enable sudo in the Danger Zone settings.")}
		avail["sudo"] = FeatureAvailability{Available: true}
	}

	// Sudo unrestricted — available only when ProtectSystem=strict is NOT active.
	if rt.ProtectSystemStrict {
		avail["sudo_unrestricted"] = FeatureAvailability{
			Available: false,
			Reason:    "ProtectSystem=strict is active in the systemd unit. To allow system-wide sudo writes, run: sudo systemctl edit --full aurago, comment out or remove ProtectSystem=strict, then run: sudo systemctl daemon-reload && sudo systemctl restart aurago",
		}
	} else {
		avail["sudo_unrestricted"] = FeatureAvailability{Available: true}
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
	avail["updates"] = FeatureAvailability{Available: !rt.IsDocker, Reason: func() string {
		if rt.IsDocker {
			return "Self-updates are disabled in Docker installations. Update the container image and recreate the container instead."
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
//   - /proc/self/cgroup contains a path element "docker"            (cgroup v1)
//   - /proc/1/cpuset path starts with "/docker/"
//   - /proc/self/mountinfo shows an overlay root filesystem          (cgroup v2)
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

	// Check cgroup for docker path (cgroup v1)
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

	// cgroup v2 fallback: Docker mounts an overlay filesystem at the container
	// root.  On modern kernels (Ubuntu 22.04+) with cgroup v2 enabled, the
	// previous cgroup and cpuset checks won't fire, but /proc/self/mountinfo
	// will have an overlay entry for "/".
	if mi, err := os.ReadFile("/proc/self/mountinfo"); err == nil {
		for _, line := range bytes.Split(mi, []byte{'\n'}) {
			// A Docker root overlay line looks like:
			//   <major:minor> / / rw ... - overlay overlay rw,lowerdir=...
			// We match lines where the mount point is " / " and the fs type is "overlay".
			fields := bytes.Fields(line)
			if len(fields) >= 9 && bytes.Equal(fields[4], []byte("/")) {
				// Find the separator "-" and check fs type after it
				for i, f := range fields {
					if bytes.Equal(f, []byte("-")) && i+1 < len(fields) {
						if bytes.Equal(fields[i+1], []byte("overlay")) {
							return true
						}
						break
					}
				}
			}
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

// probeNoNewPrivileges checks whether the kernel's no-new-privileges flag is
// active for this process by reading /proc/self/status.  When set, sudo cannot
// escalate privileges via the setuid bit.
// probeProtectSystemStrict checks whether the process is running under
// ProtectSystem=strict (or a similar read-only root mount) by attempting
// to create a file in /etc. It immediately cleans up the test file.
func probeProtectSystemStrict() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	testPath := fmt.Sprintf("/etc/.aurago_rw_test_%d", os.Getpid())
	f, err := os.Create(testPath)
	if err != nil {
		return strings.Contains(err.Error(), "read-only file system")
	}
	f.Close()
	_ = os.Remove(testPath)
	return false
}

func probeNoNewPrivileges() bool {
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		// Not Linux or unreadable — assume not set.
		return false
	}
	for _, line := range bytes.Split(data, []byte{'\n'}) {
		if bytes.HasPrefix(line, []byte("NoNewPrivs:")) {
			fields := bytes.Fields(line)
			return len(fields) >= 2 && string(fields[1]) == "1"
		}
	}
	return false
}
