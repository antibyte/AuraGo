package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/persondetect"
	"aurago/internal/security"
	"aurago/internal/tools"
)

// go2RTCDetectionPrefix is the base of the person-detection API (contract C6).
const go2RTCDetectionPrefix = "/api/go2rtc/detection/"

func writeGo2RTCDetectionJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeGo2RTCDetectionError(w http.ResponseWriter, status int, code, message string) {
	writeGo2RTCDetectionJSON(w, status, map[string]string{"error": message, "code": code})
}

// writeGo2RTCDetectionFieldError answers 400 invalid_request naming the offending field.
func writeGo2RTCDetectionFieldError(w http.ResponseWriter, err error) {
	body := map[string]string{"error": err.Error(), "code": "invalid_request"}
	var fieldErr *config.Go2RTCDetectionFieldError
	if errors.As(err, &fieldErr) {
		body["error"], body["field"] = fieldErr.Message, fieldErr.Field
	}
	writeGo2RTCDetectionJSON(w, http.StatusBadRequest, body)
}

// go2RTCDetectionNotFound names the 404 answer of a lookup (event or frame).
type go2RTCDetectionNotFound struct{ code, message string }

var (
	go2RTCDetectionEventNotFound = go2RTCDetectionNotFound{"event_not_found", "Event not found"}
	go2RTCDetectionFrameNotFound = go2RTCDetectionNotFound{"frame_not_found", "Frame not found"}
)

// writeGo2RTCDetectionServiceError maps a service error: ErrUnavailable (enabled,
// but detections.db is not open) is 503 detection_unavailable, ErrDisabled is 409
// detection_disabled, ErrUnknownEvent is the lookup's 404, anything else is a
// logged 500 detection_failed.
func writeGo2RTCDetectionServiceError(s *Server, w http.ResponseWriter, err error, notFound go2RTCDetectionNotFound, failure string) {
	switch {
	case errors.Is(err, persondetect.ErrUnavailable):
		writeGo2RTCDetectionError(w, http.StatusServiceUnavailable, "detection_unavailable", "Person detection is unavailable")
	case errors.Is(err, persondetect.ErrDisabled):
		writeGo2RTCDetectionError(w, http.StatusConflict, "detection_disabled", "Person detection is switched off")
	case notFound.code != "" && errors.Is(err, persondetect.ErrUnknownEvent):
		writeGo2RTCDetectionError(w, http.StatusNotFound, notFound.code, notFound.message)
	default:
		go2RTCDetectionLogger(s).Warn("[PersonDetect] API request failed", "error", security.Scrub(err.Error()))
		writeGo2RTCDetectionError(w, http.StatusInternalServerError, "detection_failed", failure)
	}
}

func go2RTCDetectionLogger(s *Server) *slog.Logger {
	if s != nil && s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

// go2RTCDetectionPath splits /api/go2rtc/detection/<route>[/<id>].
func go2RTCDetectionPath(urlPath string) (string, string) {
	rel := strings.TrimPrefix(urlPath, go2RTCDetectionPrefix)
	route, id, _ := strings.Cut(rel, "/")
	if strings.Contains(id, "/") {
		return "", ""
	}
	return route, id
}

// go2RTCDetectionAllow lists the methods of a detection route ("" = no such route).
func go2RTCDetectionAllow(route, id string) string {
	switch {
	case route == "state" && id == "", route == "events" && id == "":
		return "GET"
	case route == "events":
		return "GET, DELETE"
	case route == "images" && id != "":
		return "GET"
	case route == "arm" && id == "", route == "test" && id == "", route == "retry" && id == "":
		return "POST"
	case route == "settings" && id == "":
		return "PUT"
	default:
		return ""
	}
}

// handleGo2RTCDetection serves /api/go2rtc/detection/. Reads need the go2rtc.view
// scope (or admin); writes need admin and a same-origin or origin-less request.
func handleGo2RTCDetection(s *Server) http.HandlerFunc {
	read := requireGo2RTCView(s, func(w http.ResponseWriter, r *http.Request) {
		serveGo2RTCDetectionRead(s, w, r)
	})
	return func(w http.ResponseWriter, r *http.Request) {
		route, id := go2RTCDetectionPath(r.URL.Path)
		allow := go2RTCDetectionAllow(route, id)
		if allow == "" {
			writeGo2RTCDetectionError(w, http.StatusNotFound, "not_found", "Not found")
			return
		}
		method := r.Method
		if method == http.MethodHead {
			method = http.MethodGet
		}
		if !strings.Contains(", "+allow+",", ", "+method+",") {
			w.Header().Set("Allow", allow)
			writeGo2RTCDetectionError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
			return
		}
		if method == http.MethodGet {
			read(w, r)
			return
		}
		if !go2RTCRequestIsAdmin(s, r) || !sameOriginOrNoOrigin(r) {
			writeGo2RTCDetectionError(w, http.StatusForbidden, "forbidden", "Admin access from this origin is required")
			return
		}
		serveGo2RTCDetectionWrite(s, w, r, route, id)
	}
}

func go2RTCDetectionService(s *Server, w http.ResponseWriter) *persondetect.Service {
	if s == nil || s.PersonDetect == nil {
		writeGo2RTCDetectionError(w, http.StatusServiceUnavailable, "detection_unavailable", "Person detection is unavailable")
		return nil
	}
	return s.PersonDetect
}

func serveGo2RTCDetectionRead(s *Server, w http.ResponseWriter, r *http.Request) {
	svc := go2RTCDetectionService(s, w)
	if svc == nil {
		return
	}
	route, id := go2RTCDetectionPath(r.URL.Path)
	switch {
	case route == "state":
		serveGo2RTCDetectionState(s, svc, w, r)
	case route == "events" && id == "":
		serveGo2RTCDetectionEvents(s, svc, w, r)
	case route == "events":
		serveGo2RTCDetectionEvent(s, svc, w, r, id)
	default:
		serveGo2RTCDetectionImage(s, svc, w, r, id)
	}
}

func serveGo2RTCDetectionWrite(s *Server, w http.ResponseWriter, r *http.Request, route, id string) {
	if route == "settings" {
		serveGo2RTCDetectionSettings(s, w, r)
		return
	}
	svc := go2RTCDetectionService(s, w)
	if svc == nil {
		return
	}
	switch route {
	case "events":
		serveGo2RTCDetectionDelete(s, svc, w, r, id)
	case "arm":
		serveGo2RTCDetectionArm(s, svc, w, r)
	case "test":
		serveGo2RTCDetectionTest(s, svc, w, r)
	case "retry":
		serveGo2RTCDetectionRetry(s, svc, w)
	}
}

func serveGo2RTCDetectionState(s *Server, svc *persondetect.Service, w http.ResponseWriter, r *http.Request) {
	values, present := r.URL.Query()["since"]
	if !present {
		writeGo2RTCDetectionJSON(w, http.StatusOK, map[string]interface{}{"status": svc.Status()})
		return
	}
	cursor, err := strconv.ParseInt(strings.TrimSpace(values[0]), 10, 64)
	if err != nil || cursor < 0 {
		writeGo2RTCDetectionError(w, http.StatusBadRequest, "invalid_request", "since must be a non-negative cursor")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	changes, err := svc.Changes(ctx, cursor)
	if err != nil {
		writeGo2RTCDetectionServiceError(s, w, err, go2RTCDetectionNotFound{}, "Person detection failed")
		return
	}
	writeGo2RTCDetectionJSON(w, http.StatusOK, changes)
}

func go2RTCDetectionEventQuery(r *http.Request) (persondetect.EventQuery, error) {
	values := r.URL.Query()
	q := persondetect.EventQuery{CameraID: strings.TrimSpace(values.Get("stream_id"))}
	if q.CameraID != "" && tools.ValidateGo2RTCStreamID(q.CameraID) != nil {
		return q, errors.New("stream_id is not a valid stream id")
	}
	for _, bound := range []struct {
		name   string
		target *time.Time
	}{{"from", &q.From}, {"to", &q.To}} {
		raw := strings.TrimSpace(values.Get(bound.name))
		if raw == "" {
			continue
		}
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return q, fmt.Errorf("%s must be an RFC 3339 time", bound.name)
		}
		*bound.target = parsed
	}
	if !q.From.IsZero() && !q.To.IsZero() && !q.To.After(q.From) {
		return q, errors.New("to must be after from")
	}
	if raw := strings.TrimSpace(values.Get("before")); raw != "" {
		// Event ids start around 1.8e12 (Unix milliseconds): always int64.
		before, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || before < 1 {
			return q, errors.New("before must be a positive event id")
		}
		q.Before = before
	}
	if raw := strings.TrimSpace(values.Get("limit")); raw != "" {
		limit, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || limit < 1 || limit > 100 {
			return q, errors.New("limit must be between 1 and 100")
		}
		q.Limit = int(limit)
	}
	return q, nil
}

func serveGo2RTCDetectionEvents(s *Server, svc *persondetect.Service, w http.ResponseWriter, r *http.Request) {
	q, err := go2RTCDetectionEventQuery(r)
	if err != nil {
		writeGo2RTCDetectionError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	page, err := svc.Events(r.Context(), q)
	if err != nil {
		writeGo2RTCDetectionServiceError(s, w, err, go2RTCDetectionNotFound{}, "Person detection failed")
		return
	}
	writeGo2RTCDetectionJSON(w, http.StatusOK, page)
}

// parseGo2RTCDetectionID parses an event or frame id. Ids start around 1.8e12
// (Unix milliseconds), so they are parsed as int64, never int.
func parseGo2RTCDetectionID(raw string) (int64, bool) {
	id, err := strconv.ParseInt(raw, 10, 64)
	return id, err == nil && id > 0
}

func serveGo2RTCDetectionEvent(s *Server, svc *persondetect.Service, w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseGo2RTCDetectionID(rawID)
	if !ok {
		writeGo2RTCDetectionError(w, http.StatusNotFound, "event_not_found", "Event not found")
		return
	}
	detail, err := svc.Event(r.Context(), id)
	if err != nil {
		writeGo2RTCDetectionServiceError(s, w, err, go2RTCDetectionEventNotFound, "Person detection failed")
		return
	}
	writeGo2RTCDetectionJSON(w, http.StatusOK, detail)
}

// serveGo2RTCDetectionImage serves a stored frame or its thumbnail. FrameFile
// returns a regular file inside the detections root; frame ids are never reused,
// so the image is cacheable as immutable. A 0-byte file (frames are written
// without fsync, so a power loss can leave one) is answered as not found instead
// of an empty image the browser would keep.
func serveGo2RTCDetectionImage(s *Server, svc *persondetect.Service, w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseGo2RTCDetectionID(rawID)
	if !ok {
		writeGo2RTCDetectionError(w, http.StatusNotFound, "frame_not_found", "Frame not found")
		return
	}
	path, err := svc.FrameFile(r.Context(), id, r.URL.Query().Get("thumb") == "1")
	if err != nil {
		writeGo2RTCDetectionServiceError(s, w, err, go2RTCDetectionFrameNotFound, "Person detection failed")
		return
	}
	file, err := os.Open(path)
	if err != nil {
		writeGo2RTCDetectionError(w, http.StatusNotFound, "frame_not_found", "Frame not found")
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		writeGo2RTCDetectionError(w, http.StatusNotFound, "frame_not_found", "Frame not found")
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "private, max-age=86400, immutable")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, "", info.ModTime(), file)
}

func serveGo2RTCDetectionDelete(s *Server, svc *persondetect.Service, w http.ResponseWriter, r *http.Request, rawID string) {
	id, ok := parseGo2RTCDetectionID(rawID)
	if !ok {
		writeGo2RTCDetectionError(w, http.StatusNotFound, "event_not_found", "Event not found")
		return
	}
	if err := svc.DeleteEvent(r.Context(), id); err != nil {
		writeGo2RTCDetectionServiceError(s, w, err, go2RTCDetectionEventNotFound, "The event could not be deleted")
		return
	}
	writeGo2RTCDetectionJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func serveGo2RTCDetectionArm(s *Server, svc *persondetect.Service, w http.ResponseWriter, r *http.Request) {
	var body struct {
		Mode string `json:"mode"`
	}
	// decodeGo2RTCAppJSON bounds the body (http.MaxBytesReader, 64 KiB) before decoding.
	if err := decodeGo2RTCAppJSON(w, r, &body); err != nil {
		writeGo2RTCDetectionError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	state, err := svc.SetArmMode(r.Context(), persondetect.ArmMode(strings.TrimSpace(body.Mode)), persondetect.ActorUser)
	switch {
	case errors.Is(err, persondetect.ErrInvalidArmMode):
		writeGo2RTCDetectionError(w, http.StatusBadRequest, "invalid_request", "mode must be off, on or schedule")
	case err != nil:
		writeGo2RTCDetectionServiceError(s, w, err, go2RTCDetectionNotFound{}, "The arm mode could not be saved")
	default:
		writeGo2RTCDetectionJSON(w, http.StatusOK, state)
	}
}

func serveGo2RTCDetectionSettings(s *Server, w http.ResponseWriter, r *http.Request) {
	var payload config.Go2RTCDetectionConfig
	if err := decodeGo2RTCAppJSON(w, r, &payload); err != nil {
		writeGo2RTCDetectionError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	payload = config.NormalizeGo2RTCDetectionSettings(payload)
	if err := config.ValidateGo2RTCDetectionSettings(payload); err != nil {
		writeGo2RTCDetectionFieldError(w, err)
		return
	}
	result, err := updateManagedGo2RTCConfig(r.Context(), s, func(value *config.Go2RTCConfig) error {
		value.Detection = payload
		return nil
	}, nil, false)
	if err != nil {
		var mutationErr *go2RTCMutationError
		var fieldErr *config.Go2RTCDetectionFieldError
		switch {
		case errors.As(err, &mutationErr):
			writeGo2RTCDetectionError(w, mutationErr.Status, mutationErr.Code, security.Scrub(mutationErr.Message))
		case errors.As(err, &fieldErr):
			writeGo2RTCDetectionFieldError(w, err)
		default:
			go2RTCDetectionLogger(s).Warn("[PersonDetect] Saving the detection settings failed", "error", security.Scrub(err.Error()))
			writeGo2RTCDetectionError(w, http.StatusInternalServerError, "save_failed", "The detection settings could not be saved")
		}
		return
	}
	status, code := "ok", http.StatusOK
	if result.ReconcileErr != nil {
		status, code = "degraded", http.StatusAccepted
	}
	writeGo2RTCDetectionJSON(w, code, map[string]interface{}{"status": status, "detection": result.Config.Detection})
}

// go2RTCConfiguredStream finds a configured stream (enabled or not) by id.
func go2RTCConfiguredStream(s *Server, streamID string) (config.Go2RTCStreamConfig, bool) {
	if tools.ValidateGo2RTCStreamID(streamID) != nil {
		return config.Go2RTCStreamConfig{}, false
	}
	cfg := s.ConfigSnapshot()
	if cfg == nil {
		return config.Go2RTCStreamConfig{}, false
	}
	for _, stream := range cfg.Go2RTC.Streams {
		if stream.ID == streamID {
			return stream, true
		}
	}
	return config.Go2RTCStreamConfig{}, false
}

// serveGo2RTCDetectionTest analyzes one current frame. An optional detection object
// (the zone editor's unsaved state) replaces the camera's saved block for this test.
func serveGo2RTCDetectionTest(s *Server, svc *persondetect.Service, w http.ResponseWriter, r *http.Request) {
	var body struct {
		StreamID  string          `json:"stream_id"`
		Detection json.RawMessage `json:"detection"`
	}
	if err := decodeGo2RTCAppJSON(w, r, &body); err != nil {
		writeGo2RTCDetectionError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	stream, found := go2RTCConfiguredStream(s, strings.TrimSpace(body.StreamID))
	if !found {
		writeGo2RTCDetectionError(w, http.StatusNotFound, "stream_not_found", "Stream not found")
		return
	}
	detection := stream.Detection
	if len(body.Detection) > 0 && string(body.Detection) != "null" {
		decoded, err := decodeGo2RTCStreamDetection(body.Detection)
		if err != nil {
			writeGo2RTCDetectionFieldError(w, err)
			return
		}
		detection = decoded
	}
	detection = config.NormalizeGo2RTCStreamDetection(detection)
	if err := config.ValidateGo2RTCStreamDetection(detection); err != nil {
		writeGo2RTCDetectionFieldError(w, config.PrefixGo2RTCDetectionField(err, "detection"))
		return
	}
	camera := personDetectionCamera(stream.ID, stream.Name, detection)
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	result, err := svc.Test(ctx, stream.ID, &camera)
	switch {
	case err == nil:
		writeGo2RTCDetectionJSON(w, http.StatusOK, result)
	case errors.Is(err, persondetect.ErrNotReady):
		writeGo2RTCDetectionError(w, http.StatusConflict, "detector_not_ready", "The person detector is not ready")
	case errors.Is(err, persondetect.ErrUnknownCamera):
		writeGo2RTCDetectionError(w, http.StatusNotFound, "stream_not_found", "Stream not found")
	case errors.Is(err, persondetect.ErrFrameUnavailable):
		go2RTCDetectionLogger(s).Warn("[PersonDetect] Test frame unavailable", "stream_id", stream.ID, "error", security.Scrub(err.Error()))
		writeGo2RTCDetectionError(w, http.StatusBadGateway, "frame_unavailable", "The camera frame is unavailable")
	default:
		writeGo2RTCDetectionServiceError(s, w, err, go2RTCDetectionNotFound{}, "Person detection failed")
	}
}

func serveGo2RTCDetectionRetry(s *Server, svc *persondetect.Service, w http.ResponseWriter) {
	if err := svc.Retry(); err != nil && !errors.Is(err, persondetect.ErrDisabled) {
		writeGo2RTCDetectionServiceError(s, w, err, go2RTCDetectionNotFound{}, "Person detection failed")
		return
	}
	writeGo2RTCDetectionJSON(w, http.StatusOK, map[string]interface{}{"status": svc.Status()})
}
