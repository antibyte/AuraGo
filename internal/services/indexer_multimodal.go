package services

import (
	"context"
	"fmt"
)

func (fi *FileIndexer) indexPDFWithMultimodalFallback(path, relPath, fileName string) (string, []float32, error) {
	embedder := fi.getMultimodalEmbedder()
	if embedder == nil {
		return "", nil, fmt.Errorf("multimodal embedder unavailable")
	}
	vec, err := fi.indexEmbedWithRetry(func() ([]float32, error) {
		return embedder.EmbedFile(context.Background(), path)
	}, path, "pdf-fallback")
	if err != nil {
		return "", nil, err
	}
	return fmt.Sprintf("PDF (gescannt): %s (Pfad: %s)", fileName, relPath), vec, nil
}
