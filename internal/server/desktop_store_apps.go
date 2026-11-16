package server

import (
	"context"
	"errors"
	"time"

	"aurago/internal/desktopstore"
	"aurago/internal/storeapps"
	"aurago/internal/tools"
)

func storeAppConnection(settings desktopstore.IntegrationSettings) storeapps.Connection {
	return storeapps.Connection{URL: settings.URL, Username: settings.Username, Credential: settings.Credential, AllowedIDs: settings.AllowedIDs, Source: settings.Source, CertificateFingerprint: settings.CertificateFingerprint}
}

func (s *Server) storeApplicationEnabled(id string) bool {
	cfg := s.ConfigSnapshot()
	if cfg == nil || !cfg.VirtualDesktop.Enabled || !cfg.VirtualDesktop.AllowAgentControl || !cfg.Tools.VirtualDesktop.Enabled || !cfg.Agent.AllowNetworkRequests {
		return false
	}
	s.storeIntegrations.Lock()
	b, exists := s.storeIntegrations.bindings[id]
	s.storeIntegrations.Unlock()
	return exists && b.settings.Read && b.ctx.Err() == nil
}

func (s *Server) executeStoreApplication(ctx context.Context, id string, req tools.StoreAppRequest) (any, error) {
	s.storeIntegrations.Lock()
	b, exists := s.storeIntegrations.bindings[id]
	s.storeIntegrations.Unlock()
	if !exists {
		return nil, errors.New("Verify the Store connection first")
	}
	mutation := (id == "node-red" && req.Operation == "trigger") || (id == "olivetin" && req.Operation == "execute")
	ctx, release, err := s.acquireStoreAccess(ctx, id, b.settings.URL, mutation)
	if err != nil {
		return nil, err
	}
	defer release()
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	client := storeAppConnection(b.settings)
	if req.Limit == 0 {
		req.Limit = 25
	}
	if req.Limit < 1 || req.Limit > 50 || req.Offset < 0 || req.Offset > 100000 {
		return nil, errors.New("Invalid result page")
	}
	var result any
	switch id + ":" + req.Operation {
	case "node-red:list_flows":
		flows, e := client.NodeREDFlows(ctx)
		err = e
		start := min(req.Offset, len(flows))
		end := min(start+req.Limit, len(flows))
		result = map[string]any{"flows": flows[start:end], "has_more": end < len(flows), "next_offset": end}
	case "node-red:trigger":
		result, err = client.NodeREDTrigger(ctx, req.NodeID)
	case "olivetin:list_actions":
		result, err = client.OliveTinActions(ctx, req.Offset, req.Limit)
	case "olivetin:execute":
		result, err = client.OliveTinExecute(ctx, req.ActionID, req.Arguments)
	case "olivetin:get_execution":
		result, err = client.OliveTinStatus(ctx, req.ActionID, req.ExecutionID)
	default:
		return nil, errors.New("Unsupported Store application operation")
	}
	if err == nil && ctx.Err() != nil {
		if mutation {
			return nil, storeapps.ErrUncertain
		}
		return nil, ctx.Err()
	}
	return result, err
}
