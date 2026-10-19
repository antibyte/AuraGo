package tools

import (
	"encoding/json"
	"net/http"

	"aurago/internal/config"
)

const (
	go2RTCNanoCPUsPerCPU        int64 = 1_000_000_000
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

// go2RTCDockerNCPU returns the number of CPUs the Docker daemon reports, or 0
// when it is unknown (the request failed, a socket proxy denies /info, or the
// answer carries no count). Callers treat 0 as "do not clamp".
func go2RTCDockerNCPU(dockerCfg DockerConfig) int {
	data, code, err := dockerRequest(dockerCfg, http.MethodGet, "/info", "")
	if err != nil || code != http.StatusOK {
		return 0
	}
	var info struct {
		NCPU int `json:"NCPU"`
	}
	if json.Unmarshal(data, &info) != nil || info.NCPU <= 0 {
		return 0
	}
	return info.NCPU
}

// go2RTCHostCPUsFor returns the Docker host's CPU count when the limits could
// exceed it, otherwise 0. Docker rejects NanoCpus above the daemon's CPU count at
// create time, but every daemon has at least one CPU, so limits up to one CPU
// need no lookup.
func go2RTCHostCPUsFor(dockerCfg DockerConfig, resources Go2RTCContainerResources) int {
	if resources.NanoCPUs <= go2RTCNanoCPUsPerCPU {
		return 0
	}
	return go2RTCDockerNCPU(dockerCfg)
}

// go2RTCClampNanoCPUs caps a CPU limit at the host's CPU count; hostCPUs <= 0
// means the count is unknown and the limit stays as configured.
func go2RTCClampNanoCPUs(nanoCPUs int64, hostCPUs int) int64 {
	if hostCPUs <= 0 {
		return nanoCPUs
	}
	if limit := int64(hostCPUs) * go2RTCNanoCPUsPerCPU; nanoCPUs > limit {
		return limit
	}
	return nanoCPUs
}

// Go2RTCResourcesFor returns the sidecar limits: the hardened base (0.5 CPU,
// 256 MB), or while person detection runs 0.5 + 0.25 CPU per detecting camera
// (at most 2 CPUs) and 512 MB.
func Go2RTCResourcesFor(cfg config.Go2RTCConfig) Go2RTCContainerResources {
	for _, stream := range cfg.Streams {
		if cfg.Enabled && stream.Enabled && stream.CodecProfile != "" && stream.CodecProfile != "original" {
			return Go2RTCContainerResources{NanoCPUs: 2_000_000_000, MemoryBytes: 1 << 30, DetectingCameras: Go2RTCDetectingCameras(cfg)}
		}
	}
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
