//go:build linux

package bluetooth

import (
	"fmt"
	"log/slog"
)

// Replaced by the D-Bus implementation in the next step of the rollout.
func platformBusDialer(_ *slog.Logger) busDialer {
	return func() (bluezBus, error) {
		return nil, fmt.Errorf("the live BlueZ session is not available yet")
	}
}
