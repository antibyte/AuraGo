package server

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"aurago/internal/tools"
)

var cameraHLSSessionPattern = regexp.MustCompile(`(?:[?&]|&amp;)id=([a-zA-Z0-9]{8})(?:[^a-zA-Z0-9]|$)`)

func rememberGo2RTCHLS(manager *tools.Go2RTCManager, lease *tools.Go2RTCMediaLease, playlist []byte) error {
	if len(playlist) > 1<<20 {
		return fmt.Errorf("HLS playlist exceeds limit")
	}
	matches := cameraHLSSessionPattern.FindAllSubmatch(playlist, -1)
	if len(matches) == 0 {
		return fmt.Errorf("HLS playlist has no authorized session")
	}
	for _, match := range matches {
		if err := manager.RememberHLS(lease.StreamID, lease.Role, string(match[1]), lease.Revision); err != nil {
			return err
		}
	}
	return nil
}

func go2RTCSafeCodecFilter(value string) bool {
	if len(value) > 128 {
		return false
	}
	for _, c := range value {
		if !strings.ContainsRune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_.,-/", c) {
			return false
		}
	}
	return true
}

func serveGo2RTCProxySnapshot(s *Server, w http.ResponseWriter, r *http.Request, id, role string) {
	width, err := boundedQueryInt(r, "width", 0, 0, 1280)
	if err != nil {
		jsonError(w, err.Error(), 400)
		return
	}
	height, err := boundedQueryInt(r, "height", 0, 0, 720)
	if err != nil {
		jsonError(w, err.Error(), 400)
		return
	}
	rotate, err := boundedQueryInt(r, "rotate", 0, 0, 270)
	if err != nil {
		jsonError(w, err.Error(), 400)
		return
	}
	_, data, err := s.Go2RTC.SnapshotBytes(r.Context(), id, tools.Go2RTCSnapshotOptions{Width: width, Height: height, Rotate: rotate, SourceRole: role})
	if err != nil {
		jsonError(w, "Camera snapshot unavailable", 502)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Method != http.MethodHead {
		_, _ = w.Write(data)
	}
}
