package embeddings

import (
	"fmt"
	"path/filepath"
	"strings"

	"aurago/internal/onnxrt"
)

const (
	onnxRuntimeVersion = onnxrt.CPURuntimeVersion
	llamaRuntimeBuild  = "b9994"

	onnxRevision = "9639449beea46c2d70bc73c31458f71ad7eed838"
	ggufRevision = "45ce642d3fab2033d167ec09641a159010f7d9d9"

	onnxModelSHA256 = "07ed833d6e9f5701ae56e3a6d1984f6e945fd21a7ef1fc7563fd3ae8d6f24098"
	tokenizerSHA256 = "51947676cae1f991fa51c6b9a24e14ee5460e5f0b9f692f13bb3159829d1592a"
	ggufModelSHA256 = "25155b89638e501ac33495fa278d551d7545e1e2f62722a499bba1f064c080f2"

	llamaDockerCPUImage    = "ghcr.io/ggml-org/llama.cpp:server-b9994@sha256:823b6f019cafbee8878dfdd0d4750eae4f81dfafb60dc1fbefb66794a59903c8"
	llamaDockerCUDAImage   = "ghcr.io/ggml-org/llama.cpp:server-cuda-b9994@sha256:b57dce073940d6f347d59230d4d28fc947db8948f2eb162326008380b07bab77"
	llamaDockerVulkanImage = "ghcr.io/ggml-org/llama.cpp:server-vulkan-b9994@sha256:5c9c880045f1d6131aa7c90bf2695dbb3e9c9bf2e3f4513eb0672f1500318e02"
)

// The embeddings manifests use the shared onnxrt asset types; the aliases keep
// the package-internal names.
const (
	assetFile    = onnxrt.AssetFile
	assetZip     = onnxrt.AssetZip
	assetTarGzip = onnxrt.AssetTarGzip
)

type assetSpec = onnxrt.AssetSpec

var graniteONNXModelAsset = assetSpec{
	ID:       "granite-onnx-int8",
	URL:      "https://huggingface.co/yuiseki/granite-embedding-97m-multilingual-r2-ONNX/resolve/" + onnxRevision + "/onnx/model_quantized.onnx",
	Size:     97_996_811,
	SHA256:   onnxModelSHA256,
	Kind:     assetFile,
	FileName: "model_quantized.onnx",
}

var graniteTokenizerAsset = assetSpec{
	ID:       "granite-tokenizer",
	URL:      "https://huggingface.co/yuiseki/granite-embedding-97m-multilingual-r2-ONNX/resolve/" + onnxRevision + "/tokenizer.json",
	Size:     25_301_671,
	SHA256:   tokenizerSHA256,
	Kind:     assetFile,
	FileName: "tokenizer.json",
}

var graniteGGUFModelAsset = assetSpec{
	ID:       "granite-gguf-q8_0",
	URL:      "https://huggingface.co/mykor/granite-embedding-97m-multilingual-r2-GGUF/resolve/" + ggufRevision + "/granite-embedding-97M-multilingual-r2-Q8_0.gguf",
	Size:     115_061_088,
	SHA256:   ggufModelSHA256,
	Kind:     assetFile,
	FileName: "granite-embedding-97M-multilingual-r2-Q8_0.gguf",
}

var runtimeManifest = map[string]assetSpec{
	"llama-linux-amd64-cpu": {
		ID: "llama-linux-amd64-cpu", URL: "https://github.com/ggml-org/llama.cpp/releases/download/b9994/llama-b9994-bin-ubuntu-x64.tar.gz",
		Size: 15_857_307, SHA256: "c24bca8f672d2cc9a2d2b056f68d224de3ab017c512c988c56007491fc4f1587", Kind: assetTarGzip,
	},
	"llama-linux-arm64-cpu": {
		ID: "llama-linux-arm64-cpu", URL: "https://github.com/ggml-org/llama.cpp/releases/download/b9994/llama-b9994-bin-ubuntu-arm64.tar.gz",
		Size: 12_792_730, SHA256: "40cdd9660837a67c43543b24959ba1cb199fdd30ea7cc06e03d190e062a34b8d", Kind: assetTarGzip,
	},
	"llama-linux-amd64-vulkan": {
		ID: "llama-linux-amd64-vulkan", URL: "https://github.com/ggml-org/llama.cpp/releases/download/b9994/llama-b9994-bin-ubuntu-vulkan-x64.tar.gz",
		Size: 31_193_729, SHA256: "aa5ed038483a0e78f8e574bb98128e793d0fbff6550cd3dc7fa9483ca5c6917c", Kind: assetTarGzip,
	},
	"llama-linux-arm64-vulkan": {
		ID: "llama-linux-arm64-vulkan", URL: "https://github.com/ggml-org/llama.cpp/releases/download/b9994/llama-b9994-bin-ubuntu-vulkan-arm64.tar.gz",
		Size: 25_424_075, SHA256: "a3cc0908aa43a15630c524a8e81dfd0930091988c483d82db75b1ddd9492058c", Kind: assetTarGzip,
	},
	"llama-linux-amd64-cuda": {
		ID: "llama-linux-amd64-cuda", URL: "https://github.com/hybridgroup/llama-cpp-builder/releases/download/b9994/llama-b9994-bin-ubuntu-cuda-x64.tar.gz",
		Size: 84_595_494, SHA256: "84ebfbc69c1439b0b62281fa9d61c90adeac0fd15f4d3a08837d8a121a7599fc", Kind: assetTarGzip,
	},
	"llama-linux-arm64-cuda": {
		ID: "llama-linux-arm64-cuda", URL: "https://github.com/hybridgroup/llama-cpp-builder/releases/download/b9994/llama-b9994-bin-ubuntu-cuda-arm64.tar.gz",
		Size: 45_926_911, SHA256: "6f16fdb48de3dc9a8bddd47f0d61381edb9c9fe85de0cece522cd4ba9e3d2cfc", Kind: assetTarGzip,
	},
	"llama-windows-amd64-cpu": {
		ID: "llama-windows-amd64-cpu", URL: "https://github.com/ggml-org/llama.cpp/releases/download/b9994/llama-b9994-bin-win-cpu-x64.zip",
		Size: 18_254_390, SHA256: "0e2fe37115cebf2abf356fe32e3fd461f486a17dcdebeca5ad5e352cfd959c3d", Kind: assetZip,
	},
	"llama-windows-arm64-cpu": {
		ID: "llama-windows-arm64-cpu", URL: "https://github.com/ggml-org/llama.cpp/releases/download/b9994/llama-b9994-bin-win-cpu-arm64.zip",
		Size: 12_157_867, SHA256: "66286e4cedb25a3ad5d91167199fa3682026f918e5a4e3d944830a536fddef1b", Kind: assetZip,
	},
	"llama-windows-amd64-vulkan": {
		ID: "llama-windows-amd64-vulkan", URL: "https://github.com/ggml-org/llama.cpp/releases/download/b9994/llama-b9994-bin-win-vulkan-x64.zip",
		Size: 32_951_428, SHA256: "e21e93ed9e7a65682ac562ca48f97f7d0158da4826e617613f22f417548cf59f", Kind: assetZip,
	},
	"llama-windows-amd64-cuda": {
		ID: "llama-windows-amd64-cuda", URL: "https://github.com/ggml-org/llama.cpp/releases/download/b9994/llama-b9994-bin-win-cuda-12.4-x64.zip",
		Size: 248_835_867, SHA256: "e4b5a98183be49524c22070d9afd0e897b9db9d4f40fffe9e403eb6a67570b6b", Kind: assetZip,
	},
	"llama-windows-amd64-cuda-runtime": {
		ID: "llama-windows-amd64-cuda-runtime", URL: "https://github.com/ggml-org/llama.cpp/releases/download/b9994/cudart-llama-bin-win-cuda-12.4-x64.zip",
		Size: 391_443_627, SHA256: "8c79a9b226de4b3cacfd1f83d24f962d0773be79f1e7b75c6af4ded7e32ae1d6", Kind: assetZip,
	},
	"llama-darwin-arm64-metal": {
		ID: "llama-darwin-arm64-metal", URL: "https://github.com/ggml-org/llama.cpp/releases/download/b9994/llama-b9994-bin-macos-arm64.tar.gz",
		Size: 10_751_774, SHA256: "39aeba4d7cd04803ea15760f01af6990c35487b8a83446fa0e7bb394b5c5f3c5", Kind: assetTarGzip,
	},
	"llama-darwin-amd64-cpu": {
		ID: "llama-darwin-amd64-cpu", URL: "https://github.com/ggml-org/llama.cpp/releases/download/b9994/llama-b9994-bin-macos-x64.tar.gz",
		Size: 11_031_098, SHA256: "8bff6786825b5faa29991a354a8ab9ab0f32e761d8d85b7d1f6d462563fe32a4", Kind: assetTarGzip,
	},
}

func modelPath(cacheDir string, format string) string {
	switch format {
	case "gguf":
		return filepath.Join(cacheDir, "models", "gguf", graniteGGUFModelAsset.FileName)
	default:
		return filepath.Join(cacheDir, "models", "onnx", graniteONNXModelAsset.FileName)
	}
}

func tokenizerPath(cacheDir string) string {
	return filepath.Join(cacheDir, "models", "onnx", graniteTokenizerAsset.FileName)
}

func runtimeAssetPath(cacheDir string, asset assetSpec) string {
	return onnxrt.RuntimeDir(cacheDir, asset)
}

func onnxFingerprint() string {
	return strings.Join([]string{
		LocalGraniteProvider,
		"model=" + onnxModelSHA256,
		"format=onnx",
		"quantization=int8-dynamic",
		"pooling=cls",
		"normalization=l2",
		"dimensions=384",
		"runtime=onnxruntime-" + onnxRuntimeVersion,
	}, "|")
}

func ggufFingerprint() string {
	return strings.Join([]string{
		LocalGraniteProvider,
		"model=" + ggufModelSHA256,
		"format=gguf",
		"quantization=q8_0",
		"pooling=cls",
		"normalization=l2",
		"dimensions=384",
		"runtime=llama.cpp-" + llamaRuntimeBuild,
	}, "|")
}

func onnxRuntimeAsset(goos, goarch, backend string) (assetSpec, bool) {
	switch backend {
	case "cuda":
		return onnxrt.CUDARuntimeAsset(goos, goarch)
	case "cpu", "coreml":
		// The official macOS runtime bundle exposes CoreML from the same
		// archive as its CPU provider.
		return onnxrt.CPURuntimeAsset(goos, goarch)
	default:
		return assetSpec{}, false
	}
}

func llamaRuntimeAsset(goos, goarch, backend string) (assetSpec, bool) {
	asset, ok := runtimeManifest[fmt.Sprintf("llama-%s-%s-%s", goos, goarch, backend)]
	if ok {
		return asset, true
	}
	if (backend == "metal" || backend == "cpu") && goos == "darwin" && goarch == "arm64" {
		return runtimeManifest["llama-darwin-arm64-metal"], true
	}
	return assetSpec{}, false
}
