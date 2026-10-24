package server

import (
	"net/http"

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
