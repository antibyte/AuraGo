package tools

import (
	"os"
	"runtime"
	"strings"

	"aurago/internal/config"
)

const ollamaEmbContainerName = "aurago_ollama_embeddings"

// GPUInfo describes a detected GPU backend for container passthrough.
type GPUInfo struct {
	Backend string // "nvidia", "amd", "intel", "vulkan", "none"
	Devices []string
	Name    string   // human-readable GPU name if detected
	Env     []string // extra environment variables to pass into the container
}

// DetectGPU probes the host for available GPU hardware.
// Only meaningful on Linux; returns "none" on other platforms.
func DetectGPU(preferred string) GPUInfo {
	if runtime.GOOS != "linux" {
		return GPUInfo{Backend: "none"}
	}

	// If a specific backend is requested (not "auto"), only check that one.
	if preferred != "" && preferred != "auto" {
		switch preferred {
		case "nvidia":
			if info, ok := detectNVIDIA(); ok {
				return info
			}
		case "amd":
			if info, ok := detectAMD(); ok {
				return info
			}
		case "intel":
			if info, ok := detectIntel(); ok {
				return info
			}
		case "vulkan":
			if info, ok := detectVulkan(); ok {
				return info
			}
		}
		return GPUInfo{Backend: "none"}
	}

	// Auto-detect: try NVIDIA → AMD → Intel (ordered by Ollama support maturity)
	if info, ok := detectNVIDIA(); ok {
		return info
	}
	if info, ok := detectAMD(); ok {
		return info
	}
	if info, ok := detectIntel(); ok {
		return info
	}
	// Vulkan: generic DRI fallback for older GPUs (pre-ROCm AMD, older iGPUs, etc.)
	if info, ok := detectVulkan(); ok {
		return info
	}
	return GPUInfo{Backend: "none"}
}

func detectNVIDIA() (GPUInfo, bool) {
	// NVIDIA exposes /dev/nvidia0, /dev/nvidia1, etc. via the kernel driver.
	if _, err := os.Stat("/dev/nvidia0"); err != nil {
		return GPUInfo{}, false
	}
	return GPUInfo{
		Backend: "nvidia",
		Name:    "NVIDIA GPU",
	}, true
}

// driDeviceNodes returns only true character-device paths under /dev/dri,
// skipping subdirectories such as by-path and by-id.
func driDeviceNodes() []string {
	entries, err := os.ReadDir("/dev/dri")
	if err != nil {
		return nil
	}
	var nodes []string
	for _, e := range entries {
		p := "/dev/dri/" + e.Name()
		fi, err := os.Stat(p) // follows symlinks
		if err != nil {
			continue
		}
		if fi.Mode()&os.ModeDevice == 0 {
			continue // skip directories (by-path, by-id) and other non-devices
		}
		nodes = append(nodes, p)
	}
	return nodes
}

func detectAMD() (GPUInfo, bool) {
	// AMD ROCm uses /dev/kfd (kernel fusion driver) and /dev/dri/*.
	if _, err := os.Stat("/dev/kfd"); err != nil {
		return GPUInfo{}, false
	}
	devices := append([]string{"/dev/kfd"}, driDeviceNodes()...)
	return GPUInfo{
		Backend: "amd",
		Devices: devices,
		Name:    "AMD GPU (ROCm)",
	}, true
}

func detectIntel() (GPUInfo, bool) {
	// Intel iGPU/Arc use /dev/dri/renderD128 etc.
	// Verify the vendor is Intel (0x8086) via sysfs.
	entries, err := os.ReadDir("/sys/class/drm")
	if err != nil {
		return GPUInfo{}, false
	}
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "card") || strings.Contains(e.Name(), "-") {
			continue
		}
		vendorPath := "/sys/class/drm/" + e.Name() + "/device/vendor"
		data, err := os.ReadFile(vendorPath)
		if err != nil {
			continue
		}
		if strings.TrimSpace(string(data)) == "0x8086" {
			return GPUInfo{
				Backend: "intel",
				Devices: driDeviceNodes(),
				Name:    "Intel GPU",
			}, true
		}
	}
	return GPUInfo{}, false
}

// detectVulkan detects any DRI render node as a generic Vulkan-capable GPU.
// This covers older AMD GPUs (pre-ROCm / pre-RDNA), older integrated GPUs,
// and any other GPU that supports Vulkan 1.2+ but not vendor-specific compute APIs.
func detectVulkan() (GPUInfo, bool) {
	devices := driDeviceNodes()
	if len(devices) == 0 {
		return GPUInfo{}, false
	}
	return GPUInfo{
		Backend: "vulkan",
		Devices: devices,
		Name:    "Vulkan GPU (DRI)",
		Env:     []string{"OLLAMA_GPU_BACKEND=vulkan"},
	}, true
}

// ollamaImageForGPU returns the appropriate Ollama Docker image tag for the GPU backend.
func ollamaImageForGPU(gpu GPUInfo) string {
	switch gpu.Backend {
	case "amd":
		return "ollama/ollama:" + ollamaImageTag + "-rocm"
	default:
		// NVIDIA, Intel, Vulkan, and CPU all work with the standard image.
		// Vulkan uses OLLAMA_GPU_BACKEND=vulkan env var (set in GPUInfo.Env).
		return "ollama/ollama:" + ollamaImageTag
	}
}

// EnsureOllamaEmbeddingsRunning ensures a managed Ollama container is running
// for local embeddings. Follows the same idempotent pattern as EnsureGotenbergRunning.
func EnsureOllamaEmbeddingsRunning(cfg *config.Config, logger ollamaEnsureLogger) {
	ensureOllamaRuntime(cfg, true, logger)
}

// hostGroupIDs resolves group names to their numeric GIDs from the host's
// /etc/group file. Using numeric GIDs avoids "no matching entries in group
// file" errors when the group name does not exist inside the container image.
// Groups that are not present on the host are silently skipped.
func hostGroupIDs(names []string) []string {
	data, err := os.ReadFile("/etc/group")
	if err != nil {
		return nil
	}
	want := make(map[string]bool, len(names))
	for _, n := range names {
		want[n] = true
	}
	var ids []string
	for _, line := range strings.Split(string(data), "\n") {
		// Format: name:password:gid:members
		parts := strings.SplitN(line, ":", 4)
		if len(parts) < 3 {
			continue
		}
		if want[parts[0]] {
			ids = append(ids, parts[2])
		}
	}
	return ids
}
