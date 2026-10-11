package onnxrt

import "fmt"

const (
	// CPURuntimeVersion is the ONNX Runtime release of the CPU and CUDA bundles.
	CPURuntimeVersion = "1.26.0"
	// WebGPURuntimeVersion is the onnxruntime-webgpu wheel release.
	WebGPURuntimeVersion = "1.27.0"
)

var runtimeAssets = map[string]AssetSpec{
	"ort-linux-amd64-cpu": {
		ID: "ort-linux-amd64-cpu", URL: "https://github.com/microsoft/onnxruntime/releases/download/v1.26.0/onnxruntime-linux-x64-1.26.0.tgz",
		Size: 8_590_023, SHA256: "1254da24fb389cf39dc0ff3451ab48301740ffbfcbaf646849df92f80ee92c57", Kind: AssetTarGzip,
	},
	"ort-linux-arm64-cpu": {
		ID: "ort-linux-arm64-cpu", URL: "https://github.com/microsoft/onnxruntime/releases/download/v1.26.0/onnxruntime-linux-aarch64-1.26.0.tgz",
		Size: 7_608_947, SHA256: "34ff1c2d0f12e2cf3d33a0c5f82e39792e1d581fbd6968fd7c30d173654be01a", Kind: AssetTarGzip,
	},
	"ort-linux-amd64-cuda": {
		ID: "ort-linux-amd64-cuda", URL: "https://github.com/microsoft/onnxruntime/releases/download/v1.26.0/onnxruntime-linux-x64-gpu-1.26.0.tgz",
		Size: 225_080_769, SHA256: "cb7df7ee2ca0f962c7ce7c839aeae36223d146a91fb4646d62fb0046f297479f", Kind: AssetTarGzip,
	},
	"ort-windows-amd64-cpu": {
		ID: "ort-windows-amd64-cpu", URL: "https://github.com/microsoft/onnxruntime/releases/download/v1.26.0/onnxruntime-win-x64-1.26.0.zip",
		Size: 75_675_381, SHA256: "6ebe99b5564bf4d029b6e93eac9ff423682b6212eade769e9ca3f685eaf500b4", Kind: AssetZip,
	},
	"ort-windows-arm64-cpu": {
		ID: "ort-windows-arm64-cpu", URL: "https://github.com/microsoft/onnxruntime/releases/download/v1.26.0/onnxruntime-win-arm64-1.26.0.zip",
		Size: 77_548_904, SHA256: "852e89621fb752b261821dda131042ed7c7fa18d8bb06768bd4eb7fa7086d87f", Kind: AssetZip,
	},
	"ort-windows-amd64-cuda": {
		ID: "ort-windows-amd64-cuda", URL: "https://github.com/microsoft/onnxruntime/releases/download/v1.26.0/onnxruntime-win-x64-gpu-1.26.0.zip",
		Size: 300_180_373, SHA256: "1133b1bcb0fb6f82b1c5b470b7cc15f9080a58b27dbc7b579a1fd63125ec2a15", Kind: AssetZip,
	},
	"ort-darwin-arm64-cpu": {
		ID: "ort-darwin-arm64-cpu", URL: "https://github.com/microsoft/onnxruntime/releases/download/v1.26.0/onnxruntime-osx-arm64-1.26.0.tgz",
		Size: 31_717_869, SHA256: "7a1280bbb1701ea514f71828765237e7896e0f2e1cd332f1f70dbd5c3e33aca3", Kind: AssetTarGzip,
	},
	"ort-linux-amd64-webgpu": {
		ID:     "ort-linux-amd64-webgpu",
		URL:    "https://files.pythonhosted.org/packages/24/11/6cd5f840be97819cbd172bacc1868726ab5fa54ceaaa79508c5a64e61233/onnxruntime_webgpu-1.27.0-cp312-cp312-manylinux_2_27_x86_64.manylinux_2_28_x86_64.whl",
		Size:   25_163_729,
		SHA256: "fe5e49e00ad71bc667316d57670bfbf525438783d4a3468e0d337314ec985cbe",
		Kind:   AssetWheel,
		Members: []string{
			"onnxruntime/capi/libonnxruntime.so.1.27.0",
			"onnxruntime/capi/libonnxruntime_providers_shared.so",
		},
	},
	"ort-windows-amd64-webgpu": {
		ID:     "ort-windows-amd64-webgpu",
		URL:    "https://files.pythonhosted.org/packages/df/28/016260c51c877ba5b3eba823b43107e894659e73e0ecf250eb07e801e3c2/onnxruntime_webgpu-1.27.0-cp312-cp312-win_amd64.whl",
		Size:   27_226_899,
		SHA256: "7ef99275b13e8cb9584bd0db7a6f00ebf76095601eeccf7d34749b89ee991c19",
		Kind:   AssetWheel,
		Members: []string{
			"onnxruntime/capi/onnxruntime.dll",
			"onnxruntime/capi/onnxruntime_providers_shared.dll",
			"onnxruntime/capi/dxcompiler.dll",
			"onnxruntime/capi/dxil.dll",
		},
	},
	"ort-darwin-arm64-webgpu": {
		ID:      "ort-darwin-arm64-webgpu",
		URL:     "https://files.pythonhosted.org/packages/b4/1f/be670672fa0dc367fd28113b5e410a6e71cb6ee66c188ed1a8eb1439c941/onnxruntime_webgpu-1.27.0-cp312-cp312-macosx_14_0_arm64.whl",
		Size:    24_081_620,
		SHA256:  "8e6c43281318283a31977ebe752e7b0454db9b2dc42c768ac88641952357196a",
		Kind:    AssetWheel,
		Members: []string{"onnxruntime/capi/libonnxruntime.1.27.0.dylib"},
	},
}

func lookupRuntime(id string) (AssetSpec, bool) {
	spec, ok := runtimeAssets[id]
	if !ok {
		return AssetSpec{}, false
	}
	spec.Members = append([]string(nil), spec.Members...)
	return spec, true
}

// CPURuntimeAsset is the official ONNX Runtime 1.26.0 CPU bundle (on macOS it
// also exposes CoreML).
func CPURuntimeAsset(goos, goarch string) (AssetSpec, bool) {
	return lookupRuntime(fmt.Sprintf("ort-%s-%s-cpu", goos, goarch))
}

// CUDARuntimeAsset is the official ONNX Runtime 1.26.0 GPU (CUDA) bundle.
func CUDARuntimeAsset(goos, goarch string) (AssetSpec, bool) {
	return lookupRuntime(fmt.Sprintf("ort-%s-%s-cuda", goos, goarch))
}

// WebGPURuntimeAsset is the onnxruntime-webgpu 1.27.0 wheel; it exists for
// linux/amd64, windows/amd64 and darwin/arm64 only.
func WebGPURuntimeAsset(goos, goarch string) (AssetSpec, bool) {
	return lookupRuntime(fmt.Sprintf("ort-%s-%s-webgpu", goos, goarch))
}
