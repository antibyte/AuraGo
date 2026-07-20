package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/networkshares"
)

const networkSharesRequestLimit = 128 << 10

func handleNetworkSharesStatus(s *Server) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			networkSharesJSONError(w, fmt.Errorf("method not allowed"), http.StatusMethodNotAllowed)
			return
		}
		if s == nil || s.NetworkShares == nil {
			networkSharesJSONError(w, &networkshares.CodedError{
				Code: networkshares.ErrorUnavailable, Message: "Network share manager is unavailable.",
			}, 0)
			return
		}
		cfg := s.ConfigSnapshot()
		response := map[string]interface{}{"status": s.NetworkShares.Status()}
		if cfg != nil {
			response["permissions"] = networkSharesPermissions(cfg)
		}
		writeJSON(w, response)
	})
}

func handleNetworkSharesReprobe(s *Server) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			networkSharesJSONError(w, fmt.Errorf("method not allowed"), http.StatusMethodNotAllowed)
			return
		}
		status, err := refreshNetworkSharesRuntime(r.Context(), s)
		if err != nil {
			networkSharesJSONError(w, err, 0)
			return
		}
		writeJSON(w, map[string]interface{}{"status": status})
	})
}

func handleNetworkSharesValidate(s *Server) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			networkSharesJSONError(w, fmt.Errorf("method not allowed"), http.StatusMethodNotAllowed)
			return
		}
		if s == nil || s.NetworkShares == nil {
			networkSharesJSONError(w, &networkshares.CodedError{
				Code: networkshares.ErrorUnavailable, Message: "Network share manager is unavailable.",
			}, 0)
			return
		}
		var request struct {
			Operation string                  `json:"operation"`
			Share     networkshares.ShareSpec `json:"share"`
		}
		if err := decodeNetworkSharesJSON(w, r, &request); err != nil {
			networkSharesJSONError(w, err, 0)
			return
		}
		validated, err := s.NetworkShares.Validate(r.Context(), request.Share, "validate")
		if err != nil {
			networkSharesJSONError(w, err, 0)
			return
		}
		writeJSON(w, map[string]interface{}{"valid": true, "share": validated})
	})
}

func handleNetworkSharesCollection(s *Server) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s == nil || s.NetworkShares == nil {
			networkSharesJSONError(w, &networkshares.CodedError{
				Code: networkshares.ErrorUnavailable, Message: "Network share manager is unavailable.",
			}, 0)
			return
		}
		switch r.Method {
		case http.MethodGet:
			shares, err := s.NetworkShares.List(r.Context())
			if err != nil {
				networkSharesJSONError(w, err, 0)
				return
			}
			if shares == nil {
				shares = []networkshares.Share{}
			}
			writeJSON(w, map[string]interface{}{"shares": shares})
		case http.MethodPost:
			var request networkshares.ShareSpec
			if err := decodeNetworkSharesJSON(w, r, &request); err != nil {
				networkSharesJSONError(w, err, 0)
				return
			}
			created, err := s.NetworkShares.Create(r.Context(), request)
			if err != nil {
				networkSharesJSONError(w, err, 0)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(created)
		default:
			networkSharesJSONError(w, fmt.Errorf("method not allowed"), http.StatusMethodNotAllowed)
		}
	})
}

func handleNetworkShareByID(s *Server) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s == nil || s.NetworkShares == nil {
			networkSharesJSONError(w, &networkshares.CodedError{
				Code: networkshares.ErrorUnavailable, Message: "Network share manager is unavailable.",
			}, 0)
			return
		}
		id := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/network-shares/"))
		if id == "" || strings.Contains(id, "/") {
			networkSharesJSONError(w, &networkshares.CodedError{
				Code: networkshares.ErrorInvalidArgument, Message: "A valid share ID is required.",
			}, 0)
			return
		}
		switch r.Method {
		case http.MethodGet:
			share, err := s.NetworkShares.Get(r.Context(), id)
			if err != nil {
				networkSharesJSONError(w, err, 0)
				return
			}
			writeJSON(w, share)
		case http.MethodPatch:
			var request networkshares.SharePatch
			if err := decodeNetworkSharesJSON(w, r, &request); err != nil {
				networkSharesJSONError(w, err, 0)
				return
			}
			share, err := s.NetworkShares.Update(r.Context(), id, request)
			if err != nil {
				networkSharesJSONError(w, err, 0)
				return
			}
			writeJSON(w, share)
		case http.MethodDelete:
			if err := s.NetworkShares.Delete(r.Context(), id); err != nil {
				networkSharesJSONError(w, err, 0)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			networkSharesJSONError(w, fmt.Errorf("method not allowed"), http.StatusMethodNotAllowed)
		}
	})
}

func refreshNetworkSharesRuntime(parent context.Context, s *Server) (networkshares.Status, error) {
	if s == nil || s.NetworkShares == nil {
		return networkshares.Status{}, &networkshares.CodedError{
			Code: networkshares.ErrorUnavailable, Message: "Network share manager is unavailable.",
		}
	}
	cfg := s.ConfigSnapshot()
	if cfg == nil {
		return networkshares.Status{}, fmt.Errorf("configuration is unavailable")
	}
	sudoPassword := ""
	if s.Vault != nil && cfg.Agent.SudoEnabled {
		sudoPassword, _ = s.Vault.ReadSecret("sudo_password")
	}
	s.NetworkShares.Configure(config.NetworkSharesOptions(cfg, sudoPassword))
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	status := s.NetworkShares.Reprobe(ctx)
	cancel()

	s.CfgMu.Lock()
	s.Cfg.Runtime.NetworkShares = status
	s.replaceConfigSnapshot(s.Cfg)
	s.CfgMu.Unlock()
	return status, nil
}

func networkSharesPermissions(cfg *config.Config) map[string]interface{} {
	return map[string]interface{}{
		"enabled":            cfg.NetworkShares.Enabled,
		"readonly":           cfg.NetworkShares.ReadOnly,
		"allow_create":       cfg.NetworkShares.AllowCreate,
		"allow_update":       cfg.NetworkShares.AllowUpdate,
		"allow_delete":       cfg.NetworkShares.AllowDelete,
		"allowed_roots":      cfg.NetworkShares.AllowedRoots,
		"smb_enabled":        cfg.NetworkShares.SMB.Enabled,
		"smb_allow_guest":    cfg.NetworkShares.SMB.AllowGuest,
		"allowed_principals": cfg.NetworkShares.SMB.AllowedPrincipals,
		"nfs_enabled":        cfg.NetworkShares.NFS.Enabled,
		"allowed_clients":    cfg.NetworkShares.NFS.AllowedClients,
	}
}

func decodeNetworkSharesJSON(w http.ResponseWriter, r *http.Request, destination interface{}) error {
	if r.Body == nil {
		return &networkshares.CodedError{
			Code: networkshares.ErrorInvalidArgument, Message: "A JSON request body is required.",
		}
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, networkSharesRequestLimit))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return &networkshares.CodedError{
			Code: networkshares.ErrorInvalidArgument, Message: "The network share request is invalid.", Err: err,
		}
	}
	var trailing interface{}
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return &networkshares.CodedError{
			Code: networkshares.ErrorInvalidArgument, Message: "The request must contain exactly one JSON value.",
		}
	}
	return nil
}

func networkSharesJSONError(w http.ResponseWriter, err error, explicitStatus int) {
	status := explicitStatus
	code := networkshares.ErrorCode(err)
	if status == 0 {
		switch code {
		case networkshares.ErrorDisabled, networkshares.ErrorUnavailable:
			status = http.StatusServiceUnavailable
		case networkshares.ErrorReadOnly, networkshares.ErrorPermissionDenied, networkshares.ErrorNotManaged:
			status = http.StatusForbidden
		case networkshares.ErrorConflict, networkshares.ErrorDrift:
			status = http.StatusConflict
		case networkshares.ErrorInvalidArgument, networkshares.ErrorOutsideRoot:
			status = http.StatusBadRequest
		case networkshares.ErrorNotFound:
			status = http.StatusNotFound
		case networkshares.ErrorApplyFailed:
			status = http.StatusBadGateway
		default:
			status = http.StatusInternalServerError
		}
	}
	if code == "" {
		code = "NETWORK_SHARES_INTERNAL"
	}
	message := "Network share request failed."
	var coded *networkshares.CodedError
	if errors.As(err, &coded) && strings.TrimSpace(coded.Message) != "" {
		message = coded.Message
	} else if explicitStatus == http.StatusMethodNotAllowed {
		message = "Method not allowed."
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"code": code, "message": message})
}
