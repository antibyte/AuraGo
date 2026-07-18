package embeddings

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	onnxRuntimeVersion = "1.26.0"
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

type assetKind string

const (
	assetFile    assetKind = "file"
	assetZip     assetKind = "zip"
	assetTarGzip assetKind = "tar.gz"
)

type assetSpec struct {
	ID       string
	URL      string
	Size     int64
	SHA256   string
	Kind     assetKind
	FileName string
}

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
	"ort-linux-amd64-cpu": {
		ID: "ort-linux-amd64-cpu", URL: "https://github.com/microsoft/onnxruntime/releases/download/v1.26.0/onnxruntime-linux-x64-1.26.0.tgz",
		Size: 8_590_023, SHA256: "1254da24fb389cf39dc0ff3451ab48301740ffbfcbaf646849df92f80ee92c57", Kind: assetTarGzip,
	},
	"ort-linux-arm64-cpu": {
		ID: "ort-linux-arm64-cpu", URL: "https://github.com/microsoft/onnxruntime/releases/download/v1.26.0/onnxruntime-linux-aarch64-1.26.0.tgz",
		Size: 7_608_947, SHA256: "34ff1c2d0f12e2cf3d33a0c5f82e39792e1d581fbd6968fd7c30d173654be01a", Kind: assetTarGzip,
	},
	"ort-linux-amd64-cuda": {
		ID: "ort-linux-amd64-cuda", URL: "https://github.com/microsoft/onnxruntime/releases/download/v1.26.0/onnxruntime-linux-x64-gpu-1.26.0.tgz",
		Size: 225_080_769, SHA256: "cb7df7ee2ca0f962c7ce7c839aeae36223d146a91fb4646d62fb0046f297479f", Kind: assetTarGzip,
	},
	"ort-windows-amd64-cpu": {
		ID: "ort-windows-amd64-cpu", URL: "https://github.com/microsoft/onnxruntime/releases/download/v1.26.0/onnxruntime-win-x64-1.26.0.zip",
		Size: 75_675_381, SHA256: "6ebe99b5564bf4d029b6e93eac9ff423682b6212eade769e9ca3f685eaf500b4", Kind: assetZip,
	},
	"ort-windows-arm64-cpu": {
		ID: "ort-windows-arm64-cpu", URL: "https://github.com/microsoft/onnxruntime/releases/download/v1.26.0/onnxruntime-win-arm64-1.26.0.zip",
		Size: 77_548_904, SHA256: "852e89621fb752b261821dda131042ed7c7fa18d8bb06768bd4eb7fa7086d87f", Kind: assetZip,
	},
	"ort-windows-amd64-cuda": {
		ID: "ort-windows-amd64-cuda", URL: "https://github.com/microsoft/onnxruntime/releases/download/v1.26.0/onnxruntime-win-x64-gpu-1.26.0.zip",
		Size: 300_180_373, SHA256: "1133b1bcb0fb6f82b1c5b470b7cc15f9080a58b27dbc7b579a1fd63125ec2a15", Kind: assetZip,
	},
	"ort-darwin-arm64-cpu": {
		ID: "ort-darwin-arm64-cpu", URL: "https://github.com/microsoft/onnxruntime/releases/download/v1.26.0/onnxruntime-osx-arm64-1.26.0.tgz",
		Size: 31_717_869, SHA256: "7a1280bbb1701ea514f71828765237e7896e0f2e1cd332f1f70dbd5c3e33aca3", Kind: assetTarGzip,
	},
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
	return filepath.Join(cacheDir, "runtimes", asset.ID)
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
	assetBackend := "cpu"
	switch backend {
	case "cuda":
		assetBackend = "cuda"
	case "cpu", "coreml":
		// The official macOS runtime bundle exposes CoreML from the same
		// archive as its CPU provider.
	case "directml":
		// Microsoft does not publish an ONNX Runtime 1.26.0 DirectML bundle.
		// Keep the candidate visible as a cleanly skipped probe rather than
		// silently loading an older runtime or mislabeling the CPU archive.
		return assetSpec{}, false
	default:
		return assetSpec{}, false
	}
	asset, ok := runtimeManifest[fmt.Sprintf("ort-%s-%s-%s", goos, goarch, assetBackend)]
	return asset, ok
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

func defaultRuntimeLibraryName() string {
	switch runtime.GOOS {
	case "windows":
		return "onnxruntime.dll"
	case "darwin":
		return "libonnxruntime.dylib"
	default:
		return "libonnxruntime.so"
	}
}
