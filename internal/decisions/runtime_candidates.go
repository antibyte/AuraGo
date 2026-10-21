package decisions

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"

	"aurago/internal/onnxrt"
)

type nativeRuntimeBundle struct{ directory, binary, digest string }

func verifyNativeRuntimeCandidate(ctx context.Context, directory, backend string) (nativeRuntimeBundle, error) {
	if err := ctx.Err(); err != nil {
		return nativeRuntimeBundle{}, NewError(CodeCancelled, cancellationCause(ctx))
	}
	if _, err := os.Lstat(directory); os.IsNotExist(err) {
		return nativeRuntimeBundle{}, NewError(CodeRuntimeArtifactMissing, err)
	}
	err := validateDownloadedRuntime(ctx, directory, backend)
	var binary, digest string
	if err == nil {
		binary, digest, err = validateRuntimeManifest(ctx, directory, backend, runtime.GOOS, runtime.GOARCH)
	}
	if ctx.Err() != nil {
		return nativeRuntimeBundle{}, NewError(CodeCancelled, cancellationCause(ctx))
	}
	if err != nil {
		return nativeRuntimeBundle{}, NewError(CodeRuntimeAttestationFailed, err)
	}
	return nativeRuntimeBundle{directory, binary, digest}, nil
}

func nativeRuntimeCandidates(ctx context.Context, sourceRoot, managedRoot, backend string) ([]nativeRuntimeBundle, error) {
	paths := []string{filepath.Join(sourceRoot, "bin", "decisions-runtime", runtimeTarget(runtime.GOOS, runtime.GOARCH), backend)}
	if entry, ok := releaseRuntimeCatalog.native(runtime.GOOS, runtime.GOARCH, backend); ok {
		paths = append(paths, onnxrt.RuntimeDir(filepath.Join(managedRoot, "packages"), entry.asset()))
	}
	paths = append(paths, filepath.Join(managedRoot, runtimeTarget(runtime.GOOS, runtime.GOARCH), backend))
	var bundles []nativeRuntimeBundle
	var failures []error
	for _, path := range paths {
		bundle, err := verifyNativeRuntimeCandidate(ctx, path, backend)
		if ErrorCode(err) == CodeCancelled {
			return nil, err
		}
		if err == nil {
			bundles = append(bundles, bundle)
		} else if ErrorCode(err) != CodeRuntimeArtifactMissing {
			failures = append(failures, err)
		}
	}
	if len(bundles) > 0 {
		return bundles, nil
	}
	if len(failures) > 0 {
		return nil, NewError(CodeRuntimeAttestationFailed, errors.Join(failures...))
	}
	return nil, NewError(CodeRuntimeArtifactMissing, nil)
}
