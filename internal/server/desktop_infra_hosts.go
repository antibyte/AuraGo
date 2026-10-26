package server

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"

	"aurago/internal/config"
	"aurago/internal/infra"
)

type desktopInfraSourceHostRequest struct {
	Kind     string `json:"kind"`
	Instance string `json:"instance"`
	NodeID   string `json:"node_id"`
	Version  int64  `json:"version"`
}

// desktopInfraSetSourceHost assigns or clears the device that runs a Docker
// engine. Inferred hosts are runtime projections reported by the snapshot.
func desktopInfraSetSourceHost(w http.ResponseWriter, r *http.Request, store *infra.Store) {
	var request desktopInfraSourceHostRequest
	if !desktopInfraDecode(w, r, &request) {
		return
	}
	if request.NodeID != "" && !desktopInfraValidPathID(request.NodeID) {
		desktopInfraError(w, http.StatusBadRequest, "invalid_node_id", "Node ID is invalid")
		return
	}
	host, err := store.SetSourceHost(r.Context(), infra.SourceHost{
		Kind: request.Kind, Instance: request.Instance, NodeID: request.NodeID, Version: request.Version,
	})
	if err != nil {
		desktopInfraStoreError(w, err)
		return
	}
	desktopInfraJSON(w, http.StatusOK, map[string]any{"source_host": host})
}

// desktopInfraLocalAddresses lists the AuraGo server's own interface addresses.
// A local Docker socket or pipe runs on the machine with these addresses.
// Tests replace it.
var desktopInfraLocalAddresses = func() []netip.Addr {
	addresses, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	result := make([]netip.Addr, 0, len(addresses))
	for _, address := range addresses {
		prefix, err := netip.ParsePrefix(address.String())
		if err != nil {
			continue
		}
		result = append(result, prefix.Addr().Unmap())
	}
	return result
}

// desktopInfraDockerEngineAddresses returns the addresses that identify the
// machine running the configured Docker engine. A local socket or pipe, an
// empty host, localhost or a loopback IP means this server; a TCP endpoint with
// an IP literal names that machine. Host names are never resolved.
func desktopInfraDockerEngineAddresses(host string, local []netip.Addr) []netip.Addr {
	host = strings.TrimSpace(host)
	if host == "" || strings.HasPrefix(host, "unix://") || strings.HasPrefix(host, "npipe://") {
		return desktopInfraUsableAddresses(local)
	}
	parsed, err := url.Parse(host)
	if err != nil || (parsed.Scheme != "tcp" && parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil
	}
	name := parsed.Hostname()
	if strings.EqualFold(name, "localhost") {
		return desktopInfraUsableAddresses(local)
	}
	addr, err := netip.ParseAddr(name)
	if err != nil {
		return nil
	}
	addr = addr.Unmap()
	if addr.IsLoopback() {
		return desktopInfraUsableAddresses(local)
	}
	return []netip.Addr{addr}
}

func desktopInfraUsableAddresses(addresses []netip.Addr) []netip.Addr {
	result := make([]netip.Addr, 0, len(addresses))
	for _, addr := range addresses {
		if !addr.IsValid() || addr.IsLoopback() || addr.IsLinkLocalUnicast() || addr.IsUnspecified() {
			continue
		}
		result = append(result, addr.Unmap())
	}
	return result
}

// desktopInfraNodeWithAddress returns the only device whose profile or
// interface address is one of addresses. Ambiguity infers nothing.
func desktopInfraNodeWithAddress(nodes []infra.Node, addresses []netip.Addr) string {
	if len(addresses) == 0 {
		return ""
	}
	wanted := make(map[netip.Addr]struct{}, len(addresses))
	for _, addr := range addresses {
		wanted[addr.Unmap()] = struct{}{}
	}
	match := ""
	for _, node := range nodes {
		if node.Kind != infra.NodeDevice || !desktopInfraNodeHasAddress(node, wanted) {
			continue
		}
		if match != "" {
			return ""
		}
		match = node.ID
	}
	return match
}

func desktopInfraNodeHasAddress(node infra.Node, wanted map[netip.Addr]struct{}) bool {
	values := make([]string, 0, len(node.Profile.Interfaces)+1)
	values = append(values, node.Profile.IPAddress)
	for _, iface := range node.Profile.Interfaces {
		values = append(values, iface.IPAddress)
	}
	for _, value := range values {
		addr, err := netip.ParseAddr(strings.TrimSpace(value))
		if err != nil {
			continue
		}
		if _, ok := wanted[addr.Unmap()]; ok {
			return true
		}
	}
	return false
}

// desktopInfraProjectHosts sets the runtime-only host of every guest (VM or
// container) among the snapshot's nodes, and the inferred host of each Docker
// engine entry. Proxmox guests use their provider runs_on edge; Docker guests
// use the operator's engine mapping, else an inferred device for the configured
// engine (host_inferred). Only nodes present in the snapshot can be hosts.
func desktopInfraProjectHosts(cfg *config.Config, snapshot *infra.Snapshot, local []netip.Addr) {
	if snapshot == nil {
		return
	}
	visible := make(map[string]bool, len(snapshot.Nodes))
	for _, node := range snapshot.Nodes {
		visible[node.ID] = true
	}
	engine, inferred := "", ""
	if cfg != nil && cfg.Docker.Enabled {
		engine = desktopInfraDockerInstance(cfg)
		inferred = desktopInfraNodeWithAddress(snapshot.Nodes, desktopInfraDockerEngineAddresses(cfg.Docker.Host, local))
	}
	mapped := make(map[string]string, len(snapshot.SourceHosts))
	for i := range snapshot.SourceHosts {
		host := &snapshot.SourceHosts[i]
		host.InferredNodeID = ""
		if host.Kind != "docker" {
			continue
		}
		if host.Instance == engine {
			host.InferredNodeID = inferred
		}
		if host.NodeID != "" && visible[host.NodeID] {
			mapped[host.Instance] = host.NodeID
		}
	}
	runsOn := make(map[string]string)
	for _, edge := range snapshot.Edges {
		if edge.Kind == infra.EdgeRunsOn && edge.Source != nil && edge.Source.Kind == "proxmox" && visible[edge.To] {
			runsOn[edge.From] = edge.To
		}
	}
	for i := range snapshot.Nodes {
		node := &snapshot.Nodes[i]
		node.HostID, node.HostInferred = "", false
		if node.Kind != infra.NodeVM && node.Kind != infra.NodeContainer {
			continue
		}
		if host, ok := runsOn[node.ID]; ok {
			node.HostID = host
			continue
		}
		for _, source := range node.Sources {
			if source.Kind != "docker" {
				continue
			}
			if host := mapped[source.Instance]; host != "" {
				node.HostID = host
			} else if source.Instance == engine && inferred != "" {
				node.HostID, node.HostInferred = inferred, true
			}
			break
		}
	}
}

// desktopInfraHostedSnapshot reads the stored graph with the snapshot's
// visibility rule and host projection, without app-link or action projection.
func desktopInfraHostedSnapshot(ctx context.Context, store *infra.Store, cfg *config.Config) (infra.Snapshot, error) {
	snapshot, err := store.Snapshot(ctx)
	if err != nil {
		return infra.Snapshot{}, err
	}
	snapshot.Nodes = desktopInfraVisibleNodes(&snapshot)
	desktopInfraProjectHosts(cfg, &snapshot, desktopInfraLocalAddresses())
	return snapshot, nil
}

// desktopInfraApplyHosts projects host_id/host_inferred onto nodes returned by
// a write. It is best effort: without a readable graph the fields stay empty and
// the next snapshot carries them.
func desktopInfraApplyHosts(ctx context.Context, store *infra.Store, cfg *config.Config, nodes []infra.Node) []infra.Node {
	if store == nil || len(nodes) == 0 {
		return nodes
	}
	snapshot, err := desktopInfraHostedSnapshot(ctx, store, cfg)
	if err != nil {
		return nodes
	}
	projected := make(map[string]infra.Node, len(snapshot.Nodes))
	for _, node := range snapshot.Nodes {
		projected[node.ID] = node
	}
	for i := range nodes {
		host := projected[nodes[i].ID]
		nodes[i].HostID, nodes[i].HostInferred = host.HostID, host.HostInferred
	}
	return nodes
}
