// Package llamacpp holds the pinned llama.cpp build shared by local embeddings and the Chibi Sensei chat runtime: release assets, Docker images, server discovery, loopback helpers, GPU evidence and hardware detection. It imports only internal/onnxrt.
package llamacpp

// Build is the pinned llama.cpp release tag.
const Build = "b9994"

// Digest-pinned llama.cpp server images of the pinned build.
const (
	DockerCPUImage    = "ghcr.io/ggml-org/llama.cpp:server-b9994@sha256:823b6f019cafbee8878dfdd0d4750eae4f81dfafb60dc1fbefb66794a59903c8"
	DockerCUDAImage   = "ghcr.io/ggml-org/llama.cpp:server-cuda-b9994@sha256:b57dce073940d6f347d59230d4d28fc947db8948f2eb162326008380b07bab77"
	DockerVulkanImage = "ghcr.io/ggml-org/llama.cpp:server-vulkan-b9994@sha256:5c9c880045f1d6131aa7c90bf2695dbb3e9c9bf2e3f4513eb0672f1500318e02"
)

// DockerImage returns the pinned server image of a backend (cpu, cuda or vulkan).
func DockerImage(backend string) (string, bool) {
	switch backend {
	case "cpu":
		return DockerCPUImage, true
	case "cuda":
		return DockerCUDAImage, true
	case "vulkan":
		return DockerVulkanImage, true
	default:
		return "", false
	}
}
