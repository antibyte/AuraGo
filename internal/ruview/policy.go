package ruview

import (
	"net/http"
	"path"
	"regexp"
	"strings"
)

var resourceID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$`)

// IsStop permits only termination of existing work while RuView is read-only.
// These remain administrative mutations requiring the usual origin checks.
func IsStop(method, p string) bool {
	return method == http.MethodPost && (p == "runtime/stop" || p == "/api/v1/recording/stop" || p == "/api/v1/train/stop")
}

// Endpoint permits only the reviewed v3122 data/control API. In particular,
// model load/unload and LoRA activation are upstream status-only stubs.
func Endpoint(method, p string) (allowed, write, stream bool) {
	if strings.ContainsAny(p, "%\\\x00") || path.Clean(p) != p {
		return
	}
	if method == http.MethodGet {
		for _, v := range []string{"/ws/sensing", "/ws/introspection", "/ws/field", "/ws/train/progress", "/api/v1/stream/pose"} {
			if p == v {
				return true, false, true
			}
		}
		for _, v := range []string{"/health", "/health/health", "/health/live", "/health/ready", "/health/metrics", "/health/version", "/api/v1/info", "/api/v1/status", "/api/v1/metrics", "/api/v1/sensing/latest", "/api/v1/radar/latest", "/api/v1/nodes", "/api/v1/mesh", "/api/v1/mesh/metrics", "/api/v1/vital-signs", "/api/v1/edge-vitals", "/api/v1/edge/registry", "/api/v1/wasm-events", "/api/v1/model/info", "/api/v1/model/layers", "/api/v1/model/segments", "/api/v1/model/sona/profiles", "/api/v1/pose/current", "/api/v1/pose/physics/metrics", "/api/v1/pose/activities", "/api/v1/pose/calibration/status", "/api/v1/stream/status", "/api/v1/introspection/snapshot", "/api/v1/models", "/api/v1/models/active", "/api/v1/models/lora/profiles", "/api/v1/recording/list", "/api/v1/calibration/status", "/api/v1/adaptive/status", "/api/v1/config/dedup-factor", "/api/v1/train/status", "/api/field"} {
			if p == v {
				return true, false, false
			}
		}
		parts := strings.Split(strings.TrimPrefix(p, "/api/v1/nodes/"), "/")
		if strings.HasPrefix(p, "/api/v1/nodes/") && len(parts) == 2 && resourceID.MatchString(parts[0]) && parts[1] == "sync" {
			return true, false, false
		}
	}
	if method == http.MethodPost {
		for _, v := range []string{"/api/v1/recording/start", "/api/v1/recording/stop", "/api/v1/pose/calibrate", "/api/v1/calibration/start", "/api/v1/calibration/stop", "/api/v1/calibration/cancel", "/api/v1/calibration/reset", "/api/v1/calibration/bootstrap/promote", "/api/v1/calibration/bootstrap/refine", "/api/v1/adaptive/train", "/api/v1/adaptive/unload", "/api/v1/config/dedup-factor", "/api/v1/config/ground-truth", "/api/v1/train/start", "/api/v1/train/stop", "/api/v1/train/pretrain", "/api/v1/train/lora"} {
			if p == v {
				return true, true, false
			}
		}
	}
	if method == http.MethodDelete {
		for _, prefix := range []string{"/api/v1/recording/", "/api/v1/models/"} {
			if strings.HasPrefix(p, prefix) && resourceID.MatchString(strings.TrimPrefix(p, prefix)) {
				return true, true, false
			}
		}
	}
	return
}

var readOperations = map[string]string{"nodes": "/api/v1/nodes", "sensing": "/api/v1/sensing/latest", "model_info": "/api/v1/model/info", "recordings": "/api/v1/recording/list"}
