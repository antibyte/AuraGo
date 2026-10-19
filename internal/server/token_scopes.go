package server

import (
	"errors"
	"fmt"
	"strings"

	"aurago/internal/cydcreator"
)

// tokenScopeCYD is the on-glass display scope. CYD tokens are short so the
// device can be paired by typing them, so the scope must stand alone.
const tokenScopeCYD = "cyd"

// knownTokenScopes are the exact scopes passed to TokenManager.Validate, which
// compares case-sensitively without trimming. Validation applies to new tokens
// only; stored tokens keep whatever scopes they were created with.
var knownTokenScopes = map[string]bool{
	"admin":               true, // requireAdmin, validRouteBearer, daemon and go2rtc admin routes
	"webhook":             true, // webhooks.Handler
	tokenScopeCYD:         true, // authenticateCYD
	go2RTCViewScope:       true,
	go2RTCTalkScope:       true,
	desktopScopeRead:      true,
	desktopScopeWrite:     true,
	desktopScopeAdmin:     true,
	desktopRemoteScopeAll: true,
}

// knownTokenScopePrefixes name one inventory device or tag after the prefix;
// desktopRemoteTokenAllowsDevice compares them against trimmed IDs and tags.
var knownTokenScopePrefixes = []string{desktopRemoteScopeDevicePrefix, desktopRemoteScopeTagPrefix}

func isKnownTokenScope(scope string) bool {
	if knownTokenScopes[scope] {
		return true
	}
	for _, prefix := range knownTokenScopePrefixes {
		if suffix, ok := strings.CutPrefix(scope, prefix); ok && suffix != "" && suffix == strings.TrimSpace(suffix) {
			return true
		}
	}
	return false
}

// validateTokenScopes rejects scopes the server never checks, the CYD Creator
// device scope (minted per device by CYD Creator, never through this API) and
// the cyd scope mixed with anything else.
func validateTokenScopes(scopes []string) error {
	for _, s := range scopes {
		if s == cydcreator.TokenScopeDevice {
			return errors.New("the cyd_creator_device scope is minted by CYD Creator for one flashed device and cannot be created here")
		}
		if !isKnownTokenScope(s) {
			return fmt.Errorf("unknown token scope %q", s)
		}
	}
	if len(scopes) > 1 {
		for _, s := range scopes {
			if s == tokenScopeCYD {
				return errors.New("the cyd scope must be the token's only scope")
			}
		}
	}
	return nil
}
