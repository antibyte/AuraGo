//go:build !linux && !windows

package networkshares

import (
	"context"
	"log/slog"
)

type unsupportedAdapter struct{}

func platformSupported() bool {
	return false
}

func newPlatformAdapter(_ commandRunner, _ *slog.Logger) platformAdapter {
	return unsupportedAdapter{}
}

func (unsupportedAdapter) Probe(context.Context, Options) (Status, error) {
	return unavailableStatus("Local network share management is supported only on Linux and Windows."), nil
}

func (unsupportedAdapter) Validate(context.Context, Options, ShareSpec) error {
	return codedError(ErrorUnavailable, "Local network share management is supported only on Linux and Windows.", nil)
}

func (unsupportedAdapter) List(context.Context, Options) ([]observedShare, error) {
	return nil, codedError(ErrorUnavailable, "Local network share management is supported only on Linux and Windows.", nil)
}

func (unsupportedAdapter) Create(context.Context, Options, ShareSpec) error {
	return codedError(ErrorUnavailable, "Local network share management is supported only on Linux and Windows.", nil)
}

func (unsupportedAdapter) Update(context.Context, Options, ShareSpec, ShareSpec) error {
	return codedError(ErrorUnavailable, "Local network share management is supported only on Linux and Windows.", nil)
}

func (unsupportedAdapter) Delete(context.Context, Options, ShareSpec) error {
	return codedError(ErrorUnavailable, "Local network share management is supported only on Linux and Windows.", nil)
}
