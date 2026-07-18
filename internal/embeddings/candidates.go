package embeddings

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"aurago/internal/sandbox"
)

type candidate struct {
	ID       string
	Runtime  string
	Backend  string
	GPU      bool
	Reason   string
	Asset    assetSpec
	HasAsset bool
}

func candidateMatrix(goos, goarch, requestedBackend string, hardware hardwareInfo) []candidate {
	requestedBackend = strings.ToLower(strings.TrimSpace(requestedBackend))
	if requestedBackend == "" {
		requestedBackend = "auto"
	}
	allow := func(backend string) bool {
		if backend == "cpu" {
			return true
		}
		return requestedBackend == "auto" || requestedBackend == backend
	}
	var candidates []candidate
	addONNX := func(backend string, gpu bool, available bool, reason string) {
		if !allow(backend) || !available {
			return
		}
		asset, ok := onnxRuntimeAsset(goos, goarch, backend)
		candidates = append(candidates, candidate{
			ID: "onnx-" + backend, Runtime: "onnxruntime", Backend: backend, GPU: gpu,
			Reason: reason, Asset: asset, HasAsset: ok,
		})
	}
	addLlama := func(backend string, gpu bool, available bool, reason string) {
		if !allow(backend) || !available {
			return
		}
		asset, ok := llamaRuntimeAsset(goos, goarch, backend)
		candidates = append(candidates, candidate{
			ID: "llama-" + backend, Runtime: "llama.cpp", Backend: backend, GPU: gpu,
			Reason: reason, Asset: asset, HasAsset: ok,
		})
	}

	switch goos {
	case "linux":
		addONNX("cuda", true, hardware.NVIDIA, hardware.NVIDIAReason)
		addLlama("cuda", true, hardware.NVIDIA, hardware.NVIDIAReason)
		addLlama("vulkan", true, hardware.Vulkan, hardware.VulkanReason)
	case "windows":
		if goarch == "amd64" {
			addONNX("cuda", true, hardware.NVIDIA, hardware.NVIDIAReason)
			addLlama("cuda", true, hardware.NVIDIA, hardware.NVIDIAReason)
			addLlama("vulkan", true, hardware.Vulkan, hardware.VulkanReason)
		}
	case "darwin":
		if goarch == "arm64" {
			addONNX("coreml", true, true, "Apple Silicon")
			addLlama("metal", true, true, "Apple Silicon")
		}
	}

	addONNX("cpu", false, true, "")
	addLlama("cpu", false, true, "")
	return candidates
}

type hardwareInfo struct {
	NVIDIA       bool
	NVIDIAReason string
	Vulkan       bool
	VulkanReason string
	Fingerprint  string
}

func detectHardware(ctx context.Context) hardwareInfo {
	info := hardwareInfo{}
	var evidence []string

	if output, err := shortCommand(ctx, "nvidia-smi", "--query-gpu=name,driver_version", "--format=csv,noheader"); err == nil && output != "" {
		info.NVIDIA = true
		info.NVIDIAReason = output
		evidence = append(evidence, "nvidia="+output)
	}

	switch runtime.GOOS {
	case "linux":
		if _, err := os.Stat("/dev/dri"); err == nil {
			info.Vulkan = true
			info.VulkanReason = "/dev/dri is available"
			evidence = append(evidence, "dri=present")
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
	if dockerEvidence := augmentDockerHardware(ctx, &info); dockerEvidence != "" {
		evidence = append(evidence, dockerEvidence)
	}

	evidence = append(evidence, "os="+runtime.GOOS, "arch="+runtime.GOARCH)
	sum := sha256.Sum256([]byte(strings.Join(evidence, "\n")))
	info.Fingerprint = hex.EncodeToString(sum[:])
	return info
}

func shortCommand(parent context.Context, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, name, args...)
	configureHiddenProcess(command)
	command.Env = sandbox.FilterEnv(os.Environ())
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %w", name, err)
	}
	return strings.TrimSpace(string(output)), nil
}
