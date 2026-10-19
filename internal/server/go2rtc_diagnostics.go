package server

import (
	"context"
	"net/http"
	"strings"
	"time"

	"aurago/internal/tools"
)

func handleGo2RTCDiagnostics(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", 405)
			return
		}
		if !sameOriginOrNoOrigin(r) {
			jsonError(w, "Request origin does not match server host", 403)
			return
		}
		manager := go2RTCManager(s, w)
		if manager == nil {
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/api/go2rtc/diagnostics/")
		if tools.ValidateGo2RTCStreamID(id) != nil {
			jsonError(w, "Invalid camera", 400)
			return
		}
		writeGo2RTCJSON(w, manager.Diagnose(r.Context(), id))
	}
}

func handleGo2RTCHardwareTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", 405)
			return
		}
		if !sameOriginOrNoOrigin(r) {
			jsonError(w, "Request origin does not match server host", 403)
			return
		}
		manager := go2RTCManager(s, w)
		if manager == nil {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		result, err := manager.CheckHardware(ctx)
		status := http.StatusOK
		if err != nil {
			status = http.StatusBadGateway
		}
		writeGo2RTCJSONStatus(w, status, result)
	}
}
