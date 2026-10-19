package agent

import (
	"context"
	"fmt"
	"sync"

	"aurago/internal/config"
)

// ToolBridgeGrantIssuer binds a broker capability to the calling run. The
// returned release function must be called when that execution finishes.
type ToolBridgeGrantIssuer func(context.Context, *DispatchContext) (string, string, func(), error)

var bridgeGrantIssuer struct {
	sync.RWMutex
	issue ToolBridgeGrantIssuer
}

func ToolBridgeEndpoint(cfg *config.Config) string { return toolBridgeURL(cfg) }

func SetToolBridgeGrantIssuer(issue ToolBridgeGrantIssuer) {
	bridgeGrantIssuer.Lock()
	bridgeGrantIssuer.issue = issue
	bridgeGrantIssuer.Unlock()
}

func IssueToolBridgeGrant(ctx context.Context, dc *DispatchContext) (string, string, func(), error) {
	if ctx == nil || dc == nil || dc.SessionID == "" {
		return "", "", nil, fmt.Errorf("tool bridge requires an originating run")
	}
	if err := ctx.Err(); err != nil {
		return "", "", nil, err
	}
	bridgeGrantIssuer.RLock()
	issue := bridgeGrantIssuer.issue
	bridgeGrantIssuer.RUnlock()
	if issue == nil {
		return "", "", nil, fmt.Errorf("tool bridge supervisor is unavailable")
	}
	return issue(ctx, dc)
}
