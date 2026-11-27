package chatrt

import (
	"context"
	"net/url"
	"strings"
	"time"

	"aurago/internal/llamacpp"
)

// hardwareProbeTimeout bounds one hardware probe.
const hardwareProbeTimeout = 20 * time.Second

// resolution is the backend choice for one configured backend value.
type resolution struct {
	requested   string
	selected    string
	unsupported bool
	assets      []llamacpp.AssetSpec // install set: CPU archive first
}

func normalizeBackend(requested string) string {
	switch b := strings.ToLower(strings.TrimSpace(requested)); b {
	case "cpu", "vulkan", "cuda":
		return b
	default:
		return "auto"
	}
}

// hasBackendAsset reports whether the platform has an archive of its own for a GPU backend: the install set then
// holds more than the CPU archive.
func hasBackendAsset(l Launcher, backend string) bool {
	set, ok := l.Assets(backend)
	return ok && len(set) > 1
}

// resolveBackend picks the backend of a platform. Hardware is consulted only for "auto".
func resolveBackend(ctx context.Context, l Launcher, requested string) resolution {
	res := resolution{requested: requested}
	goos, goarch := l.Platform()
	cpuSet, ok := l.Assets("cpu")
	if !ok {
		res.unsupported = true
		return res
	}
	selected := "cpu"
	switch requested {
	case "auto":
		switch {
		case goos == "darwin" && goarch == "arm64":
			selected = "metal"
		default:
			hw := l.Hardware(ctx)
			switch {
			case hw.NVIDIA && hasBackendAsset(l, "cuda"):
				selected = "cuda"
			case hw.Vulkan && hasBackendAsset(l, "vulkan"):
				selected = "vulkan"
			}
		}
	case "cuda", "vulkan":
		if hasBackendAsset(l, requested) {
			selected = requested
		}
	}
	res.selected = selected
	res.assets = cpuSet
	if selected != "cpu" {
		if set, ok := l.Assets(selected); ok {
			res.assets = set
		}
	}
	return res
}

// launchAssets returns the runtime archives of one start: the CPU archive alone, or the GPU archive and its
// supplements (a darwin/arm64 metal start uses the one archive that serves both).
func (r resolution) launchAssets(backend string) []llamacpp.AssetSpec {
	if len(r.assets) == 0 {
		return nil
	}
	if backend == "cpu" || r.selected == "cpu" {
		return r.assets[:1]
	}
	if len(r.assets) == 1 {
		return r.assets
	}
	return r.assets[1:]
}

func (r resolution) totalBytes() int64 {
	var total int64
	for _, a := range r.assets {
		total += a.Size
	}
	return total
}

func (r resolution) host() string {
	if len(r.assets) == 0 {
		return ""
	}
	u, err := url.Parse(r.assets[0].URL)
	if err != nil {
		return ""
	}
	return u.Host
}

// resolve returns the cached resolution of the configured backend. A changed configured backend drops the
// cache, the install check and the sticky GPU fallback.
//
// The hardware probe runs on the manager's lifetime context with its own timeout, never on the caller's request
// context: a request that ended early must not pin "no GPU" for the lifetime of the manager. A result produced
// while the manager is closing is returned but not cached.
func (m *Manager) resolve(_ context.Context) resolution {
	requested := normalizeBackend(m.cfg().Backend)
	m.mu.Lock()
	if m.res != nil && m.res.requested != requested {
		m.res = nil
		m.installed = nil
		m.gpuFailed = false
		m.failedFor = ""
	}
	if m.res != nil {
		res := *m.res
		m.mu.Unlock()
		return res
	}
	m.mu.Unlock()
	probeCtx, cancel := context.WithTimeout(m.life, hardwareProbeTimeout)
	res := resolveBackend(probeCtx, m.launcher, requested)
	cancel()
	cacheable := m.life.Err() == nil
	m.mu.Lock()
	if m.res == nil {
		if cacheable {
			m.res = &res
		}
	} else {
		res = *m.res
	}
	m.mu.Unlock()
	return res
}
