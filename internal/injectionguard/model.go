// Package injectionguard owns the optional, local Horizon prompt-injection classifier.
package injectionguard

import "aurago/internal/onnxrt"

const (
	Revision      = "0e3774536debbcd71dbb8534ef71451b583853b8"
	ModelName     = "Horizon-Labs/prompt-injection-guard-small"
	Threshold     = 0.5
	MaxTextBytes  = 256 << 10
	WindowTokens  = 2048
	WindowOverlap = 512
	assetBase     = "https://huggingface.co/" + ModelName + "/resolve/" + Revision + "/"
)

var ModelAsset = onnxrt.AssetSpec{
	ID: "horizon-injection-guard-small-0e377453", URL: assetBase + "onnx/model_quantized.onnx",
	Size: 268409234, SHA256: "61cd0741cdbfcc229e32e1de1392f352736d87d1e8b3c2b67ad9836bb7910d5d",
	Kind: onnxrt.AssetFile, FileName: "model.onnx",
}

var TokenizerAsset = onnxrt.AssetSpec{
	ID: "horizon-injection-tokenizer-0e377453", URL: assetBase + "tokenizer.json",
	Size: 34387555, SHA256: "47834a7dbbb0c4324fe9569fe47955241594a9ee539c9e478755d6fbc51e5f46",
	Kind: onnxrt.AssetFile, FileName: "tokenizer.json",
}
