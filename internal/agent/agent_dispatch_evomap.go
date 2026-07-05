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

type evomapSecretWriter interface {
	WriteSecret(key, value string) error
}

func dispatchEvomapCall(ctx context.Context, req evomapArgs, cfg *config.Config, secretWriters ...evomapSecretWriter) string {
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
		return evomapExternalRaw(map[string]interface{}{
			"status":    "success",
			"operation": "status",
			"evomap":    json.RawMessage(status.Raw),
		})

	case "register_node":
		client, err := evomapClientFromConfig(cfg)
		if err != nil {
			return evomapErrorOutput(op, err)
		}
		return dispatchEvomapRegisterNode(ctx, client, req, cfg, secretWriters...)

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
		return evomapExternalRaw(map[string]interface{}{
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
		return evomapExternalRaw(map[string]interface{}{
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
		return evomapExternalRaw(map[string]interface{}{
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

func dispatchEvomapRegisterNode(ctx context.Context, client *evomapclient.Client, req evomapArgs, cfg *config.Config, secretWriters ...evomapSecretWriter) string {
	var writer evomapSecretWriter
	if len(secretWriters) > 0 {
		writer = secretWriters[0]
	}
	if writer == nil {
		return evomapPolicyDenied("register_node", "EvoMap node registration requires a vault writer so node_secret can be stored without exposing it.")
	}
	result, err := client.RegisterNode(ctx, evomapclient.RegisterRequest{
		Capabilities: []string{"status", "fetch_capsules", "get_asset", "kg_query"},
		Metadata: map[string]interface{}{
			"client": "aurago",
		},
	})
	if err != nil {
		return evomapErrorOutput("register_node", err)
	}
	if strings.TrimSpace(result.NodeSecret) != "" {
		security.RegisterSensitive(result.NodeSecret)
		if err := writer.WriteSecret("evomap_node_secret", result.NodeSecret); err != nil {
			return evomapErrorOutput("register_node", fmt.Errorf("store EvoMap node secret: %w", err))
		}
		cfg.Evomap.NodeSecret = result.NodeSecret
	}
	if strings.TrimSpace(result.NodeID) != "" {
		cfg.Evomap.NodeID = strings.TrimSpace(result.NodeID)
		if strings.TrimSpace(cfg.ConfigPath) != "" {
			if err := cfg.Save(cfg.ConfigPath); err != nil {
				return evomapErrorOutput("register_node", fmt.Errorf("persist EvoMap node_id: %w", err))
			}
		}
	}
	return evomapJSONOutput(map[string]interface{}{
		"status":                  "success",
		"operation":               "register_node",
		"node_id":                 cfg.Evomap.NodeID,
		"claim_url":               result.ClaimURL,
		"node_secret_configured":  strings.TrimSpace(cfg.Evomap.NodeSecret) != "",
		"node_secret_vault_key":   "evomap_node_secret",
		"node_secret_was_hidden":  true,
		"external_raw_suppressed": true,
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

func evomapExternalRaw(payload map[string]interface{}) string {
	raw, _ := json.Marshal(payload)
	return "Tool Output: " + security.IsolateExternalData(security.Scrub(string(raw)))
}
