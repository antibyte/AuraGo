package onnxrt

import (
	"context"
	"fmt"
)

// VerifyRuntime checks a prepared cache without downloading or repairing anything.
// Optional security components use it on startup so activation cannot trigger a download.
func (c *AssetCache) VerifyRuntime(ctx context.Context, spec AssetSpec) (string, error) {
	if err := validateRuntimeArchiveKind(spec); err != nil {
		return "", err
	}
	target := RuntimeDir(c.root, spec)
	err := c.withProcessLock(ctx, func() error {
		if !fileMatchesContext(ctx, ArchivePath(c.root, spec), spec.Size, spec.SHA256) || !runtimeManifestMatchesContext(ctx, target, spec) {
			return fmt.Errorf("verified runtime is not installed")
		}
		return ctx.Err()
	})
	if err != nil {
		return "", err
	}
	return target, nil
}
