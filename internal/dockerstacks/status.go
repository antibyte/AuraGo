package dockerstacks

import (
	"cmp"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"aurago/internal/dockerhost"
)

var reExitedStatus = regexp.MustCompile(`^Exited \((-?\d+)\)`)

// exitCodeOf reads the exit code out of the status text of an exited
// container ("Exited (137) 3 minutes ago"); the list API has no exit code field.
func exitCodeOf(c dockerhost.ContainerSummary) (int, bool) {
	m := reExitedStatus.FindStringSubmatch(c.Status)
	if m == nil {
		return 0, false
	}
	code, err := strconv.Atoi(m[1])
	return code, err == nil
}

// DeriveStatus applies the status rules to the containers of one project:
//
//   - no containers: draft (files only);
//   - every container running and none unhealthy: running;
//   - some running, others down, unhealthy or restarting: partial;
//   - nothing running and a container restarting, dead or exited with a
//     non-zero code: error;
//   - otherwise: stopped.
//
// A container that is only starting its health check counts as running. The
// count is of containers, not of services: the list API knows no services
// that never got a container.
func DeriveStatus(containers []dockerhost.ContainerSummary) (Status, ServiceCount) {
	count := ServiceCount{Total: len(containers)}
	if count.Total == 0 {
		return StatusDraft, count
	}
	healthy := 0
	failed := false
	for _, c := range containers {
		switch c.State {
		case "running":
			count.Running++
			if c.Health != "unhealthy" {
				healthy++
			}
		case "restarting", "dead":
			failed = true
		case "exited":
			if code, ok := exitCodeOf(c); ok && code != 0 {
				failed = true
			}
		}
	}
	switch {
	case count.Running == count.Total && healthy == count.Total:
		return StatusRunning, count
	case count.Running > 0:
		return StatusPartial, count
	case failed:
		return StatusError, count
	}
	return StatusStopped, count
}

// normalizeBindIP folds "all interfaces" spellings into "".
func normalizeBindIP(ip string) string {
	switch ip {
	case "0.0.0.0", "::", "[::]":
		return ""
	}
	return ip
}

// portLinks lists the published ports of the containers, sorted by public
// port. The Engine reports a port bound to all interfaces once for IPv4 and
// once for IPv6; those collapse into one link.
func portLinks(containers []dockerhost.ContainerSummary) []PortLink {
	links := []PortLink{}
	seen := map[PortLink]bool{}
	for _, c := range containers {
		for _, p := range c.Ports {
			if p.PublicPort <= 0 {
				continue
			}
			link := PortLink{
				Service:     c.ComposeService,
				IP:          normalizeBindIP(p.IP),
				PublicPort:  p.PublicPort,
				PrivatePort: p.PrivatePort,
				Protocol:    p.Type,
			}
			if seen[link] {
				continue
			}
			seen[link] = true
			links = append(links, link)
		}
	}
	slices.SortFunc(links, func(a, b PortLink) int {
		switch {
		case a.PublicPort != b.PublicPort:
			return a.PublicPort - b.PublicPort
		case a.Protocol != b.Protocol:
			return strings.Compare(a.Protocol, b.Protocol)
		case a.Service != b.Service:
			return strings.Compare(a.Service, b.Service)
		}
		return strings.Compare(a.IP, b.IP)
	})
	return links
}

// isHelperContainer reports one of the Docker Manager's own runner, lister or
// IO containers. They never belong to a stack or a project.
func isHelperContainer(c dockerhost.ContainerSummary) bool {
	return c.Labels[LabelRole] != ""
}

// shortIDs returns the 12-character ids of the containers, oldest first.
func shortIDs(containers []dockerhost.ContainerSummary) []string {
	sorted := slices.Clone(containers)
	slices.SortStableFunc(sorted, func(a, b dockerhost.ContainerSummary) int { return cmp.Compare(a.Created, b.Created) })
	ids := make([]string, 0, len(sorted))
	for _, c := range sorted {
		ids = append(ids, c.ID)
	}
	return ids
}

// oldestCreated is the creation time (unix seconds) of the oldest container, 0 for none.
func oldestCreated(containers []dockerhost.ContainerSummary) int64 {
	var oldest int64
	for _, c := range containers {
		if c.Created > 0 && (oldest == 0 || c.Created < oldest) {
			oldest = c.Created
		}
	}
	return oldest
}
