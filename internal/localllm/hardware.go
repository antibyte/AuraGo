package localllm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// GPUDevice is a sanitized physical-device record.
type GPUDevice struct {
	ComputeCapability string `json:"compute_capability,omitempty"`
	ID                string `json:"id"`
	Vendor            string `json:"vendor"`
	Device            string `json:"device"`
	Driver            string `json:"driver,omitempty"`
	RenderNode        string `json:"render_node,omitempty"`
	VRAMBytes         int64  `json:"vram_bytes,omitempty"`
	Discrete          bool   `json:"discrete"`
	DockerID          string `json:"-"`
}

// HardwareProfile is the passive compatibility result.
type HardwareProfile struct {
	OS                 string      `json:"os"`
	Architecture       string      `json:"architecture"`
	DockerAvailable    bool        `json:"docker_available"`
	Vulkan12Verified   bool        `json:"vulkan_1_2_verified"`
	Devices            []GPUDevice `json:"devices"`
	SelectedBackend    string      `json:"selected_backend,omitempty"`
	SelectedDevice     string      `json:"selected_device,omitempty"`
	Compatibility      string      `json:"compatibility"`
	AcknowledgementDue bool        `json:"acknowledgement_required"`
	Warnings           []string    `json:"warnings"`
	Fingerprint        string      `json:"fingerprint"`
}

type hardwareProbeOptions struct {
	goos          string
	goarch        string
	drmRoot       string
	dockerOnline  bool
	nvidiaToolkit bool
	nvidiaSMI     string
	vulkanSummary string
}

func probeHardware(parent context.Context, requestedBackend string, dockerOnline, nvidiaToolkit bool) HardwareProfile {
	return probeHardwareAllowed(parent, requestedBackend, dockerOnline, nvidiaToolkit, nil)
}

func probeHardwareAllowed(parent context.Context, requestedBackend string, dockerOnline, nvidiaToolkit bool, allowedBackends map[string]bool) HardwareProfile {
	opts := hardwareProbeOptions{
		goos: runtime.GOOS, goarch: runtime.GOARCH, drmRoot: "/sys/class/drm",
		dockerOnline: dockerOnline, nvidiaToolkit: nvidiaToolkit,
	}
	if opts.goos == "linux" {
		if parent == nil {
			parent = context.Background()
		}
		ctx, cancel := context.WithTimeout(parent, 3*time.Second)
		defer cancel()
		if output, err := exec.CommandContext(ctx, "nvidia-smi",
			"--query-gpu=pci.bus_id,uuid,memory.total,driver_version,compute_cap",
			"--format=csv,noheader,nounits").Output(); err == nil {
			opts.nvidiaSMI = string(output)
		}
		if output, err := exec.CommandContext(ctx, "vulkaninfo", "--summary").Output(); err == nil {
			opts.vulkanSummary = string(output)
		}
	}
	return probeHardwareWithOptionsAllowed(requestedBackend, opts, allowedBackends)
}

func probeHardwareWithOptions(requestedBackend string, opts hardwareProbeOptions) HardwareProfile {
	return probeHardwareWithOptionsAllowed(requestedBackend, opts, nil)
}

func probeHardwareWithOptionsAllowed(requestedBackend string, opts hardwareProbeOptions, allowedBackends map[string]bool) HardwareProfile {
	profile := HardwareProfile{
		OS: opts.goos, Architecture: opts.goarch, DockerAvailable: opts.dockerOnline,
		Compatibility: "unsupported", Vulkan12Verified: vulkan12OrNewer(opts.vulkanSummary),
	}
	if opts.goos != "linux" || opts.goarch != "amd64" {
		profile.Warnings = append(profile.Warnings, "v1_supports_linux_amd64_only")
		profile.Fingerprint = hardwareFingerprint(profile)
		return profile
	}
	if !opts.dockerOnline {
		profile.Warnings = append(profile.Warnings, "docker_unavailable")
	}
	profile.Devices = enumerateDRMDevices(opts.drmRoot)
	enrichNVIDIADevices(profile.Devices, opts.nvidiaSMI)

	requestedBackend = strings.ToLower(strings.TrimSpace(requestedBackend))
	if requestedBackend == "cpu" {
		profile.SelectedBackend = "cpu"
		profile.Compatibility = "experimental"
		profile.AcknowledgementDue = true
		profile.Warnings = append(profile.Warnings, "cpu_mode_may_be_unacceptably_slow")
		profile.Fingerprint = hardwareFingerprint(profile)
		return profile
	}

	selectBackend := func(vendor string) string {
		switch vendor {
		case "nvidia":
			return "cuda"
		case "intel":
			return "sycl"
		default:
			return "vulkan"
		}
	}
	backendOrder := []string{requestedBackend}
	if requestedBackend == "" || requestedBackend == "auto" {
		backendOrder = []string{"cuda", "sycl", "vulkan"}
	}
	var selected *GPUDevice
	for _, backend := range backendOrder {
		if allowedBackends != nil && !allowedBackends[backend] {
			continue
		}
		for pass := 0; pass < 2 && selected == nil; pass++ {
			for index := range profile.Devices {
				device := &profile.Devices[index]
				deviceBackend := selectBackend(device.Vendor)
				matches := backend == deviceBackend || backend == "vulkan" && device.RenderNode != ""
				if backend == "cuda" && (!opts.nvidiaToolkit || device.DockerID == "") {
					continue
				}
				preferred := device.Discrete && device.VRAMBytes >= 8<<30
				if matches && ((pass == 0 && preferred) || (pass == 1 && !preferred)) {
					selected = device
					profile.SelectedBackend = backend
					profile.SelectedDevice = device.ID
					break
				}
			}
		}
		if selected != nil {
			break
		}
	}
	if selected != nil {
		device := *selected
		if device.Discrete && device.VRAMBytes >= 8<<30 {
			profile.Compatibility = "recommended"
		} else {
			profile.Compatibility = "experimental"
			profile.AcknowledgementDue = true
			if !device.Discrete {
				profile.Warnings = append(profile.Warnings, "integrated_gpu_may_be_unacceptably_slow")
			}
			if device.VRAMBytes <= 0 {
				profile.Warnings = append(profile.Warnings, "vram_unknown")
			} else if device.VRAMBytes < 8<<30 {
				profile.Warnings = append(profile.Warnings, "less_than_8gb_vram")
			}
		}
		if profile.SelectedBackend == "vulkan" && !profile.Vulkan12Verified {
			profile.Compatibility = "experimental"
			profile.AcknowledgementDue = true
			profile.Warnings = appendWarning(profile.Warnings, "vulkan_1_2_not_verified")
		}
	}
	if profile.SelectedBackend == "" {
		if requestedBackend == "cuda" {
			profile.Warnings = append(profile.Warnings, "nvidia_container_toolkit_unavailable")
		}
		profile.Warnings = append(profile.Warnings, "no_compatible_gpu_detected")
	}
	profile.Fingerprint = hardwareFingerprint(profile)
	return profile
}

func vulkan12OrNewer(summary string) bool {
	for _, line := range strings.Split(summary, "\n") {
		if !strings.Contains(strings.ToLower(line), "vulkan instance version") {
			continue
		}
		_, versionText, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		parts := strings.Split(strings.TrimSpace(versionText), ".")
		if len(parts) < 2 {
			continue
		}
		major, majorErr := strconv.Atoi(parts[0])
		minor, minorErr := strconv.Atoi(parts[1])
		if majorErr == nil && minorErr == nil && (major > 1 || major == 1 && minor >= 2) {
			return true
		}
	}
	return false
}

func enrichNVIDIADevices(devices []GPUDevice, output string) {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Split(line, ",")
		if len(fields) < 3 || len(fields) > 5 {
			continue
		}
		pci := normalizePCIID(fields[0])
		memoryMB, err := strconv.ParseInt(strings.TrimSpace(fields[2]), 10, 64)
		if err != nil {
			continue
		}
		for index := range devices {
			if devices[index].Vendor == "nvidia" && normalizePCIID(devices[index].ID) == pci {
				devices[index].DockerID = strings.TrimSpace(fields[1])
				devices[index].VRAMBytes = memoryMB << 20
				devices[index].Discrete = true
				if len(fields) >= 4 {
					devices[index].Driver = "nvidia:" + strings.TrimSpace(fields[3])
				}
				if len(fields) == 5 {
					devices[index].ComputeCapability = strings.TrimSpace(fields[4])
				}
			}
		}
	}
}

func normalizePCIID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if strings.HasPrefix(value, "00000000:") {
		value = "0000:" + strings.TrimPrefix(value, "00000000:")
	}
	return value
}

func enumerateDRMDevices(root string) []GPUDevice {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	devices := make([]GPUDevice, 0)
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "card") || strings.Contains(name, "-") {
			continue
		}
		deviceRoot := filepath.Join(root, name, "device")
		vendorHex := readTrimmed(filepath.Join(deviceRoot, "vendor"))
		deviceHex := readTrimmed(filepath.Join(deviceRoot, "device"))
		if vendorHex == "" {
			continue
		}
		physicalID := name + ":" + strings.TrimPrefix(deviceHex, "0x")
		if resolved, resolveErr := filepath.EvalSymlinks(deviceRoot); resolveErr == nil {
			if base := filepath.Base(resolved); strings.Contains(base, ":") {
				physicalID = base
			}
		}
		device := GPUDevice{
			ID:     physicalID,
			Vendor: drmVendor(vendorHex), Device: deviceHex,
			RenderNode: findRenderNode(deviceRoot),
			VRAMBytes:  readInt64(filepath.Join(deviceRoot, "mem_info_vram_total")),
			Driver:     drmDriver(deviceRoot),
		}
		if device.Vendor != "nvidia" {
			device.DockerID = strings.TrimPrefix(name, "card")
		}
		device.Discrete = likelyDiscreteGPU(
			device.Vendor,
			device.Device,
			readTrimmed(filepath.Join(deviceRoot, "boot_vga")),
			device.VRAMBytes,
		)
		devices = append(devices, device)
	}
	return devices
}

func likelyDiscreteGPU(vendor, device, bootVGA string, vramBytes int64) bool {
	device = strings.ToLower(strings.TrimSpace(device))
	switch vendor {
	case "nvidia":
		// NVIDIA identity and VRAM are confirmed by nvidia-smi before CUDA
		// selection; enumerateDRMDevices must not guess.
		return false
	case "intel":
		// Known Arc discrete IDs. Other Intel devices stay conservative unless
		// they are a non-boot adapter with dedicated VRAM.
		switch device {
		case "0xe20b", "0x56a0", "0x56a1", "0x56a2", "0x56a5", "0x5690", "0x5691", "0x5692":
			return true
		}
		return bootVGA == "0" && vramBytes >= 4<<30
	default:
		// A primary AMD/unknown adapter may be an APU/iGPU backed by shared
		// memory. Treat it as experimental unless it is a non-boot adapter.
		return bootVGA == "0" && vramBytes > 0
	}
}

func drmDriver(deviceRoot string) string {
	driver := ""
	if resolved, err := filepath.EvalSymlinks(filepath.Join(deviceRoot, "driver")); err == nil {
		driver = filepath.Base(resolved)
	}
	version := readTrimmed(filepath.Join(deviceRoot, "driver", "module", "version"))
	if driver == "" {
		return version
	}
	if version != "" {
		return driver + ":" + version
	}
	return driver
}

func findRenderNode(deviceRoot string) string {
	entries, _ := os.ReadDir(filepath.Join(deviceRoot, "drm"))
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "renderD") {
			return "/dev/dri/" + entry.Name()
		}
	}
	return ""
}

func drmVendor(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "0x10de":
		return "nvidia"
	case "0x8086":
		return "intel"
	case "0x1002":
		return "amd"
	default:
		return "unknown"
	}
}

func readTrimmed(path string) string {
	value, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(value))
}

func readInt64(path string) int64 {
	value, err := strconv.ParseInt(readTrimmed(path), 10, 64)
	if err != nil {
		return 0
	}
	return value
}

func hardwareFingerprint(profile HardwareProfile) string {
	copy := profile
	copy.Fingerprint = ""
	payload, _ := json.Marshal(copy)
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func (p HardwareProfile) selectedGPU() (GPUDevice, error) {
	for _, device := range p.Devices {
		if device.ID == p.SelectedDevice {
			return device, nil
		}
	}
	return GPUDevice{}, fmt.Errorf("selected_gpu_unavailable")
}
