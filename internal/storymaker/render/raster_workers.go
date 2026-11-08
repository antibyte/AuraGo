package render

import "aurago/internal/storymaker"

// BoundRasterWorkers reserves encoded assets, a bounded decoded working set and browser overhead per worker.
func BoundRasterWorkers(proposed int, freeMB int64, files []storymaker.CatalogFile) int {
	if proposed < 1 || freeMB <= 0 {
		return 1
	}
	var encoded int64
	for _, file := range files {
		if file.Type != "audio/wav" {
			encoded += file.Bytes
		}
	}
	// Decoded bitmaps and GPU copies can coexist; leave headroom for Go and FFmpeg.
	perWorker := int64(1280) + (encoded+(1<<20)-1)/(1<<20)
	bound := int((freeMB - 768) / perWorker)
	if bound < 1 {
		return 1
	}
	if bound < proposed {
		return bound
	}
	return proposed
}
