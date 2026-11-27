package llamacpp

import (
	"fmt"
	"maps"

	"aurago/internal/onnxrt"
)

// AssetSpec is the shared pinned download description.
type AssetSpec = onnxrt.AssetSpec

// WindowsCUDARuntimeID is the manifest ID of the CUDA runtime libraries that windows/amd64 CUDA needs next to the server.
const WindowsCUDARuntimeID = "llama-windows-amd64-cuda-runtime"

var runtimeManifest = map[string]AssetSpec{
	"llama-linux-amd64-cpu": {
		ID: "llama-linux-amd64-cpu", URL: "https://github.com/ggml-org/llama.cpp/releases/download/b9994/llama-b9994-bin-ubuntu-x64.tar.gz",
		Size: 15_857_307, SHA256: "c24bca8f672d2cc9a2d2b056f68d224de3ab017c512c988c56007491fc4f1587", Kind: onnxrt.AssetTarGzip,
	},
	"llama-linux-arm64-cpu": {
		ID: "llama-linux-arm64-cpu", URL: "https://github.com/ggml-org/llama.cpp/releases/download/b9994/llama-b9994-bin-ubuntu-arm64.tar.gz",
		Size: 12_792_730, SHA256: "40cdd9660837a67c43543b24959ba1cb199fdd30ea7cc06e03d190e062a34b8d", Kind: onnxrt.AssetTarGzip,
	},
	"llama-linux-amd64-vulkan": {
		ID: "llama-linux-amd64-vulkan", URL: "https://github.com/ggml-org/llama.cpp/releases/download/b9994/llama-b9994-bin-ubuntu-vulkan-x64.tar.gz",
		Size: 31_193_729, SHA256: "aa5ed038483a0e78f8e574bb98128e793d0fbff6550cd3dc7fa9483ca5c6917c", Kind: onnxrt.AssetTarGzip,
	},
	"llama-linux-arm64-vulkan": {
		ID: "llama-linux-arm64-vulkan", URL: "https://github.com/ggml-org/llama.cpp/releases/download/b9994/llama-b9994-bin-ubuntu-vulkan-arm64.tar.gz",
		Size: 25_424_075, SHA256: "a3cc0908aa43a15630c524a8e81dfd0930091988c483d82db75b1ddd9492058c", Kind: onnxrt.AssetTarGzip,
	},
	"llama-linux-amd64-cuda": {
		ID: "llama-linux-amd64-cuda", URL: "https://github.com/hybridgroup/llama-cpp-builder/releases/download/b9994/llama-b9994-bin-ubuntu-cuda-x64.tar.gz",
		Size: 84_595_494, SHA256: "84ebfbc69c1439b0b62281fa9d61c90adeac0fd15f4d3a08837d8a121a7599fc", Kind: onnxrt.AssetTarGzip,
	},
	"llama-linux-arm64-cuda": {
		ID: "llama-linux-arm64-cuda", URL: "https://github.com/hybridgroup/llama-cpp-builder/releases/download/b9994/llama-b9994-bin-ubuntu-cuda-arm64.tar.gz",
		Size: 45_926_911, SHA256: "6f16fdb48de3dc9a8bddd47f0d61381edb9c9fe85de0cece522cd4ba9e3d2cfc", Kind: onnxrt.AssetTarGzip,
	},
	"llama-windows-amd64-cpu": {
		ID: "llama-windows-amd64-cpu", URL: "https://github.com/ggml-org/llama.cpp/releases/download/b9994/llama-b9994-bin-win-cpu-x64.zip",
		Size: 18_254_390, SHA256: "0e2fe37115cebf2abf356fe32e3fd461f486a17dcdebeca5ad5e352cfd959c3d", Kind: onnxrt.AssetZip,
	},
	"llama-windows-arm64-cpu": {
		ID: "llama-windows-arm64-cpu", URL: "https://github.com/ggml-org/llama.cpp/releases/download/b9994/llama-b9994-bin-win-cpu-arm64.zip",
		Size: 12_157_867, SHA256: "66286e4cedb25a3ad5d91167199fa3682026f918e5a4e3d944830a536fddef1b", Kind: onnxrt.AssetZip,
	},
	"llama-windows-amd64-vulkan": {
		ID: "llama-windows-amd64-vulkan", URL: "https://github.com/ggml-org/llama.cpp/releases/download/b9994/llama-b9994-bin-win-vulkan-x64.zip",
		Size: 32_951_428, SHA256: "e21e93ed9e7a65682ac562ca48f97f7d0158da4826e617613f22f417548cf59f", Kind: onnxrt.AssetZip,
	},
	"llama-windows-amd64-cuda": {
		ID: "llama-windows-amd64-cuda", URL: "https://github.com/ggml-org/llama.cpp/releases/download/b9994/llama-b9994-bin-win-cuda-12.4-x64.zip",
		Size: 248_835_867, SHA256: "e4b5a98183be49524c22070d9afd0e897b9db9d4f40fffe9e403eb6a67570b6b", Kind: onnxrt.AssetZip,
	},
	"llama-windows-amd64-cuda-runtime": {
		ID: "llama-windows-amd64-cuda-runtime", URL: "https://github.com/ggml-org/llama.cpp/releases/download/b9994/cudart-llama-bin-win-cuda-12.4-x64.zip",
		Size: 391_443_627, SHA256: "8c79a9b226de4b3cacfd1f83d24f962d0773be79f1e7b75c6af4ded7e32ae1d6", Kind: onnxrt.AssetZip,
	},
	"llama-darwin-arm64-metal": {
		ID: "llama-darwin-arm64-metal", URL: "https://github.com/ggml-org/llama.cpp/releases/download/b9994/llama-b9994-bin-macos-arm64.tar.gz",
		Size: 10_751_774, SHA256: "39aeba4d7cd04803ea15760f01af6990c35487b8a83446fa0e7bb394b5c5f3c5", Kind: onnxrt.AssetTarGzip,
	},
	"llama-darwin-amd64-cpu": {
		ID: "llama-darwin-amd64-cpu", URL: "https://github.com/ggml-org/llama.cpp/releases/download/b9994/llama-b9994-bin-macos-x64.tar.gz",
		Size: 11_031_098, SHA256: "8bff6786825b5faa29991a354a8ab9ab0f32e761d8d85b7d1f6d462563fe32a4", Kind: onnxrt.AssetTarGzip,
	},
}

// Runtimes returns a copy of the native runtime manifest keyed by asset ID.
func Runtimes() map[string]AssetSpec { return maps.Clone(runtimeManifest) }

// RuntimeAsset returns the native llama.cpp archive of a platform and backend (cpu, vulkan, cuda or metal).
func RuntimeAsset(goos, goarch, backend string) (AssetSpec, bool) {
	asset, ok := runtimeManifest[fmt.Sprintf("llama-%s-%s-%s", goos, goarch, backend)]
	if ok {
		return asset, true
	}
	if (backend == "metal" || backend == "cpu") && goos == "darwin" && goarch == "arm64" {
		return runtimeManifest["llama-darwin-arm64-metal"], true
	}
	return AssetSpec{}, false
}

// SupplementalAssets lists the extra runtime archives a backend needs: on windows/amd64 the CUDA runtime libraries
// (unpacked as their own runtime root and put on the library path); nil everywhere else.
func SupplementalAssets(goos, goarch, backend string) []AssetSpec {
	if goos == "windows" && goarch == "amd64" && backend == "cuda" {
		return []AssetSpec{runtimeManifest[WindowsCUDARuntimeID]}
	}
	return nil
}
