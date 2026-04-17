package server

import "strings"

// extractSkillPathID extracts the resource ID from a URL path after a given prefix.
func extractSkillPathID(path, prefix string) string {
	id := strings.TrimPrefix(path, prefix)
	// Remove trailing slash
	id = strings.TrimSuffix(id, "/")
	// Stop at the next slash (sub-path like /verify)
	if idx := strings.Index(id, "/"); idx >= 0 {
		id = id[:idx]
	}
	return id
}
