package chatrt

import (
	"context"

	"aurago/internal/llamacpp"
)

// Launcher is the seam between the manager and the operating system. NativeLauncher is the real one.
type Launcher interface {
	Platform() (goos, goarch string)
	Hardware(ctx context.Context) llamacpp.HardwareInfo
	// Assets is the install set of a backend: the CPU archive first, then the backend's archive and its
	// supplements, without duplicate IDs. ok is false when the platform has no CPU archive.
	Assets(backend string) (assets []llamacpp.AssetSpec, ok bool)
	// Verify reports, without network access, whether every asset is unpacked intact below cacheDir.
	Verify(ctx context.Context, cacheDir string, assets []llamacpp.AssetSpec) bool
	// Install downloads and unpacks the assets in order; progress gets the asset ID and its bytes done.
	Install(ctx context.Context, cacheDir string, assets []llamacpp.AssetSpec, progress func(assetID string, done int64)) error
	// Launch starts one server and returns when /health answers 200, or fails. It never downloads.
	Launch(ctx context.Context, spec LaunchSpec) (Process, error)
}

// LaunchSpec describes one server start.
type LaunchSpec struct {
	Key         string // ModelRef.Key (logs, fakes)
	CacheDir    string
	Assets      []llamacpp.AssetSpec // runtime roots of this start: [cpu] or [gpu, supplements...]
	Backend     string               // cpu | vulkan | cuda | metal
	ModelPath   string
	ContextSize int
	Threads     int
}

// Process is one running server.
type Process interface {
	Chat(ctx context.Context, req ChatRequest, onDelta func(text string)) (ChatStats, error) // Backend left empty
	GPUVerified() bool                                                                       // GPU evidence in the server log; false for cpu
	Done() <-chan struct{}                                                                   // closed when the process exited
	Stop() error                                                                             // kill by handle and wait; idempotent
}
