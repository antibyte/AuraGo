package server

import (
	"context"
	"net/http"
	"strings"

	"aurago/internal/config"
	"aurago/internal/infra"
)

// desktopInfraBatchLimit bounds one batch import.
const desktopInfraBatchLimit = 500

type desktopInfraBatchItem struct {
	CandidateID string               `json:"candidate_id"`
	NodeID      string               `json:"node_id,omitempty"`
	ParentID    *string              `json:"parent_id,omitempty"`
	Placement   *infra.ViewPlacement `json:"placement,omitempty"`
}

type desktopInfraBatchError struct {
	CandidateID string `json:"candidate_id"`
	Error       string `json:"error"`
	Message     string `json:"message"`
}

// desktopInfraImportBatch imports or links 1 to 500 candidates. Every provider
// behind the batch is refreshed once (forced) before any write; each item then
// runs in its own store transaction, so one failure never undoes another item.
func desktopInfraImportBatch(w http.ResponseWriter, r *http.Request, s *Server, cfg *config.Config, store *infra.Store, runtime *desktopInfraRuntime) {
	var request struct {
		Items []desktopInfraBatchItem `json:"items"`
	}
	if !desktopInfraDecode(w, r, &request) {
		return
	}
	if len(request.Items) == 0 || len(request.Items) > desktopInfraBatchLimit {
		desktopInfraError(w, http.StatusBadRequest, "invalid_request", "A batch needs 1 to 500 items")
		return
	}
	seen := make(map[string]struct{}, len(request.Items))
	for _, item := range request.Items {
		if strings.TrimSpace(item.CandidateID) == "" {
			desktopInfraError(w, http.StatusBadRequest, "invalid_candidate_id", "Every item needs a candidate ID")
			return
		}
		if _, duplicate := seen[item.CandidateID]; duplicate {
			desktopInfraError(w, http.StatusBadRequest, "invalid_request", "A candidate appears more than once")
			return
		}
		seen[item.CandidateID] = struct{}{}
	}
	failures := make([]desktopInfraBatchError, 0)
	pending := make([]desktopInfraBatchItem, 0, len(request.Items))
	providers := make(map[string]infra.SourceIdentity)
	for _, item := range request.Items {
		candidate, err := store.GetCandidate(r.Context(), item.CandidateID)
		if err != nil {
			failures = append(failures, desktopInfraBatchStoreError(item, err))
			continue
		}
		if !desktopInfraSourceConfigured(cfg, candidate.Source) {
			failures = append(failures, desktopInfraBatchError{CandidateID: item.CandidateID, Error: "source_unavailable", Message: "This source is no longer configured"})
			continue
		}
		providers[candidate.Source.Kind+"\x00"+candidate.Source.Instance] = candidate.Source.SourceIdentity
		pending = append(pending, item)
	}
	for _, identity := range providers {
		desktopInfraRefreshSource(r.Context(), s, cfg, store, runtime, identity)
	}
	nodes := make([]infra.Node, 0, len(pending))
	proxmoxInstances := make(map[string]struct{})
	for _, item := range pending {
		node, identity, failure := desktopInfraImportOne(r.Context(), store, item)
		if failure != nil {
			failures = append(failures, *failure)
			continue
		}
		if identity.Kind == "proxmox" {
			proxmoxInstances[identity.Instance] = struct{}{}
		}
		nodes = append(nodes, node)
	}
	for instance := range proxmoxInstances {
		// The graph writes already succeeded; placement edges catch up on the next
		// refresh if this best-effort reconcile fails.
		_ = desktopInfraReconcileProxmoxEdges(r.Context(), store, instance)
	}
	nodes = desktopInfraApplyHosts(r.Context(), store, cfg, desktopInfraProjectNodes(r.Context(), s, cfg, nodes))
	desktopInfraJSON(w, http.StatusOK, map[string]any{"nodes": nodes, "errors": failures})
}

// desktopInfraImportOne re-reads a candidate after the batch refresh and imports
// it (or links it to item.NodeID) with its optional parent and placement.
func desktopInfraImportOne(ctx context.Context, store *infra.Store, item desktopInfraBatchItem) (infra.Node, infra.SourceIdentity, *desktopInfraBatchError) {
	candidate, err := store.GetCandidate(ctx, item.CandidateID)
	if err != nil {
		failure := desktopInfraBatchStoreError(item, err)
		return infra.Node{}, infra.SourceIdentity{}, &failure
	}
	identity := candidate.Source.SourceIdentity
	if candidate.Source.State != infra.SourceCurrent {
		return infra.Node{}, identity, &desktopInfraBatchError{CandidateID: item.CandidateID, Error: "source_stale", Message: "The source could not be freshly verified"}
	}
	var node infra.Node
	if item.NodeID != "" {
		if !desktopInfraValidPathID(item.NodeID) {
			return infra.Node{}, identity, &desktopInfraBatchError{CandidateID: item.CandidateID, Error: "invalid_node_id", Message: "Node ID is invalid"}
		}
		node, err = store.LinkSourceWithLayout(ctx, item.NodeID, identity, infra.ImportLayout{ParentID: item.ParentID, Placement: item.Placement})
	} else {
		node = candidate.Node
		node.ID, node.Version, node.RegistryID, node.Sources = "", 0, "", []infra.Source{}
		if item.ParentID != nil {
			node.ParentID = strings.TrimSpace(*item.ParentID)
		}
		if item.Placement != nil {
			node.Placements = infra.Placements{item.Placement.View: item.Placement.AsPlacement()}
		}
		node, err = store.CreateImportedNode(ctx, node, identity)
	}
	if err != nil {
		failure := desktopInfraBatchStoreError(item, err)
		return infra.Node{}, identity, &failure
	}
	return node, identity, nil
}

func desktopInfraBatchStoreError(item desktopInfraBatchItem, err error) desktopInfraBatchError {
	_, code, message := desktopInfraStoreErrorStatus(err)
	return desktopInfraBatchError{CandidateID: item.CandidateID, Error: code, Message: message}
}
