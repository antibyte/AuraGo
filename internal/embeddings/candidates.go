package embeddings

import (
	"context"
	"strings"

	"aurago/internal/llamacpp"
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

type hardwareInfo = llamacpp.HardwareInfo

func detectHardware(ctx context.Context) hardwareInfo {
	return llamacpp.DetectHardwareWith(ctx, augmentDockerHardware)
}
