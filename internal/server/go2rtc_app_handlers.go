package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"aurago/internal/config"
	"aurago/internal/onvif"
	"aurago/internal/security"
	"aurago/internal/tools"
)

const go2RTCAppBodyLimit = 64 << 10

type go2RTCMutationError struct {
	Status  int
	Code    string
	Message string
}

func (e *go2RTCMutationError) Error() string {
	return e.Message
}

func writeGo2RTCMutationFailure(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	code := "save_failed"
	message := "The camera configuration could not be saved"
	var mutationErr *go2RTCMutationError
	if errors.As(err, &mutationErr) {
		status = mutationErr.Status
		code = mutationErr.Code
		message = mutationErr.Message
	}
	writeGo2RTCJSONStatus(w, status, map[string]interface{}{
		"status": "error", "code": code, "message": security.Scrub(message), "saved": false,
	})
}

func writeGo2RTCMutationResult(w http.ResponseWriter, successStatus int, result go2RTCConfigUpdateResult, payload map[string]interface{}) {
	payload["saved"] = result.Published
	payload["runtime_reconciled"] = result.ReconcileErr == nil
	if result.ReconcileErr != nil {
		payload["status"] = "degraded"
		payload["message"] = security.Scrub(result.ReconcileErr.Error())
		writeGo2RTCJSONStatus(w, http.StatusAccepted, payload)
		return
	}
	payload["status"] = "ok"
	writeGo2RTCJSONStatus(w, successStatus, payload)
}

func handleGo2RTCStreamsRoute(s *Server) http.HandlerFunc {
	view := requireGo2RTCView(s, handleGo2RTCStreams(s))
	create := requireAdmin(s, handleGo2RTCCreateStream(s)).ServeHTTP
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			view(w, r)
		case http.MethodPost:
			create(w, r)
		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func handleGo2RTCAppState(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		manager := go2RTCManager(s, w)
		if manager == nil {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		status := manager.Status(ctx)
		cfg := manager.Config()
		streams := configuredGo2RTCStreamStatus(cfg.Streams)
		if status.APIUsable {
			if live, err := manager.ListStreams(ctx); err == nil {
				streams = live
			}
		}
		available, reason := false, "ONVIF discovery is unavailable"
		if s.Go2RTCDiscovery != nil {
			available, reason = s.Go2RTCDiscovery.DiscoveryAvailable()
		}
		canManage := go2RTCRequestIsAdmin(s, r)
		reasonCode := ""
		if !available {
			reasonCode = "broadcast_unavailable"
		}
		response := map[string]interface{}{
			"status":            "ok",
			"integration":       status,
			"webrtc_enabled":    cfg.WebRTC.Enabled,
			"can_manage":        canManage,
			"discovery":         map[string]interface{}{"available": available, "reason": reason, "reason_code": reasonCode},
			"streams":           streams,
			"web_ui_enabled":    cfg.WebUIEnabled,
			"thumbnail_refresh": 5,
		}
		if canManage {
			disabled := make([]map[string]interface{}, 0)
			for _, stream := range cfg.Streams {
				if stream.Enabled {
					continue
				}
				disabled = append(disabled, map[string]interface{}{
					"id": stream.ID, "name": stream.Name, "enabled": false,
					"source_configured": stream.SourceConfigured || strings.TrimSpace(stream.Source) != "",
				})
			}
			response["disabled_streams"] = disabled
		}
		writeGo2RTCJSON(w, response)
	}
}

func configuredGo2RTCStreamStatus(streams []config.Go2RTCStreamConfig) []tools.Go2RTCStreamStatus {
	result := make([]tools.Go2RTCStreamStatus, 0, len(streams))
	for _, stream := range streams {
		if stream.Enabled {
			result = append(result, tools.Go2RTCStreamStatus{ID: stream.ID, Name: stream.Name, Codecs: []string{}})
		}
	}
	return result
}

func handleGo2RTCThumbnail(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		manager := go2RTCManager(s, w)
		if manager == nil {
			return
		}
		rawID := strings.TrimPrefix(r.URL.Path, "/api/go2rtc/thumbnail/")
		if !strings.HasSuffix(rawID, ".jpg") {
			http.NotFound(w, r)
			return
		}
		streamID, err := url.PathUnescape(strings.TrimSuffix(rawID, ".jpg"))
		if err != nil || tools.ValidateGo2RTCStreamID(streamID) != nil {
			jsonError(w, "Invalid stream ID", http.StatusBadRequest)
			return
		}
		width, err := boundedQueryInt(r, "width", 640, 1, 1280)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		height, err := boundedQueryInt(r, "height", 360, 1, 720)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		cacheSeconds, err := boundedQueryInt(r, "cache", 5, 0, 30)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		result, data, err := manager.SnapshotBytes(ctx, streamID, tools.Go2RTCSnapshotOptions{Width: width, Height: height, CacheSeconds: cacheSeconds})
		if err != nil {
			if s.Logger != nil {
				s.Logger.Warn("[go2rtc] Thumbnail unavailable", "stream_id", streamID, "error", security.Scrub(err.Error()))
			}
			jsonError(w, "Camera thumbnail is unavailable", http.StatusBadGateway)
			return
		}
		etag := `"` + result.SHA256 + `"`
		w.Header().Set("ETag", etag)
		w.Header().Set("Cache-Control", fmt.Sprintf("private, max-age=%d", cacheSeconds))
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		if r.Method == http.MethodHead {
			return
		}
		_, _ = w.Write(data)
	}
}

func boundedQueryInt(r *http.Request, key string, fallback, minimum, maximum int) (int, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < minimum || value > maximum {
		return 0, fmt.Errorf("%s must be between %d and %d", key, minimum, maximum)
	}
	return value, nil
}

func handleGo2RTCSetupEnable(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !sameOriginOrNoOrigin(r) {
			jsonError(w, "Request origin does not match server host", http.StatusForbidden)
			return
		}
		requirements := go2RTCDockerRequirements(r.Context(), s)
		if len(requirements) > 0 {
			writeGo2RTCJSONStatus(w, http.StatusPreconditionFailed, map[string]interface{}{
				"status": "requirements_missing", "saved": false, "requirements": requirements,
			})
			return
		}
		result, err := updateManagedGo2RTCConfig(r.Context(), s, func(value *config.Go2RTCConfig) error {
			value.Enabled = true
			return nil
		}, nil, true)
		if err != nil {
			writeGo2RTCMutationFailure(w, err)
			return
		}
		writeGo2RTCMutationResult(w, http.StatusOK, result, map[string]interface{}{"enabled": result.Config.Enabled})
	}
}

func go2RTCDockerRequirements(ctx context.Context, s *Server) []map[string]string {
	if s == nil || s.Cfg == nil {
		return []map[string]string{{"code": "config_unavailable", "message": "AuraGo configuration is unavailable"}}
	}
	s.CfgMu.RLock()
	cfg := *s.Cfg
	s.CfgMu.RUnlock()
	result := make([]map[string]string, 0, 3)
	if !cfg.Docker.Enabled {
		result = append(result, map[string]string{"code": "docker_disabled", "message": "Enable the Docker integration first"})
	}
	if cfg.Docker.ReadOnly {
		result = append(result, map[string]string{"code": "docker_read_only", "message": "Disable Docker read-only mode so AuraGo can manage the sidecar"})
	}
	if strings.TrimSpace(cfg.Docker.Host) == "" && !cfg.Runtime.DockerSocketOK {
		result = append(result, map[string]string{"code": "docker_unreachable", "message": "Configure a reachable Docker endpoint or start the Docker socket proxy"})
	}
	if len(result) == 0 && s.Go2RTC == nil {
		result = append(result, map[string]string{"code": "docker_unreachable", "message": "The Docker manager is unavailable"})
	}
	if len(result) == 0 {
		probeCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
		defer cancel()
		for _, requirement := range s.Go2RTC.DockerAccessRequirements(probeCtx) {
			result = append(result, map[string]string{"code": requirement.Code, "message": requirement.Message})
		}
	}
	return result
}

func handleGo2RTCDiscovery(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !sameOriginOrNoOrigin(r) {
			jsonError(w, "Request origin does not match server host", http.StatusForbidden)
			return
		}
		service := go2RTCDiscoveryService(s)
		if service == nil {
			jsonError(w, "ONVIF discovery is unavailable", http.StatusServiceUnavailable)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		candidates, err := service.Discover(ctx)
		if err != nil {
			writeGo2RTCJSONStatus(w, http.StatusServiceUnavailable, map[string]interface{}{"status": "error", "message": security.Scrub(err.Error())})
			return
		}
		writeGo2RTCJSON(w, map[string]interface{}{"status": "ok", "candidates": candidates})
	}
}

func handleGo2RTCDiscoveryProfiles(s *Server) http.HandlerFunc {
	type request struct {
		CandidateID string `json:"candidate_id"`
		Address     string `json:"address"`
		Username    string `json:"username"`
		Password    string `json:"password"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !sameOriginOrNoOrigin(r) {
			jsonError(w, "Request origin does not match server host", http.StatusForbidden)
			return
		}
		var payload request
		if err := decodeGo2RTCAppJSON(w, r, &payload); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		security.RegisterSensitive(payload.Username)
		security.RegisterSensitive(payload.Password)
		service := go2RTCDiscoveryService(s)
		if service == nil {
			jsonError(w, "ONVIF setup is unavailable", http.StatusServiceUnavailable)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		result, err := service.Profiles(ctx, onvif.ProfileRequest{
			CandidateID: payload.CandidateID, Address: payload.Address,
			Username: payload.Username, Password: payload.Password,
		})
		if err != nil {
			writeGo2RTCJSONStatus(w, http.StatusBadGateway, map[string]interface{}{"status": "error", "message": security.Scrub(err.Error())})
			return
		}
		writeGo2RTCJSON(w, result)
	}
}

func go2RTCDiscoveryService(s *Server) *onvif.Service {
	if s == nil {
		return nil
	}
	return s.Go2RTCDiscovery
}

func handleGo2RTCCreateStream(s *Server) http.HandlerFunc {
	type request struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		Enabled    *bool  `json:"enabled"`
		Source     string `json:"source"`
		SetupToken string `json:"setup_token"`
		ProfileID  string `json:"profile_id"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !sameOriginOrNoOrigin(r) {
			jsonError(w, "Request origin does not match server host", http.StatusForbidden)
			return
		}
		var payload request
		if err := decodeGo2RTCAppJSON(w, r, &payload); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		id := strings.TrimSpace(strings.ToLower(payload.ID))
		if err := tools.ValidateGo2RTCStreamID(id); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		name, err := validateCameraName(payload.Name, id)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		source := strings.TrimSpace(payload.Source)
		var reservation *onvif.SetupReservation
		if strings.TrimSpace(payload.SetupToken) != "" {
			if source != "" {
				jsonError(w, "Use either a setup token or a manual source", http.StatusBadRequest)
				return
			}
			service := go2RTCDiscoveryService(s)
			if service == nil {
				writeGo2RTCJSONStatus(w, http.StatusPreconditionFailed, map[string]interface{}{
					"status": "error", "code": "onvif_unavailable", "message": "ONVIF setup is unavailable", "saved": false,
				})
				return
			}
			reservation, err = service.Reserve(payload.SetupToken, payload.ProfileID)
			if err != nil {
				jsonError(w, security.Scrub(err.Error()), http.StatusBadRequest)
				return
			}
			defer reservation.Release()
			source = reservation.Source()
		}
		security.RegisterSensitive(source)
		if err := tools.ValidateGo2RTCSource(source); err != nil {
			jsonError(w, "A supported network stream URL or ONVIF profile is required", http.StatusBadRequest)
			return
		}
		enabled := true
		if payload.Enabled != nil {
			enabled = *payload.Enabled
		}
		result, err := updateManagedGo2RTCConfig(r.Context(), s, func(value *config.Go2RTCConfig) error {
			if !value.Enabled {
				return &go2RTCMutationError{Status: http.StatusPreconditionFailed, Code: "go2rtc_disabled", Message: "Enable go2rtc before adding cameras"}
			}
			for _, stream := range value.Streams {
				if stream.ID == id {
					return &go2RTCMutationError{Status: http.StatusConflict, Code: "stream_exists", Message: fmt.Sprintf("stream id %q already exists", id)}
				}
			}
			value.Streams = append(value.Streams, config.Go2RTCStreamConfig{ID: id, Name: name, Enabled: enabled})
			return nil
		}, []go2RTCSourceChange{{ID: id, Value: source}}, false)
		if err != nil {
			writeGo2RTCMutationFailure(w, err)
			return
		}
		if reservation != nil {
			reservation.Commit()
		}
		writeGo2RTCMutationResult(w, http.StatusCreated, result, map[string]interface{}{"stream": safeGo2RTCStream(result.Config, id)})
	}
}

func handleGo2RTCStreamMutation(s *Server) http.HandlerFunc {
	type request struct {
		Name    *string `json:"name"`
		Enabled *bool   `json:"enabled"`
		Source  *string `json:"source"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch && r.Method != http.MethodDelete {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !sameOriginOrNoOrigin(r) {
			jsonError(w, "Request origin does not match server host", http.StatusForbidden)
			return
		}
		streamID, err := url.PathUnescape(strings.TrimPrefix(r.URL.Path, "/api/go2rtc/streams/"))
		if err != nil || tools.ValidateGo2RTCStreamID(streamID) != nil {
			jsonError(w, "Invalid stream ID", http.StatusBadRequest)
			return
		}
		if r.Method == http.MethodDelete {
			result, err := updateManagedGo2RTCConfig(r.Context(), s, func(value *config.Go2RTCConfig) error {
				if !value.Enabled {
					return &go2RTCMutationError{Status: http.StatusPreconditionFailed, Code: "go2rtc_disabled", Message: "Enable go2rtc before managing cameras"}
				}
				for index, stream := range value.Streams {
					if stream.ID == streamID {
						value.Streams = append(value.Streams[:index], value.Streams[index+1:]...)
						return nil
					}
				}
				return &go2RTCMutationError{Status: http.StatusNotFound, Code: "stream_not_found", Message: fmt.Sprintf("stream %q was not found", streamID)}
			}, []go2RTCSourceChange{{ID: streamID, Delete: true}}, false)
			if err != nil {
				writeGo2RTCMutationFailure(w, err)
				return
			}
			writeGo2RTCMutationResult(w, http.StatusOK, result, map[string]interface{}{"streams": len(result.Config.Streams)})
			return
		}

		var payload request
		if err := decodeGo2RTCAppJSON(w, r, &payload); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		if payload.Name == nil && payload.Enabled == nil && payload.Source == nil {
			jsonError(w, "At least one stream field is required", http.StatusBadRequest)
			return
		}
		var changes []go2RTCSourceChange
		if payload.Source != nil && strings.TrimSpace(*payload.Source) != "" {
			source := strings.TrimSpace(*payload.Source)
			security.RegisterSensitive(source)
			if err := tools.ValidateGo2RTCSource(source); err != nil {
				jsonError(w, "The replacement source is not a supported network URL", http.StatusBadRequest)
				return
			}
			changes = append(changes, go2RTCSourceChange{ID: streamID, Value: source})
		}
		result, err := updateManagedGo2RTCConfig(r.Context(), s, func(value *config.Go2RTCConfig) error {
			if !value.Enabled {
				return &go2RTCMutationError{Status: http.StatusPreconditionFailed, Code: "go2rtc_disabled", Message: "Enable go2rtc before managing cameras"}
			}
			for index := range value.Streams {
				if value.Streams[index].ID != streamID {
					continue
				}
				if payload.Name != nil {
					name, err := validateCameraName(*payload.Name, streamID)
					if err != nil {
						return &go2RTCMutationError{Status: http.StatusBadRequest, Code: "invalid_stream", Message: err.Error()}
					}
					value.Streams[index].Name = name
				}
				if payload.Enabled != nil {
					value.Streams[index].Enabled = *payload.Enabled
				}
				return nil
			}
			return &go2RTCMutationError{Status: http.StatusNotFound, Code: "stream_not_found", Message: fmt.Sprintf("stream %q was not found", streamID)}
		}, changes, false)
		if err != nil {
			writeGo2RTCMutationFailure(w, err)
			return
		}
		writeGo2RTCMutationResult(w, http.StatusOK, result, map[string]interface{}{"stream": safeGo2RTCStream(result.Config, streamID)})
	}
}

func safeGo2RTCStream(cfg config.Go2RTCConfig, streamID string) map[string]interface{} {
	for _, stream := range cfg.Streams {
		if stream.ID == streamID {
			return map[string]interface{}{
				"id": stream.ID, "name": stream.Name, "enabled": stream.Enabled,
				"source_configured": stream.SourceConfigured || strings.TrimSpace(stream.Source) != "",
			}
		}
	}
	return nil
}

func validateCameraName(raw, fallback string) (string, error) {
	value := strings.Join(strings.Fields(strings.TrimSpace(raw)), " ")
	if value == "" {
		value = fallback
	}
	if len([]rune(value)) > 96 {
		return "", fmt.Errorf("camera name is too long")
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return "", fmt.Errorf("camera name contains unsupported characters")
		}
	}
	return value, nil
}

func decodeGo2RTCAppJSON(w http.ResponseWriter, r *http.Request, target interface{}) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, go2RTCAppBodyLimit))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid JSON request")
	}
	var extra interface{}
	if err := decoder.Decode(&extra); err != io.EOF {
		return fmt.Errorf("request must contain one JSON object")
	}
	return nil
}
