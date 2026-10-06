package dockerutil

import (
	"regexp"
	"strings"
)

// Self detection: which container does this process run in? The server's
// container protection and the security proxy placement share these rules.
//
// Podman is not covered: it creates no /.dockerenv and keeps the /etc files
// under overlay-containers/<id>/userdata/, so no ID is found there.

// defaultContainerHostnamePattern matches the hostname Docker assigns by
// default: the first twelve hex characters of the container ID (or more).
var defaultContainerHostnamePattern = regexp.MustCompile(`^[0-9a-f]{12,64}$`)

// mountinfoContainerIDPattern matches the mount root of the files Docker
// bind-mounts into every container from <data-root>/containers/<id>/. The
// root is /<id>/... when that containers directory is its own filesystem.
// containerIDFromMountinfo only applies it to mounts at the three /etc files.
var mountinfoContainerIDPattern = regexp.MustCompile(`(?:^|/)([0-9a-f]{64})/(?:hostname|hosts|resolv\.conf)$`)

// cgroupContainerIDPattern matches the cgroup v1 path of a Docker container,
// both the cgroupfs (/docker/<id>) and the systemd (docker-<id>.scope) form.
var cgroupContainerIDPattern = regexp.MustCompile(`(?:/docker/|/docker-)([0-9a-f]{64})(?:\.scope)?$`)

// DefaultContainerHostname returns hostname, trimmed and lower-cased, when it
// has the form of Docker's default hostname (a prefix of the container ID),
// else "". A custom hostname names nothing reliable: looked up as a container
// reference it may name another container.
func DefaultContainerHostname(hostname string) string {
	hostname = strings.ToLower(strings.TrimSpace(hostname))
	if defaultContainerHostnamePattern.MatchString(hostname) {
		return hostname
	}
	return ""
}

// OwnContainerID returns the 64-hex ID of the container this process runs in,
// or "" when /proc does not name exactly one. readFile reads the /proc files
// (os.ReadFile; tests pass fixtures). mountinfo comes first because it works
// for cgroup v1 and v2; /proc/self/cgroup names the container only on cgroup
// v1. The ID survives a custom hostname. When AuraGo joins another
// container's network namespace, the /etc files and so the ID belong to that
// container; callers that must tell the two apart check the container list.
func OwnContainerID(readFile func(string) ([]byte, error)) string {
	if data, err := readFile("/proc/self/mountinfo"); err == nil {
		if id := containerIDFromMountinfo(string(data)); id != "" {
			return id
		}
	}
	if data, err := readFile("/proc/self/cgroup"); err == nil {
		return containerIDFromCgroup(string(data))
	}
	return ""
}

// containerIDFromMountinfo reads the container ID from the /etc/hostname,
// /etc/hosts and /etc/resolv.conf bind mounts (mount point = field 5, mount
// root = field 4 of /proc/self/mountinfo). Other mounts are ignored; lines that
// disagree on the ID prove nothing.
func containerIDFromMountinfo(text string) string {
	found := ""
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		switch fields[4] {
		case "/etc/hostname", "/etc/hosts", "/etc/resolv.conf":
		default:
			continue
		}
		match := mountinfoContainerIDPattern.FindStringSubmatch(fields[3])
		if match == nil {
			continue
		}
		if found != "" && found != match[1] {
			return ""
		}
		found = match[1]
	}
	return found
}

// containerIDFromCgroup reads the container ID from cgroup v1 lines
// (hierarchy:controllers:path); lines that disagree prove nothing.
func containerIDFromCgroup(text string) string {
	found := ""
	for _, line := range strings.Split(text, "\n") {
		parts := strings.SplitN(strings.TrimSpace(line), ":", 3)
		if len(parts) != 3 {
			continue
		}
		match := cgroupContainerIDPattern.FindStringSubmatch(parts[2])
		if match == nil {
			continue
		}
		if found != "" && found != match[1] {
			return ""
		}
		found = match[1]
	}
	return found
}
