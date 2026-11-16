package tools

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	"aurago/internal/security"
	"aurago/internal/storeapps"
)

type StoreAppRequest struct {
	Operation   string                       `json:"operation"`
	NodeID      string                       `json:"node_id"`
	ActionID    string                       `json:"action_id"`
	ExecutionID string                       `json:"execution_id"`
	Arguments   []storeapps.OliveTinArgument `json:"arguments"`
	Limit       int                          `json:"limit"`
	Offset      int                          `json:"offset"`
	SystemID    string                       `json:"system_id"`
	Path        string                       `json:"path"`
	Query       string                       `json:"query"`
	Files       []string                     `json:"files"`
	Languages   []string                     `json:"languages"`
	Pages       string                       `json:"pages"`
}

var storeApplications struct {
	sync.RWMutex
	execute func(context.Context, string, StoreAppRequest) (any, error)
	enabled func(string) bool
}

func SetStoreApplications(execute func(context.Context, string, StoreAppRequest) (any, error), enabled func(string) bool) {
	storeApplications.Lock()
	storeApplications.execute, storeApplications.enabled = execute, enabled
	storeApplications.Unlock()
}

func StoreApplicationEnabled(id string) bool {
	storeApplications.RLock()
	enabled := storeApplications.enabled
	storeApplications.RUnlock()
	return enabled != nil && enabled(id)
}

func ExecuteStoreApplication(ctx context.Context, id string, req StoreAppRequest) string {
	storeApplications.RLock()
	execute := storeApplications.execute
	storeApplications.RUnlock()
	if execute == nil {
		return ErrorJSON("Store integration is not connected")
	}
	data, err := execute(ctx, id, req)
	result := map[string]any{"status": "ok", "data": data}
	if err != nil {
		result = map[string]any{"status": "error", "message": security.Scrub(err.Error()), "uncertain": errors.Is(err, storeapps.ErrUncertain)}
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return ErrorJSON("Store result could not be encoded")
	}
	return security.Scrub(string(raw))
}
