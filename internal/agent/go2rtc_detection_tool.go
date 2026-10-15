package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/persondetect"
	"aurago/internal/security"
	"aurago/internal/tools"
)

const (
	go2RTCDetectionDefaultLimit = 10
	go2RTCDetectionMaxLimit     = 50
	// go2RTCDetectionMessageRunes bounds error texts taken from the service or the vision call.
	go2RTCDetectionMessageRunes = 200
	// go2RTCDetectionDefaultPrompt must not ask for a quantity ("how many", "number of"):
	// tools.PrepareVisionPrompt would turn it into a structured vehicle count.
	go2RTCDetectionDefaultPrompt = "Analyze this camera frame from a person detection event. Describe the persons you see (where they are, what they do, what they carry), whether anything looks unusual, and anything that may need attention."
)

// go2RTCDetectionOperations are the person detection operations of the go2rtc tool.
var go2RTCDetectionOperations = []string{"detection_status", "set_arm_mode", "list_detection_events", "analyze_detection_event", "show_detection_event"}

func isGo2RTCDetectionOperation(operation string) bool {
	return slices.Contains(go2RTCDetectionOperations, operation)
}

// detectionAPI is what the go2rtc tool needs from person detection. Values come back in the
// JSON shapes of the detection HTTP API (overview contract C4), so the tool depends on those
// shapes only and its tests use a fake instead of a detector.
type detectionAPI interface {
	Status(ctx context.Context) (detectionStatusView, error)
	SetArmMode(ctx context.Context, mode string) (detectionArmView, error)
	Events(ctx context.Context, q detectionEventQuery) (detectionEventPageView, error)
	Event(ctx context.Context, id int64) (detectionEventView, error)
	// FrameLocation names a stored frame as its image root and a path below it, so the
	// frame can be opened through os.Root even when the root links to another disk.
	FrameLocation(ctx context.Context, frameID int64) (root, rel string, err error)
}

// detectionArmView is C4's ArmState. Timestamps are UTC; time_zone and utc_offset name the
// server zone the schedule runs in.
type detectionArmView struct {
	Mode       string     `json:"mode"`
	Armed      bool       `json:"armed"`
	NextChange *time.Time `json:"next_change"`
	ChangedAt  *time.Time `json:"changed_at"`
	ChangedBy  string     `json:"changed_by"`
	TimeZone   string     `json:"time_zone,omitempty"`
	UTCOffset  string     `json:"utc_offset,omitempty"`
}

type detectionDownloadView struct {
	Percent float64 `json:"percent"`
}

// detectionDetectorView keeps the detector fields the agent needs; runtime, providers and
// self_test of C4's DetectorStatus are left out.
type detectionDetectorView struct {
	State            string                `json:"state"`
	Phase            string                `json:"phase"`
	Download         detectionDownloadView `json:"download"`
	Error            string                `json:"error"`
	Model            string                `json:"model"`
	ModelName        string                `json:"model_name"`
	RequestedBackend string                `json:"requested_backend"`
	Backend          string                `json:"backend"`
	FallbackReason   string                `json:"fallback_reason"`
	WallMS           float64               `json:"wall_ms"`
	CPUMS            float64               `json:"cpu_ms"`
}

type detectionCameraView struct {
	ID                string     `json:"id"`
	Name              string     `json:"name"`
	State             string     `json:"state"`
	AnalysesPerMinute float64    `json:"analyses_per_minute"`
	LastEventID       *int64     `json:"last_event_id"`
	LastEventAt       *time.Time `json:"last_event_at"`
	LastError         string     `json:"last_error"`
}

// detectionStatusView is C4's Status without storage and cursor.
type detectionStatusView struct {
	Enabled  bool                  `json:"enabled"`
	Detector detectionDetectorView `json:"detector"`
	Arm      detectionArmView      `json:"arm"`
	Cameras  []detectionCameraView `json:"cameras"`
}

type detectionAlertView struct {
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

type detectionEventView struct {
	ID           int64                         `json:"id"`
	CameraID     string                        `json:"camera_id"`
	CameraName   string                        `json:"camera_name"`
	StartedAt    time.Time                     `json:"started_at"`
	EndedAt      *time.Time                    `json:"ended_at"`
	MaxScore     float64                       `json:"max_score"`
	MaxPersons   int                           `json:"max_persons"`
	Zones        []string                      `json:"zones"`
	Armed        bool                          `json:"armed"`
	CoverFrameID *int64                        `json:"cover_frame_id"`
	FrameCount   int                           `json:"frame_count"`
	Alerts       map[string]detectionAlertView `json:"alerts"`
}

type detectionEventPageView struct {
	Events     []detectionEventView `json:"events"`
	NextBefore *int64               `json:"next_before"`
}

// detectionEventQuery filters events on their start time: From inclusive, To exclusive.
type detectionEventQuery struct {
	CameraID string
	From, To time.Time
	Limit    int
}

// personDetectAPI adapts the server's *persondetect.Service to detectionAPI. The tool only
// calls it while go2rtc.detection.enabled is on, so a service that still reports detection
// as off has not been configured yet: the adapter answers that as ErrUnavailable
// ("starting"), not ErrDisabled.
type personDetectAPI struct{ svc *persondetect.Service }

var errDetectionNotRunningYet = fmt.Errorf("%w: person detection is not running yet", persondetect.ErrUnavailable)

func (a personDetectAPI) Status(context.Context) (detectionStatusView, error) {
	var view detectionStatusView
	return view, detectionViaJSON(a.svc.Status(), &view)
}

func (a personDetectAPI) SetArmMode(ctx context.Context, mode string) (detectionArmView, error) {
	state, err := a.svc.SetArmMode(ctx, persondetect.ArmMode(mode), persondetect.ActorAgent)
	if errors.Is(err, persondetect.ErrDisabled) {
		return detectionArmView{}, errDetectionNotRunningYet
	}
	if err != nil {
		return detectionArmView{}, err
	}
	var view detectionArmView
	return view, detectionViaJSON(state, &view)
}

// Events answers an empty page while detection is off; the adapter reports that as not
// running yet instead of "no events".
func (a personDetectAPI) Events(ctx context.Context, q detectionEventQuery) (detectionEventPageView, error) {
	page, err := a.svc.Events(ctx, persondetect.EventQuery{CameraID: q.CameraID, From: q.From, To: q.To, Limit: q.Limit})
	if err != nil {
		return detectionEventPageView{}, err
	}
	if len(page.Events) == 0 && !a.svc.Status().Enabled {
		return detectionEventPageView{}, errDetectionNotRunningYet
	}
	var view detectionEventPageView
	return view, detectionViaJSON(page, &view)
}

func (a personDetectAPI) Event(ctx context.Context, id int64) (detectionEventView, error) {
	detail, err := a.svc.Event(ctx, id)
	if err != nil {
		return detectionEventView{}, a.offOr(err)
	}
	var view detectionEventView
	return view, detectionViaJSON(detail, &view)
}

func (a personDetectAPI) FrameLocation(ctx context.Context, frameID int64) (string, string, error) {
	root, rel, err := a.svc.FrameLocation(ctx, frameID, false)
	if err != nil {
		return "", "", a.offOr(err)
	}
	return root, rel, nil
}

// offOr turns the "unknown event" that Event and FrameLocation answer while detection is
// off into "not running yet".
func (a personDetectAPI) offOr(err error) error {
	if errors.Is(err, persondetect.ErrUnknownEvent) && !a.svc.Status().Enabled {
		return errDetectionNotRunningYet
	}
	return err
}

func detectionViaJSON(src, dst any) error {
	raw, err := json.Marshal(src)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, dst)
}

// currentDetectionAPI returns the published person detection service, nil without one;
// tests replace it.
var currentDetectionAPI = func() detectionAPI {
	svc := persondetect.Default()
	if svc == nil {
		return nil
	}
	return personDetectAPI{svc: svc}
}

// dispatchGo2RTCDetection runs a person detection operation of the go2rtc tool. The caller
// has applied the go2rtc gate (enabled, agent access, reachable API); this adds
// go2rtc.detection.enabled and a running service.
func dispatchGo2RTCDetection(ctx context.Context, operation string, tc ToolCall, dc *DispatchContext) string {
	if dc.Logger != nil {
		dc.Logger.Info("LLM requested go2rtc person detection", "operation", operation)
	}
	if dc.Cfg == nil || !dc.Cfg.Go2RTC.Detection.Enabled {
		return go2RTCDetectionFailure("detection_disabled", "Person detection is switched off. The user can enable it in the Network Cameras app.")
	}
	api := currentDetectionAPI()
	if api == nil {
		return go2RTCDetectionFailure("detection_unavailable", "Person detection is not running right now; try again shortly.")
	}
	switch operation {
	case "detection_status":
		return go2RTCDetectionStatus(ctx, dc.Cfg, api)
	case "set_arm_mode":
		return go2RTCSetArmMode(ctx, api, toolArgString(tc.Params, "mode"), isAutonomousDetectionRun(dc))
	case "list_detection_events":
		return go2RTCListDetectionEvents(ctx, dc.Cfg, api, tc)
	case "analyze_detection_event":
		return go2RTCAnalyzeDetectionEvent(ctx, api, tc, dc)
	case "show_detection_event":
		return go2RTCShowDetectionEvent(ctx, api, tc, dc)
	default:
		return go2RTCDetectionFailure("invalid_request", "Unknown person detection operation.")
	}
}

func go2RTCDetectionStatus(ctx context.Context, cfg *config.Config, api detectionAPI) string {
	status, err := api.Status(ctx)
	if err != nil {
		return go2RTCDetectionError(err)
	}
	var lines strings.Builder
	listed := map[string]bool{}
	for _, c := range status.Cameras {
		listed[c.ID] = true
		fmt.Fprintf(&lines, "%s | %s | %s | %.1f | %s | %s", c.ID, c.Name, c.State, c.AnalysesPerMinute,
			detectionOptionalID(c.LastEventID), detectionOptionalTime(c.LastEventAt))
		if c.LastError != "" {
			lines.WriteString(" | " + c.LastError)
		}
		lines.WriteString("\n")
	}
	for _, stream := range cfg.Go2RTC.Streams {
		if !stream.Enabled || listed[stream.ID] {
			continue
		}
		state := "off"
		if stream.Detection.Enabled {
			state = "starting"
		}
		fmt.Fprintf(&lines, "%s | %s | %s | 0.0 | - | -\n", stream.ID, stream.Name, state)
	}
	detector := map[string]any{
		"state":             status.Detector.State,
		"model":             status.Detector.Model,
		"model_name":        status.Detector.ModelName,
		"backend":           status.Detector.Backend,
		"requested_backend": status.Detector.RequestedBackend,
		"wall_ms":           math.Round(status.Detector.WallMS),
		"cpu_ms":            math.Round(status.Detector.CPUMS),
	}
	if status.Detector.FallbackReason != "" {
		detector["fallback_reason"] = status.Detector.FallbackReason
	}
	if status.Detector.Error != "" {
		detector["error"] = status.Detector.Error
	}
	if status.Detector.State == "downloading" {
		detector["download_percent"] = math.Round(status.Detector.Download.Percent)
	}
	payload := map[string]any{
		"status":        "ok",
		"operation":     "detection_status",
		"enabled":       status.Enabled,
		"detector":      detector,
		"arm":           status.Arm,
		"camera_format": "camera_id | name | state | analyses_per_minute | last_event_id | last_event_at | last_error",
	}
	if lines.Len() > 0 {
		payload["cameras"] = security.IsolateExternalData(strings.TrimRight(lines.String(), "\n"))
	}
	return go2RTCDetectionResult(payload)
}

func isAutonomousDetectionRun(dc *DispatchContext) bool {
	return dc.MissionID != "" || isAutonomousAgentRun(RunConfig{MessageSource: dc.MessageSource}, dc.SessionID)
}

// go2RTCSetArmMode only lets autonomous runs arm: a staged picture must not silence later alerts.
func go2RTCSetArmMode(ctx context.Context, api detectionAPI, raw string, autonomous bool) string {
	mode := strings.ToLower(strings.TrimSpace(raw))
	if !persondetect.ValidArmMode(mode) {
		return go2RTCDetectionFailure("invalid_request", "mode must be off, on or schedule.")
	}
	if autonomous && mode != string(persondetect.ArmOn) {
		return go2RTCDetectionFailure("not_allowed", "Only the user can switch person detection alerts off or back to the schedule; an autonomous run may only arm them (mode on). Tell the user what you saw instead.")
	}
	arm, err := api.SetArmMode(ctx, mode)
	if err != nil {
		return go2RTCDetectionError(err)
	}
	return go2RTCDetectionResult(map[string]any{
		"status":    "ok",
		"operation": "set_arm_mode",
		"arm":       arm,
		"message":   "Arm mode set to " + mode + "; the arm log records the agent as the actor.",
	})
}

func go2RTCListDetectionEvents(ctx context.Context, cfg *config.Config, api detectionAPI, tc ToolCall) string {
	since, err := parseDetectionTime(toolArgString(tc.Params, "since"), false)
	if err != nil {
		return go2RTCDetectionFailure("invalid_request", "since must be an RFC 3339 time or a date YYYY-MM-DD.")
	}
	until, err := parseDetectionTime(toolArgString(tc.Params, "until"), true)
	if err != nil {
		return go2RTCDetectionFailure("invalid_request", "until must be an RFC 3339 time or a date YYYY-MM-DD.")
	}
	if !since.IsZero() && !until.IsZero() && !until.After(since) {
		return go2RTCDetectionFailure("invalid_request", "until must be after since; a date as until covers that whole day.")
	}
	cameraID := strings.ToLower(strings.TrimSpace(toolArgString(tc.Params, "stream_id")))
	// The service answers an unknown camera with an empty page. Configured streams (also
	// disabled ones, whose events stay listed) are the camera IDs detection can know.
	if cameraID != "" && !slices.ContainsFunc(cfg.Go2RTC.Streams, func(s config.Go2RTCStreamConfig) bool { return s.ID == cameraID }) {
		return go2RTCDetectionFailure("invalid_request", "Unknown stream_id; take it from list_streams or detection_status.")
	}
	limit := toolArgInt(tc.Params, go2RTCDetectionDefaultLimit, "limit")
	if limit <= 0 {
		limit = go2RTCDetectionDefaultLimit
	}
	if limit > go2RTCDetectionMaxLimit {
		limit = go2RTCDetectionMaxLimit
	}
	page, err := api.Events(ctx, detectionEventQuery{CameraID: cameraID, From: since, To: until, Limit: limit})
	if err != nil {
		return go2RTCDetectionError(err)
	}
	lines := make([]string, 0, len(page.Events))
	for _, ev := range page.Events {
		lines = append(lines, detectionEventLine(ev))
	}
	payload := map[string]any{
		"status":    "ok",
		"operation": "list_detection_events",
		"returned":  len(lines),
		"format":    "event_id | camera_id | camera_name | started_at | ended_at | max_score | persons | zones | armed | alerts",
	}
	if len(lines) > 0 {
		payload["events"] = security.IsolateExternalData(strings.Join(lines, "\n"))
	}
	if page.NextBefore != nil {
		payload["next_before"] = *page.NextBefore
		payload["note"] = "More events exist; narrow since/until instead of paging far back."
	}
	return go2RTCDetectionResult(payload)
}

func go2RTCAnalyzeDetectionEvent(ctx context.Context, api detectionAPI, tc ToolCall, dc *DispatchContext) string {
	if dc.BudgetTracker != nil && dc.BudgetTracker.IsBlocked("vision") {
		return go2RTCDetectionFailure("vision_budget_exceeded", "Vision analysis is blocked because the daily budget is exceeded.")
	}
	ev, frameID, frameRoot, frameRel, failure := go2RTCDetectionCover(ctx, api, tc.Params)
	if failure != "" {
		return failure
	}
	prompt := strings.TrimSpace(toolArgString(tc.Params, "prompt"))
	if prompt == "" {
		prompt = go2RTCDetectionDefaultPrompt
	}
	prompt, _ = tools.PrepareVisionPrompt(prompt)
	analysis, promptTokens, completionTokens, err := analyzeManagedImageAtWithPrompt(frameRoot, frameRel, prompt, dc.Cfg)
	if err != nil {
		return go2RTCDetectionFailure("analysis_failed", "Event picture analysis failed: "+boundedRunes(err.Error(), go2RTCDetectionMessageRunes))
	}
	if dc.BudgetTracker != nil {
		model := dc.Cfg.Vision.Model
		if model == "" {
			model = tools.DefaultVisionModel
		}
		dc.BudgetTracker.RecordForCategory("vision", model, promptTokens, completionTokens)
	}
	return go2RTCDetectionResult(map[string]any{
		"status":    "ok",
		"operation": "analyze_detection_event",
		"event_id":  ev.ID,
		"camera_id": ev.CameraID,
		"frame_id":  frameID,
		"analysis":  analysis,
		"prompt":    prompt,
		"event":     security.IsolateExternalData(detectionEventLine(ev)),
	})
}

func go2RTCShowDetectionEvent(ctx context.Context, api detectionAPI, tc ToolCall, dc *DispatchContext) string {
	ev, frameID, _, _, failure := go2RTCDetectionCover(ctx, api, tc.Params)
	if failure != "" {
		return failure
	}
	// A camera wake-up (and every other autonomous run) has no chat attached: its broker
	// is a NoopBroker, so the picture would reach nobody. Say so instead of claiming it.
	noopBroker := false
	switch dc.Broker.(type) {
	case NoopBroker, *NoopBroker:
		noopBroker = true
	}
	if dc.Broker == nil || noopBroker || isAutonomousDetectionRun(dc) {
		return go2RTCDetectionResult(map[string]any{
			"status":    "ok",
			"operation": "show_detection_event",
			"event_id":  ev.ID,
			"camera_id": ev.CameraID,
			"frame_id":  frameID,
			"shown":     false,
			"message":   "No chat is attached to this run, so the picture was not shown to anyone; use send_notification to reach the user.",
			"event":     security.IsolateExternalData(detectionEventLine(ev)),
		})
	}
	shown := false
	if dc.Broker != nil {
		payload, err := json.Marshal(map[string]string{
			"path":    go2RTCDetectionImagePath(frameID),
			"caption": fmt.Sprintf("Person detection: %s, %s", ev.CameraID, ev.StartedAt.Local().Format("2006-01-02 15:04")),
		})
		if err == nil {
			dc.Broker.Send("image", string(payload))
			shown = true
		}
	}
	return go2RTCDetectionResult(map[string]any{
		"status":    "ok",
		"operation": "show_detection_event",
		"event_id":  ev.ID,
		"camera_id": ev.CameraID,
		"frame_id":  frameID,
		"shown":     shown,
		"event":     security.IsolateExternalData(detectionEventLine(ev)),
		"recommended_follow_up": map[string]any{
			"call_method": "native_tool",
			"tool_name":   "go2rtc",
			"arguments":   map[string]any{"operation": "analyze_detection_event", "event_id": ev.ID},
		},
	})
}

// go2RTCDetectionCover resolves the event_id argument to the event, its cover frame and the
// frame's location (image root and path below it); failure is a finished tool output when
// it is not empty.
func go2RTCDetectionCover(ctx context.Context, api detectionAPI, params map[string]interface{}) (ev detectionEventView, frameID int64, root, rel, failure string) {
	id, ok := detectionEventID(params)
	if !ok {
		return ev, 0, "", "", go2RTCDetectionFailure("invalid_request", "event_id is required; take it from list_detection_events.")
	}
	ev, err := api.Event(ctx, id)
	if err != nil {
		return ev, 0, "", "", go2RTCDetectionError(err)
	}
	if ev.CoverFrameID == nil || *ev.CoverFrameID <= 0 {
		return ev, 0, "", "", go2RTCDetectionFailure("event_not_found", "This event has no stored picture any more (retention may have removed it).")
	}
	root, rel, err = api.FrameLocation(ctx, *ev.CoverFrameID)
	if err != nil {
		return ev, 0, "", "", go2RTCDetectionFrameError(err)
	}
	return ev, *ev.CoverFrameID, root, rel, ""
}

// go2RTCDetectionImagePath is the web path of a stored frame (C6, gated by requireGo2RTCView).
func go2RTCDetectionImagePath(frameID int64) string {
	return "/api/go2rtc/detection/images/" + strconv.FormatInt(frameID, 10)
}

// detectionEventID reads event_id as a positive integer from a JSON number or digits. Ids
// are about 1.8e12, so float64 holds them exactly.
func detectionEventID(params map[string]interface{}) (int64, bool) {
	switch v := params["event_id"].(type) {
	case float64:
		if v >= 1 && v <= 1<<53 && v == math.Trunc(v) {
			return int64(v), true
		}
	case int:
		if v > 0 {
			return int64(v), true
		}
	case int64:
		if v > 0 {
			return v, true
		}
	case json.Number:
		if id, err := v.Int64(); err == nil && id > 0 {
			return id, true
		}
	case string:
		if id, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil && id > 0 {
			return id, true
		}
	}
	return 0, false
}

// detectionLocalTimeLayouts are RFC 3339 times without an offset, read as server time.
var detectionLocalTimeLayouts = []string{"2006-01-02T15:04:05", "2006-01-02T15:04", "2006-01-02 15:04:05", "2006-01-02 15:04"}

// parseDetectionTime reads an RFC 3339 time, a time without an offset (server time) or a
// server-local date. The service's end of a range is exclusive, so a date as the end
// becomes the next local midnight and covers the whole day.
func parseDetectionTime(raw string, end bool) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, nil
	}
	for _, layout := range detectionLocalTimeLayouts {
		if t, err := time.ParseInLocation(layout, raw, time.Local); err == nil {
			return t, nil
		}
	}
	day, err := time.ParseInLocation("2006-01-02", raw, time.Local)
	if err != nil {
		return time.Time{}, err
	}
	if end {
		return day.AddDate(0, 0, 1), nil
	}
	return day, nil
}

// detectionEventLine renders one event for the agent; callers isolate it, because camera and
// zone names are configuration text and alert details carry channel errors.
func detectionEventLine(ev detectionEventView) string {
	ended := "ongoing"
	if ev.EndedAt != nil {
		ended = ev.EndedAt.Local().Format(time.RFC3339)
	}
	zones := strings.Join(ev.Zones, ", ")
	if zones == "" {
		zones = "-"
	}
	armed := "no"
	if ev.Armed {
		armed = "yes"
	}
	return fmt.Sprintf("%d | %s | %s | %s | %s | %d%% | %d | %s | %s | %s", ev.ID, ev.CameraID, ev.CameraName,
		ev.StartedAt.Local().Format(time.RFC3339), ended, detectionPercent(ev.MaxScore), ev.MaxPersons, zones, armed,
		detectionAlertsText(ev.Alerts))
}

func detectionAlertsText(alerts map[string]detectionAlertView) string {
	parts := []string{}
	for _, action := range []string{persondetect.ActionMessage, persondetect.ActionAgent, persondetect.ActionDesktop, persondetect.ActionMission} {
		a, ok := alerts[action]
		if !ok || a.Status == "" {
			continue
		}
		text := action + "=" + a.Status
		if a.Detail != "" {
			text += " (" + a.Detail + ")"
		}
		parts = append(parts, text)
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, ", ")
}

func detectionPercent(score float64) int {
	if math.IsNaN(score) {
		return 0
	}
	return int(math.Round(math.Max(0, math.Min(1, score)) * 100))
}

func detectionOptionalID(id *int64) string {
	if id == nil {
		return "-"
	}
	return strconv.FormatInt(*id, 10)
}

func detectionOptionalTime(t *time.Time) string {
	if t == nil || t.IsZero() {
		return "-"
	}
	return t.Local().Format(time.RFC3339)
}

func go2RTCDetectionError(err error) string {
	switch {
	case errors.Is(err, persondetect.ErrDisabled):
		return go2RTCDetectionFailure("detection_disabled", "Person detection is switched off. The user can enable it in the Network Cameras app.")
	case errors.Is(err, persondetect.ErrUnavailable):
		return go2RTCDetectionFailure("detection_unavailable", "Person detection is starting or its database is not open; try again shortly.")
	case errors.Is(err, persondetect.ErrUnknownEvent):
		return go2RTCDetectionFailure("event_not_found", "No such detection event. List events first and pass an event_id from the results.")
	case errors.Is(err, persondetect.ErrInvalidArmMode):
		return go2RTCDetectionFailure("invalid_request", "mode must be off, on or schedule.")
	case errors.Is(err, persondetect.ErrUnknownCamera):
		return go2RTCDetectionFailure("invalid_request", "Unknown stream_id; take it from list_streams or detection_status.")
	default:
		return go2RTCDetectionFailure("detection_unavailable", "Person detection could not answer: "+boundedRunes(err.Error(), go2RTCDetectionMessageRunes))
	}
}

// go2RTCDetectionFrameError maps a FrameLocation error: "disabled" and "unavailable" keep their
// meaning, any other error means the frame's file is gone.
func go2RTCDetectionFrameError(err error) string {
	if errors.Is(err, persondetect.ErrDisabled) || errors.Is(err, persondetect.ErrUnavailable) {
		return go2RTCDetectionError(err)
	}
	return go2RTCDetectionFailure("event_not_found", "The picture of this event is no longer stored (retention may have removed it).")
}

func go2RTCDetectionFailure(code, message string) string {
	return go2RTCDetectionResult(map[string]any{"status": "error", "code": code, "message": message})
}

// go2RTCDetectionResult encodes without HTML escaping so the isolation tags stay readable;
// IsolateExternalData has already escaped the configuration text inside them.
func go2RTCDetectionResult(payload map[string]any) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(payload); err != nil {
		return `Tool Output: {"status":"error","code":"detection_unavailable","message":"result could not be encoded"}`
	}
	return "Tool Output: " + strings.TrimRight(buf.String(), "\n")
}
