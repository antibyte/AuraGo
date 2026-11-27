package llamacpp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"aurago/internal/onnxrt"
)

// HardwareInfo is the GPU evidence of this machine and a fingerprint of it.
type HardwareInfo struct {
	NVIDIA       bool
	NVIDIAReason string
	Vulkan       bool
	VulkanReason string
	Fingerprint  string
}

// DetectHardware probes NVIDIA and Vulkan availability.
func DetectHardware(ctx context.Context) HardwareInfo {
	return DetectHardwareWith(ctx, nil)
}

// DetectHardwareWith is DetectHardware plus an optional probe (for example Docker) that may adjust the info and returns
// one extra evidence line (empty for none). The line enters the fingerprint before the os and arch lines.
func DetectHardwareWith(ctx context.Context, augment func(context.Context, *HardwareInfo) string) HardwareInfo {
	info := HardwareInfo{}
	var evidence []string

	if output, err := shortCommand(ctx, "nvidia-smi", "--query-gpu=name,driver_version", "--format=csv,noheader"); err == nil && output != "" {
		info.NVIDIA = true
		info.NVIDIAReason = output
		evidence = append(evidence, "nvidia="+output)
	}

	switch runtime.GOOS {
	case "linux":
		if renderDevice, err := onnxrt.AccessibleRenderNode("/dev/dri"); err == nil {
			info.Vulkan = true
			info.VulkanReason = renderDevice + " is accessible"
			evidence = append(evidence, "dri="+renderDevice)
		} else {
			info.VulkanReason = err.Error()
			evidence = append(evidence, "dri=unavailable:"+err.Error())
		}
	case "windows":
		if loaderEvidence := vulkanLoaderEvidence(); loaderEvidence != "" {
			info.Vulkan = true
			info.VulkanReason = "Windows Vulkan loader is available"
			evidence = append(evidence, "vulkan_loader="+loaderEvidence)
		}
	case "darwin":
		evidence = append(evidence, "apple="+runtime.GOARCH)
	}
	if augment != nil {
		if line := augment(ctx, &info); line != "" {
			evidence = append(evidence, line)
		}
	}

	evidence = append(evidence, "os="+runtime.GOOS, "arch="+runtime.GOARCH)
	sum := sha256.Sum256([]byte(strings.Join(evidence, "\n")))
	info.Fingerprint = hex.EncodeToString(sum[:])
	return info
}

// shortCommand runs a probe command with a three second limit.
func shortCommand(parent context.Context, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, name, args...)
	onnxrt.ConfigureWorkerCommand(command)
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %w", name, err)
	}
	return strings.TrimSpace(string(output)), nil
}
