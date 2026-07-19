//go:build !linux

package bluetooth

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

type unsupportedAdapter struct{}

func platformSupported() bool {
	return false
}

func newPlatformAdapter(_ *slog.Logger) platformAdapter {
	return unsupportedAdapter{}
}

func (unsupportedAdapter) Probe(context.Context) (AdapterStatus, error) {
	return AdapterStatus{}, fmt.Errorf("Bluetooth is currently supported only on Linux with BlueZ")
}

func (unsupportedAdapter) List(context.Context) ([]Device, error) {
	return nil, fmt.Errorf("Bluetooth is not supported on this platform")
}

func (unsupportedAdapter) Discover(context.Context, time.Duration) ([]Device, error) {
	return nil, fmt.Errorf("Bluetooth is not supported on this platform")
}

func (unsupportedAdapter) Pair(context.Context, string, string) error {
	return fmt.Errorf("Bluetooth is not supported on this platform")
}

func (unsupportedAdapter) Connect(context.Context, string) error {
	return fmt.Errorf("Bluetooth is not supported on this platform")
}

func (unsupportedAdapter) Disconnect(context.Context, string) error {
	return fmt.Errorf("Bluetooth is not supported on this platform")
}
