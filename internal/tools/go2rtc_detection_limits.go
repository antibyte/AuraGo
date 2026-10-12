package tools

import "aurago/internal/config"

const (
	go2RTCBaseNanoCPUs          int64 = 500_000_000
	go2RTCBaseMemoryBytes       int64 = 256 << 20
	go2RTCDetectionNanoCPUsStep int64 = 250_000_000
	go2RTCDetectionMaxNanoCPUs  int64 = 2_000_000_000
	go2RTCDetectionMemoryBytes  int64 = 512 << 20
)

// Go2RTCContainerResources are the CPU and memory limits of the managed sidecar.
type Go2RTCContainerResources struct {
	NanoCPUs         int64
	MemoryBytes      int64
	DetectingCameras int
}

// SameLimits reports whether two resource sets give the container the same limits.
func (r Go2RTCContainerResources) SameLimits(other Go2RTCContainerResources) bool {
	return r.NanoCPUs == other.NanoCPUs && r.MemoryBytes == other.MemoryBytes
}

// Go2RTCDetectingCameras counts the cameras whose frames person detection pulls
// from go2rtc (each frame.jpeg is transcoded by ffmpeg inside the sidecar).
func Go2RTCDetectingCameras(cfg config.Go2RTCConfig) int {
	if !cfg.Enabled || !cfg.Detection.Enabled {
		return 0
	}
	count := 0
	for _, stream := range cfg.Streams {
		if stream.Enabled && stream.Detection.Enabled {
			count++
		}
	}
	return count
}

// Go2RTCResourcesFor returns the sidecar limits: the hardened base (0.5 CPU,
// 256 MB), or while person detection runs 0.5 + 0.25 CPU per detecting camera
// (at most 2 CPUs) and 512 MB.
func Go2RTCResourcesFor(cfg config.Go2RTCConfig) Go2RTCContainerResources {
	count := Go2RTCDetectingCameras(cfg)
	if count == 0 {
		return Go2RTCContainerResources{NanoCPUs: go2RTCBaseNanoCPUs, MemoryBytes: go2RTCBaseMemoryBytes}
	}
	nano := go2RTCBaseNanoCPUs + int64(count)*go2RTCDetectionNanoCPUsStep
	if nano > go2RTCDetectionMaxNanoCPUs {
		nano = go2RTCDetectionMaxNanoCPUs
	}
	return Go2RTCContainerResources{NanoCPUs: nano, MemoryBytes: go2RTCDetectionMemoryBytes, DetectingCameras: count}
}
