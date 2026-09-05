//go:build !linux

package meshcore

import (
	"context"
	"fmt"
)

func openBLE(context.Context, string) (frameLink, error) {
	return nil, fmt.Errorf("meshcore_ble_linux_required")
}
