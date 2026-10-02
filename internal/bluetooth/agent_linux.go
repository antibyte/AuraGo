//go:build linux

package bluetooth

import (
	"fmt"
	"log/slog"
)

// Replaced by the interactive BlueZ agent in the next step of the rollout.
func newPlatformAgentHost(*interactionBroker, deviceResolver, *slog.Logger) (agentHost, error) {
	return nil, fmt.Errorf("interactive Bluetooth pairing is not available yet")
}
