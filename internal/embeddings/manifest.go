package embeddings

import (
	"path/filepath"
	"strings"

	"aurago/internal/llamacpp"
	"aurago/internal/onnxrt"
)

const (
	onnxRuntimeVersion = onnxrt.CPURuntimeVersion
	llamaRuntimeBuild  = llamacpp.Build

	onnxRevision = "9639449beea46c2d70bc73c31458f71ad7eed838"
	ggufRevision = "45ce642d3fab2033d167ec09641a159010f7d9d9"

	onnxModelSHA256 = "07ed833d6e9f5701ae56e3a6d1984f6e945fd21a7ef1fc7563fd3ae8d6f24098"
	tokenizerSHA256 = "51947676cae1f991fa51c6b9a24e14ee5460e5f0b9f692f13bb3159829d1592a"
	ggufModelSHA256 = "25155b89638e501ac33495fa278d551d7545e1e2f62722a499bba1f064c080f2"

	llamaDockerCPUImage    = llamacpp.DockerCPUImage
	llamaDockerCUDAImage   = llamacpp.DockerCUDAImage
	llamaDockerVulkanImage = llamacpp.DockerVulkanImage
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

var runtimeManifest = llamacpp.Runtimes()

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
	return llamacpp.RuntimeAsset(goos, goarch, backend)
}
