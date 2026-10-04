package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"aurago/internal/config"
	evomapclient "aurago/internal/evomap"
	"aurago/internal/security"
)

func dispatchEvomapCall(ctx context.Context, req evomapArgs, cfg *config.Config) string {
	if cfg == nil || !cfg.Evomap.Enabled {
		return `Tool Output: {"status":"error","message":"EvoMap is disabled. Enable evomap.enabled in config.yaml."}`
	}

	op := strings.ToLower(strings.TrimSpace(req.Operation))
	switch op {
	case "publish_bundle", "submit_report", "kg_ingest", "claim_bounty", "heartbeat":
		return evomapPolicyDenied(op, "This EvoMap operation is prepared but not implemented in the MVP.")
	}

	switch op {
	case "status", "":
		client, err := evomapClientFromConfig(cfg)
		if err != nil {
			return evomapErrorOutput(op, err)
		}
		status, err := client.Status(ctx)
		if err != nil {
			return evomapErrorOutput("status", err)
		}
		return evomapExternalRaw(ctx, map[string]interface{}{
			"status":    "success",
			"operation": "status",
			"evomap":    json.RawMessage(status.Raw),
		})

	case "register_node":
		return dispatchEvomapRegisterNode(ctx, cfg)

	case "fetch_capsules":
		client, err := evomapClientFromConfig(cfg)
		if err != nil {
			return evomapErrorOutput(op, err)
		}
		result, err := client.FetchCapsules(ctx, evomapclient.FetchRequest{
			Problem: req.Problem,
			Query:   req.Query,
			Signals: req.Signals,
			Limit:   req.Limit,
		})
		if err != nil {
			return evomapErrorOutput(op, err)
		}
		return evomapExternalRaw(ctx, map[string]interface{}{
			"status":    "success",
			"operation": op,
			"capsules":  json.RawMessage(result.Raw),
		})

	case "get_asset":
		client, err := evomapClientFromConfig(cfg)
		if err != nil {
			return evomapErrorOutput(op, err)
		}
		result, err := client.GetAsset(ctx, evomapclient.AssetRequest{AssetID: req.AssetID})
		if err != nil {
			return evomapErrorOutput(op, err)
		}
		return evomapExternalRaw(ctx, map[string]interface{}{
			"status":    "success",
			"operation": op,
			"asset":     json.RawMessage(result.Raw),
		})

	case "kg_query":
		if !cfg.Evomap.KGEnabled {
			return evomapPolicyDenied(op, "EvoMap KG access is disabled. Set evomap.kg_enabled=true to allow kg_query.")
		}
		if strings.TrimSpace(cfg.Evomap.APIKey) == "" {
			return evomapJSONOutput(map[string]interface{}{
				"status":    "error",
				"operation": op,
				"message":   "EvoMap API key is not configured in the vault.",
			})
		}
		client, err := evomapClientFromConfig(cfg)
		if err != nil {
			return evomapErrorOutput(op, err)
		}
		result, err := client.KGQuery(ctx, evomapclient.KGQueryRequest{
			Question: req.Question,
			Query:    req.Query,
			Limit:    req.Limit,
		})
		if err != nil {
			return evomapErrorOutput(op, err)
		}
		return evomapExternalRaw(ctx, map[string]interface{}{
			"status":    "success",
			"operation": op,
			"answer":    json.RawMessage(result.Raw),
		})

	default:
		return evomapJSONOutput(map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("unknown EvoMap operation %q", op),
		})
	}
}

func dispatchEvomapRegisterNode(ctx context.Context, cfg *config.Config) string {
	if cfg.Evomap.ReadOnly {
		return evomapPolicyDenied("register_node", "EvoMap is read-only; node registration is disabled.")
	}
	if cfg.RegisterEvomapNode == nil {
		return evomapPolicyDenied("register_node", "EvoMap registration requires the server configuration writer.")
	}
	nodeID, claimURL, secretConfigured, err := cfg.RegisterEvomapNode(ctx)
	if err != nil {
		return evomapErrorOutput("register_node", err)
	}
	return evomapJSONOutput(map[string]interface{}{
		"status": "success", "operation": "register_node", "node_id": nodeID, "claim_url": claimURL,
		"node_secret_configured": secretConfigured, "node_secret_vault_key": "evomap_node_secret",
		"node_secret_was_hidden": true, "external_raw_suppressed": true,
	})
}

func evomapClientFromConfig(cfg *config.Config) (*evomapclient.Client, error) {
	timeout := time.Duration(cfg.Evomap.RequestTimeoutSeconds) * time.Second
	return evomapclient.NewClient(evomapclient.Config{
		BaseURL:        cfg.Evomap.BaseURL,
		NodeID:         cfg.Evomap.NodeID,
		NodeSecret:     cfg.Evomap.NodeSecret,
		APIKey:         cfg.Evomap.APIKey,
		Timeout:        timeout,
		MaxResultBytes: int64(cfg.Evomap.MaxResultBytes),
	})
}

func evomapPolicyDenied(operation, message string) string {
	return evomapJSONOutput(map[string]interface{}{
		"status":    "policy_denied",
		"operation": operation,
		"message":   message,
	})
}

func evomapErrorOutput(operation string, err error) string {
	return evomapJSONOutput(map[string]interface{}{
		"status":    "error",
		"operation": operation,
		"message":   security.Scrub(err.Error()),
	})
}

func evomapJSONOutput(payload map[string]interface{}) string {
	raw, _ := json.Marshal(payload)
	return "Tool Output: " + string(raw)
}

func evomapExternalRaw(ctx context.Context, payload map[string]interface{}) string {
	raw, _ := json.Marshal(payload)
	return externalToolOutput(ctx, string(raw))
}
