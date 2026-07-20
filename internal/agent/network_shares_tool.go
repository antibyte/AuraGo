package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"aurago/internal/config"
	"aurago/internal/networkshares"
)

type networkSharesToolArgs struct {
	Operation string
	ID        string
	Protocol  string
	Name      string
	Path      string
	Comment   string
	ReadOnly  bool
	Guest     bool
	ACL       []networkshares.ACLEntry
	Clients   []string
	Managed   *bool
}

func decodeNetworkSharesToolArgs(tc ToolCall) networkSharesToolArgs {
	args := networkSharesToolArgs{
		Operation: strings.ToLower(firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation"))),
		ID:        toolArgString(tc.Params, "id", "share_id"),
		Protocol:  strings.ToLower(toolArgString(tc.Params, "protocol")),
		Name:      toolArgString(tc.Params, "name"),
		Path:      toolArgString(tc.Params, "path"),
		Comment:   toolArgString(tc.Params, "comment"),
		Clients:   toolArgStringSlice(tc.Params, "clients"),
		Managed:   toolArgBoolPtr(tc.Params, "managed"),
	}
	args.ReadOnly, _ = toolArgBool(tc.Params, "read_only")
	args.Guest, _ = toolArgBool(tc.Params, "guest")
	for _, item := range toolArgItems(tc.Params, "acl") {
		args.ACL = append(args.ACL, networkshares.ACLEntry{
			Principal: toolArgString(item, "principal"),
			Level:     toolArgString(item, "level"),
		})
	}
	return args
}

func dispatchNetworkShares(ctx context.Context, tc ToolCall, dc *DispatchContext) string {
	if dc == nil || dc.Cfg == nil {
		return networkSharesToolResult(nil, fmt.Errorf("network shares configuration is unavailable"))
	}
	manager := networkshares.DefaultManager()
	if manager == nil {
		return networkSharesToolResult(nil, &networkshares.CodedError{
			Code: networkshares.ErrorUnavailable, Message: "Network share manager is unavailable.",
		})
	}
	sudoPassword := ""
	if dc.Vault != nil && dc.Cfg.Agent.SudoEnabled {
		sudoPassword, _ = dc.Vault.ReadSecret("sudo_password")
	}
	manager.Configure(config.NetworkSharesOptions(dc.Cfg, sudoPassword))
	request := decodeNetworkSharesToolArgs(tc)

	switch request.Operation {
	case "status":
		return networkSharesToolResult(map[string]interface{}{"runtime": manager.Status()}, nil)
	case "list":
		shares, err := manager.List(ctx)
		if err != nil {
			return networkSharesToolResult(nil, err)
		}
		filtered := make([]networkshares.Share, 0, len(shares))
		for _, share := range shares {
			if request.Protocol != "" && !strings.EqualFold(request.Protocol, share.Protocol) {
				continue
			}
			if request.Managed != nil && share.Managed != *request.Managed {
				continue
			}
			filtered = append(filtered, share)
		}
		return networkSharesToolResult(map[string]interface{}{"shares": filtered}, nil)
	case "get":
		share, err := manager.Get(ctx, request.ID)
		return networkSharesToolResult(map[string]interface{}{"share": share}, err)
	case "create":
		share, err := manager.Create(ctx, networkshares.ShareSpec{
			Protocol: request.Protocol,
			Name:     request.Name,
			Path:     request.Path,
			Comment:  request.Comment,
			ReadOnly: request.ReadOnly,
			Access: networkshares.ShareAccess{
				Guest: request.Guest, ACL: request.ACL, Clients: request.Clients,
			},
		})
		return networkSharesToolResult(map[string]interface{}{"share": share}, err)
	case "update":
		patch := networkshares.SharePatch{}
		if _, exists := tc.Params["comment"]; exists {
			patch.Comment = &request.Comment
		}
		patch.ReadOnly = toolArgBoolPtr(tc.Params, "read_only")
		if networkSharesAccessWasProvided(tc.Params) {
			patch.Access = &networkshares.ShareAccess{
				Guest: request.Guest, ACL: request.ACL, Clients: request.Clients,
			}
		}
		share, err := manager.Update(ctx, request.ID, patch)
		return networkSharesToolResult(map[string]interface{}{"share": share}, err)
	case "delete":
		err := manager.Delete(ctx, request.ID)
		return networkSharesToolResult(map[string]interface{}{"id": request.ID}, err)
	default:
		return networkSharesToolResult(nil, &networkshares.CodedError{
			Code: networkshares.ErrorInvalidArgument, Message: "Unknown network_shares operation.",
		})
	}
}

func networkSharesAccessWasProvided(params map[string]interface{}) bool {
	for _, key := range []string{"guest", "acl", "clients"} {
		if _, exists := params[key]; exists {
			return true
		}
	}
	return false
}

func networkSharesToolResult(payload map[string]interface{}, err error) string {
	if payload == nil {
		payload = make(map[string]interface{})
	}
	if err != nil {
		payload["status"] = "error"
		code := networkshares.ErrorCode(err)
		if code == "" {
			code = networkshares.ErrorApplyFailed
			payload["message"] = "The network share operation failed."
		} else {
			payload["message"] = err.Error()
		}
		payload["code"] = code
	} else {
		payload["status"] = "ok"
	}
	encoded, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		return `Tool Output: {"status":"error","message":"Could not encode network share result."}`
	}
	return "Tool Output: " + string(encoded)
}
