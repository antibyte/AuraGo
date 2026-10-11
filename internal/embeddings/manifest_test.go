package embeddings

import (
	"strings"
	"testing"

	"aurago/internal/onnxrt"
)

// Every pinned embeddings download goes through onnxrt's asset cache, which
// refuses specs that fail onnxrt.ValidateAssetSpec. The ONNX Runtime bundles
// live in onnxrt's runtime manifest and are guarded there.
func TestManifestAssetsPassOnnxrtValidation(t *testing.T) {
	for _, spec := range []assetSpec{graniteONNXModelAsset, graniteTokenizerAsset, graniteGGUFModelAsset} {
		if err := onnxrt.ValidateAssetSpec(spec); err != nil {
			t.Errorf("model asset %s: %v", spec.ID, err)
		}
	}
	for id, spec := range runtimeManifest {
		if spec.ID != id {
			t.Errorf("runtime manifest key %s holds ID %s", id, spec.ID)
		}
		if strings.HasPrefix(id, "ort-") {
			t.Errorf("runtime manifest duplicates the onnxrt runtime %s", id)
		}
		if spec.Kind != assetZip && spec.Kind != assetTarGzip {
			t.Errorf("runtime asset %s has archive kind %q", id, spec.Kind)
		}
		if err := onnxrt.ValidateAssetSpec(spec); err != nil {
			t.Errorf("runtime asset %s: %v", id, err)
		}
	}
}
