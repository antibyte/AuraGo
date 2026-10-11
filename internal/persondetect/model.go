// Package persondetect runs the D-FINE person detector in an isolated worker
// process on the ONNX Runtime WebGPU execution provider (GPU/iGPU) or on the
// CPU, with pinned model downloads, a WebGPU model patcher, a startup
// self-test and restart supervision.
package persondetect

import "aurago/internal/onnxrt"

// ModelTier selects the detector model.
type ModelTier string

const (
	ModelStandard ModelTier = "standard" // D-FINE-N
	ModelAccurate ModelTier = "accurate" // D-FINE-S
)

// Backend is the requested execution backend.
type Backend string

const (
	BackendAuto Backend = "auto"
	BackendGPU  Backend = "gpu"
	BackendCPU  Backend = "cpu"
)

// ValidModelTier reports whether s names a model tier.
func ValidModelTier(s string) bool {
	switch ModelTier(s) {
	case ModelStandard, ModelAccurate:
		return true
	default:
		return false
	}
}

// ValidBackend reports whether s names a backend.
func ValidBackend(s string) bool {
	switch Backend(s) {
	case BackendAuto, BackendGPU, BackendCPU:
		return true
	default:
		return false
	}
}

// D-FINE ONNX I/O, identical for both tiers: input pixel_values float32
// [1,3,640,640] RGB /255 (plain resize, no normalization); outputs logits
// [1,300,80] (sigmoid, class 0 = person) and pred_boxes [1,300,4]
// (cx, cy, w, h normalized). No NMS.
const (
	dfineInputName   = "pixel_values"
	dfineLogitsName  = "logits"
	dfineBoxesName   = "pred_boxes"
	dfineInputSize   = 640
	dfineQueries     = 300
	dfineClasses     = 80
	dfinePersonClass = 0
)

// ModelSpec describes one pinned detector model.
type ModelSpec struct {
	Tier        ModelTier
	Name        string // "D-FINE-N" | "D-FINE-S"
	Asset       onnxrt.AssetSpec
	InputName   string // "pixel_values"
	LogitsName  string // "logits"
	BoxesName   string // "pred_boxes"
	InputSize   int    // 640
	Queries     int    // 300
	Classes     int    // 80
	PersonClass int    // 0
}

// Both models are Apache-2.0 (D-FINE). D-FINE-N is the AnnotateIt export of
// ustc-community/dfine-nano-coco (onnx-community/dfine_n_coco-ONNX finds no
// persons); D-FINE-S is onnx-community's export of
// ustc-community/dfine-small-coco.
var models = map[ModelTier]ModelSpec{
	ModelStandard: {
		Tier: ModelStandard,
		Name: "D-FINE-N",
		Asset: onnxrt.AssetSpec{
			ID:       "dfine-n-coco",
			URL:      "https://huggingface.co/AnnotateIt/dfine-nano-coco-onnx/resolve/571d3971bfb47dc5ddd3746134dd464954bc9df0/model.onnx",
			Size:     15_434_935,
			SHA256:   "b0ec2e5793f3a3cbd07440d098645a0e102cd450fb92f4f35e713f36e35d152e",
			Kind:     onnxrt.AssetFile,
			FileName: "model.onnx",
		},
		InputName: dfineInputName, LogitsName: dfineLogitsName, BoxesName: dfineBoxesName,
		InputSize: dfineInputSize, Queries: dfineQueries, Classes: dfineClasses, PersonClass: dfinePersonClass,
	},
	ModelAccurate: {
		Tier: ModelAccurate,
		Name: "D-FINE-S",
		Asset: onnxrt.AssetSpec{
			ID:       "dfine-s-coco",
			URL:      "https://huggingface.co/onnx-community/dfine_s_coco-ONNX/resolve/a3cf03147a9b86c78475139115c8ac142577352d/onnx/model.onnx",
			Size:     41_535_197,
			SHA256:   "cd8a49a945feda6d28c6304ae8ae85c2759ba1d78a5a83a22c5ce8db82ef7238",
			Kind:     onnxrt.AssetFile,
			FileName: "model.onnx",
		},
		InputName: dfineInputName, LogitsName: dfineLogitsName, BoxesName: dfineBoxesName,
		InputSize: dfineInputSize, Queries: dfineQueries, Classes: dfineClasses, PersonClass: dfinePersonClass,
	},
}

// Model returns the pinned spec of a tier.
func Model(tier ModelTier) (ModelSpec, bool) {
	spec, ok := models[tier]
	return spec, ok
}
