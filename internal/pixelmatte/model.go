// Package pixelmatte is the on-demand AI background matting of the Pixel image
// editor: pinned ISNet and BiRefNet models, the image pre- and post-processing
// around them, the isolated `aurago --pixel-matte-worker run` process that
// runs ONNX Runtime, and the Manager the HTTP handlers use (downloads, status,
// one run at a time).
package pixelmatte

import (
	"fmt"
	goruntime "runtime"

	"aurago/internal/onnxrt"
)

// Kind selects a matting model. The values are the model ids of the HTTP API.
type Kind string

const (
	KindStandard Kind = "standard" // ISNet general use
	KindPrecise  Kind = "precise"  // BiRefNet general lite
)

// ValidKind reports whether s names a model.
func ValidKind(s string) bool {
	_, ok := models[Kind(s)]
	return ok
}

// InputSize is the side of the square model input and of the matte the models
// produce. Both pinned models take [1,3,1024,1024] float32.
const InputSize = 1024

// ModelSpec describes one pinned matting model and how rembg feeds it.
type ModelSpec struct {
	Kind    Kind
	Name    string
	License string
	Asset   onnxrt.AssetSpec
	Mean    [3]float32
	Std     [3]float32
	// Sigmoid applies a logistic function to the output before the min-max
	// normalisation (BiRefNet emits logits, ISNet a probability-like map).
	Sigmoid bool
}

// Both models are the rembg release v0.0.0 exports of their upstream
// projects: ISNet general use (DIS, Apache-2.0) and BiRefNet general with the
// Swin-tiny backbone (MIT). Pre-processing matches rembg: resize to 1024x1024,
// divide by the largest channel value of the resized image, then subtract the
// mean and divide by the standard deviation.
var models = map[Kind]ModelSpec{
	KindStandard: {
		Kind:    KindStandard,
		Name:    "ISNet general-use",
		License: "Apache-2.0",
		Asset: onnxrt.AssetSpec{
			ID:       "isnet-general-use",
			URL:      "https://github.com/danielgatis/rembg/releases/download/v0.0.0/isnet-general-use.onnx",
			Size:     178_648_008,
			SHA256:   "60920e99c45464f2ba57bee2ad08c919a52bbf852739e96947fbb4358c0d964a",
			Kind:     onnxrt.AssetFile,
			FileName: "isnet-general-use.onnx",
		},
		Mean: [3]float32{0.485, 0.456, 0.406},
		Std:  [3]float32{1, 1, 1},
	},
	KindPrecise: {
		Kind:    KindPrecise,
		Name:    "BiRefNet general lite",
		License: "MIT",
		Asset: onnxrt.AssetSpec{
			ID:       "birefnet-general-lite",
			URL:      "https://github.com/danielgatis/rembg/releases/download/v0.0.0/BiRefNet-general-bb_swin_v1_tiny-epoch_232.onnx",
			Size:     224_005_088,
			SHA256:   "5600024376f572a557870a5eb0afb1e5961636bef4e1e22132025467d0f03333",
			Kind:     onnxrt.AssetFile,
			FileName: "birefnet-general-lite.onnx",
		},
		Mean:    [3]float32{0.485, 0.456, 0.406},
		Std:     [3]float32{0.229, 0.224, 0.225},
		Sigmoid: true,
	},
}

// modelOrder is the order Status lists the models in.
var modelOrder = []Kind{KindStandard, KindPrecise}

// Model returns the pinned spec of a model.
func Model(kind Kind) (ModelSpec, bool) {
	spec, ok := models[kind]
	return spec, ok
}

// Models returns the pinned specs in display order.
func Models() []ModelSpec {
	specs := make([]ModelSpec, 0, len(modelOrder))
	for _, kind := range modelOrder {
		specs = append(specs, models[kind])
	}
	return specs
}

// runtimeAsset is the ONNX Runtime CPU bundle for a platform.
func runtimeAsset(goos, goarch string) (onnxrt.AssetSpec, bool) {
	return onnxrt.CPURuntimeAsset(goos, goarch)
}

// Available reports whether a pinned ONNX Runtime CPU bundle exists for this
// platform, the only precondition of the feature.
func Available() bool {
	_, ok := runtimeAsset(goruntime.GOOS, goruntime.GOARCH)
	return ok
}

// DefaultThreads is the intra-op thread count of a matting run:
// max(1, NumCPU/2), at most 8.
func DefaultThreads() int {
	return min(8, max(1, goruntime.NumCPU()/2))
}

// validateSpec checks a model spec the way the asset cache will.
func validateSpec(spec ModelSpec) error {
	if err := onnxrt.ValidateAssetSpec(spec.Asset); err != nil {
		return fmt.Errorf("model %s: %w", spec.Kind, err)
	}
	if spec.Asset.Kind != onnxrt.AssetFile {
		return fmt.Errorf("model %s: asset must be a plain file", spec.Kind)
	}
	for _, std := range spec.Std {
		if !(std > 0) {
			return fmt.Errorf("model %s: standard deviation must be positive", spec.Kind)
		}
	}
	return nil
}
