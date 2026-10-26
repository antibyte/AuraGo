package server

import (
	"regexp"
	"strings"

	"aurago/internal/infra"
)

var desktopInfraOpaqueDeviceName = regexp.MustCompile(`(?i)^(?:(?:urn:uuid:|uuid:|sha256:)?[0-9a-f]{12,}|(?:urn:uuid:|uuid:)?[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}|[0-9a-f]{2}(?::[0-9a-f]{2}){5}|[0-9a-f]{2}(?:-[0-9a-f]{2}){5})$`)

func desktopInfraCandidateNamed(source infra.Source) bool {
	if source.Kind != "registry" && source.Kind != "fritzbox" {
		return true
	}
	name := strings.TrimSpace(source.Observed.Name)
	if name == "" {
		name = strings.TrimSpace(source.Name)
	}
	return desktopInfraRecognizableDeviceName(name)
}

func desktopInfraRecognizableDeviceName(name string) bool {
	name = strings.TrimSpace(name)
	switch strings.ToLower(name) {
	case "", "unnamed device", "fritz!box host":
		return false
	}
	return !desktopInfraOpaqueDeviceName.MatchString(name)
}

func desktopInfraVisibleNodes(snapshot *infra.Snapshot) []infra.Node {
	referenced := make(map[string]bool)
	for _, edge := range snapshot.Edges {
		referenced[edge.From], referenced[edge.To] = true, true
	}
	for _, node := range snapshot.Nodes {
		referenced[node.ParentID] = true
	}
	// An explicit Docker engine mapping is a deliberate use, like an edge.
	for _, host := range snapshot.SourceHosts {
		referenced[host.NodeID] = true
	}
	nodes := make([]infra.Node, 0, len(snapshot.Nodes))
	for _, node := range snapshot.Nodes {
		// Preserve edited, placed and connected nodes; hide only untouched discovery imports.
		hidden := node.Kind == infra.NodeDevice && node.Version == 1 && node.ParentID == "" &&
			len(node.Placements) == 0 && !referenced[node.ID] && len(node.Sources) > 0 &&
			!desktopInfraRecognizableDeviceName(node.Name)
		if hidden {
			for _, source := range node.Sources {
				if desktopInfraCandidateNamed(source) {
					hidden = false
					if source.Kind == "registry" || source.Kind == "fritzbox" {
						node.Name = strings.TrimSpace(source.Observed.Name)
						if node.Name == "" {
							node.Name = strings.TrimSpace(source.Name)
						}
					}
					break
				}
			}
		}
		if !hidden {
			nodes = append(nodes, node)
		}
	}
	return nodes
}
