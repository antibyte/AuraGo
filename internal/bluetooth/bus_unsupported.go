//go:build !linux

package bluetooth

import (
	"fmt"
	"log/slog"
)

func platformBusDialer(_ *slog.Logger) busDialer {
	return func() (bluezBus, error) {
		return nil, fmt.Errorf("Bluetooth is currently supported only on Linux with BlueZ")
	}
}
