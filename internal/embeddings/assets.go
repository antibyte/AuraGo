package embeddings

import (
	"context"

	"aurago/internal/onnxrt"
)

// assetCache keeps the embeddings call sites stable while the verified
// download and extraction logic lives in internal/onnxrt. The cache root is
// <data>/embeddings, shared with person detection's runtimes.
type assetCache struct {
	*onnxrt.AssetCache
}

func newAssetCache(root string, onProgress func(DownloadStatus)) *assetCache {
	return &assetCache{AssetCache: onnxrt.NewAssetCache(root, "AuraGo/"+LocalGraniteProvider, onProgress)}
}

func (c *assetCache) ensureDirectAsset(ctx context.Context, spec assetSpec, destination string) error {
	return c.EnsureFile(ctx, spec, destination)
}

func (c *assetCache) ensureRuntimeAsset(ctx context.Context, spec assetSpec) (string, error) {
	return c.EnsureRuntime(ctx, spec)
}
