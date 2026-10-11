package persondetect

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"image"
	_ "image/jpeg"
	"testing"

	"aurago/internal/onnxrt"
)

func TestModelManifestPinsDFINEAssets(t *testing.T) {
	cases := []struct {
		tier          ModelTier
		name, id, url string
		sha           string
		size          int64
	}{
		{
			ModelStandard, "D-FINE-N", "dfine-n-coco",
			"https://huggingface.co/AnnotateIt/dfine-nano-coco-onnx/resolve/571d3971bfb47dc5ddd3746134dd464954bc9df0/model.onnx",
			"b0ec2e5793f3a3cbd07440d098645a0e102cd450fb92f4f35e713f36e35d152e", 15_434_935,
		},
		{
			ModelAccurate, "D-FINE-S", "dfine-s-coco",
			"https://huggingface.co/onnx-community/dfine_s_coco-ONNX/resolve/a3cf03147a9b86c78475139115c8ac142577352d/onnx/model.onnx",
			"cd8a49a945feda6d28c6304ae8ae85c2759ba1d78a5a83a22c5ce8db82ef7238", 41_535_197,
		},
	}
	for _, tc := range cases {
		spec, ok := Model(tc.tier)
		if !ok {
			t.Fatalf("Model(%s) missing", tc.tier)
		}
		if spec.Tier != tc.tier || spec.Name != tc.name || spec.Asset.ID != tc.id || spec.Asset.URL != tc.url ||
			spec.Asset.SHA256 != tc.sha || spec.Asset.Size != tc.size || spec.Asset.Kind != onnxrt.AssetFile ||
			spec.Asset.FileName != "model.onnx" {
			t.Fatalf("Model(%s) = %+v", tc.tier, spec)
		}
		if err := onnxrt.ValidateAssetSpec(spec.Asset); err != nil {
			t.Fatalf("Model(%s) asset: %v", tc.tier, err)
		}
		if spec.InputName != "pixel_values" || spec.LogitsName != "logits" || spec.BoxesName != "pred_boxes" ||
			spec.InputSize != 640 || spec.Queries != 300 || spec.Classes != 80 || spec.PersonClass != 0 {
			t.Fatalf("Model(%s) I/O = %+v", tc.tier, spec)
		}
	}
	if _, ok := Model("huge"); ok {
		t.Fatal("unknown model tier resolved")
	}
}

func TestValidModelTierAndBackend(t *testing.T) {
	for _, value := range []string{"standard", "accurate"} {
		if !ValidModelTier(value) {
			t.Errorf("ValidModelTier(%q) = false", value)
		}
	}
	for _, value := range []string{"", "Standard", "nano"} {
		if ValidModelTier(value) {
			t.Errorf("ValidModelTier(%q) = true", value)
		}
	}
	for _, value := range []string{"auto", "gpu", "cpu"} {
		if !ValidBackend(value) {
			t.Errorf("ValidBackend(%q) = false", value)
		}
	}
	for _, value := range []string{"", "cuda", "GPU"} {
		if ValidBackend(value) {
			t.Errorf("ValidBackend(%q) = true", value)
		}
	}
}

func TestReferenceImageIsPinned(t *testing.T) {
	data := ReferenceImage()
	sum := sha256.Sum256(data)
	if len(data) != 104_328 || hex.EncodeToString(sum[:]) != "26c05f7f2861024e9756683cea133a930b73b4498784d99e9212e5e26e64f1a2" {
		t.Fatalf("reference image: %d bytes, sha256 %x", len(data), sum)
	}
	data[0] ^= 0xff
	if again := ReferenceImage(); again[0] == data[0] {
		t.Fatal("ReferenceImage exposes the embedded bytes")
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(ReferenceImage()))
	if err != nil || format != "jpeg" || config.Width != 800 || config.Height != 533 {
		t.Fatalf("reference image = %s %dx%d, %v", format, config.Width, config.Height, err)
	}
}
