//go:build !linux

package bluetooth

import (
	"fmt"
	"log/slog"
)

func newPlatformAgentHost(*interactionBroker, deviceResolver, *slog.Logger) (agentHost, error) {
	return nil, fmt.Errorf("interactive Bluetooth pairing is supported only on Linux with BlueZ")
}
