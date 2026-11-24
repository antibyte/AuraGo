package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/videostudio/storyplan"
)

// videoStudioPlanConfigure publishes a changed copy of the config snapshot, the way a saved config does.
func videoStudioPlanConfigure(s *Server, change func(cfg *config.Config)) {
	next := *s.ConfigSnapshot()
	change(&next)
	s.replaceConfigSnapshot(&next)
}

func decodeVideoStudioPlanReply(t *testing.T, body []byte) videoStudioPlanResponse {
	t.Helper()
	var reply videoStudioPlanResponse
	if err := json.Unmarshal(body, &reply); err != nil {
		t.Fatalf("plan reply: %v %s", err, body)
	}
	return reply
}

func TestVideoStudioPlanAPIGetAndPut(t *testing.T) {
	s, readToken, writeToken, projectID := videoStudioPlanFixture(t)
	path := "/projects/" + projectID + "/plan"
	if w := videoStudioAPIRequest(s, readToken, http.MethodGet, path, nil, nil); w.Code != http.StatusNotFound || !strings.Contains(w.Body.String(), `"plan_not_found"`) {
		t.Fatalf("GET without plan = %d %s", w.Code, w.Body.String())
	}
	fixture, err := json.Marshal(videoStudioPlanFixturePlan())
	if err != nil {
		t.Fatal(err)
	}
	someVersion := `"` + strings.Repeat("a", 64) + `"`
	if w := videoStudioAPIRequest(s, writeToken, http.MethodPut, path, bytes.NewReader(fixture), map[string]string{"If-Match": someVersion}); w.Code != http.StatusNotFound || !strings.Contains(w.Body.String(), `"plan_not_found"`) {
		t.Fatalf("PUT without plan = %d %s", w.Code, w.Body.String())
	}

	seed := videoStudioPlanFixturePlan()
	seed.Status = storyplan.StatusPlanning // a job that was interrupted: GET reports review
	seed.Scenes[0].State = storyplan.SceneReady
	seed.Scenes[0].ClipAssetID = "clip-1"
	seeded := videoStudioPlanSeed(t, s, projectID, seed, "")

	got := videoStudioAPIRequest(s, readToken, http.MethodGet, path, nil, nil)
	if got.Code != http.StatusOK || got.Header().Get("ETag") != seeded {
		t.Fatalf("GET = %d (ETag %q, want %q) %s", got.Code, got.Header().Get("ETag"), seeded, got.Body.String())
	}
	if body := got.Body.String(); !strings.Contains(body, `"agent":null`) || !strings.Contains(body, `"issues":[`) {
		t.Fatalf("GET body lacks agent null or an issues array: %s", body)
	}
	reply := decodeVideoStudioPlanReply(t, got.Body.Bytes())
	if reply.ETag != seeded || reply.Plan.Status != storyplan.StatusReview || reply.Profile.Provider != "minimax" || !reply.Availability.Enabled || len(reply.Issues) == 0 {
		t.Fatalf("GET reply = %+v", reply)
	}

	edited := reply.Plan
	edited.ID = "plan_forged"
	edited.Scenes[0].Title = "Harbour at dawn"
	edited.Scenes[0].State = storyplan.SceneFailed
	edited.Scenes[0].ClipAssetID = "forged"
	body, err := json.Marshal(edited)
	if err != nil {
		t.Fatal(err)
	}
	if w := videoStudioAPIRequest(s, writeToken, http.MethodPut, path, bytes.NewReader(body), nil); w.Code != http.StatusPreconditionRequired || !strings.Contains(w.Body.String(), "precondition_required") {
		t.Fatalf("PUT without If-Match = %d %s", w.Code, w.Body.String())
	}
	if w := videoStudioAPIRequest(s, readToken, http.MethodPut, path, bytes.NewReader(body), map[string]string{"If-Match": seeded}); w.Code != http.StatusForbidden {
		t.Fatalf("PUT with the read token = %d", w.Code)
	}
	saved := videoStudioAPIRequest(s, writeToken, http.MethodPut, path, bytes.NewReader(body), map[string]string{"If-Match": seeded})
	if saved.Code != http.StatusOK {
		t.Fatalf("PUT = %d %s", saved.Code, saved.Body.String())
	}
	savedReply := decodeVideoStudioPlanReply(t, saved.Body.Bytes())
	version := saved.Header().Get("ETag")
	if version == "" || version == seeded || savedReply.ETag != version {
		t.Fatalf("PUT ETag %q (reply %q, before %q)", version, savedReply.ETag, seeded)
	}
	scene := savedReply.Plan.Scenes[0]
	if savedReply.Plan.ID != "plan_fixture" || savedReply.Plan.Status != storyplan.StatusReview || scene.Title != "Harbour at dawn" || scene.State != storyplan.SceneReady || scene.ClipAssetID != "clip-1" {
		t.Fatalf("PUT kept the wrong fields: %+v", savedReply.Plan)
	}
	svc, _, _ := videoStudioPlanDeps(t, s)
	stored, _, storedVersion, err := readVideoStudioPlan(context.Background(), svc, projectID)
	if err != nil || storedVersion != version || stored.Status != storyplan.StatusReview {
		t.Fatalf("stored after PUT: status %q, version %q (reply %q), err %v", stored.Status, storedVersion, version, err)
	}
	if w := videoStudioAPIRequest(s, writeToken, http.MethodPut, path, bytes.NewReader(body), map[string]string{"If-Match": seeded}); w.Code != http.StatusPreconditionFailed {
		t.Fatalf("stale PUT = %d %s", w.Code, w.Body.String())
	}
	if w := videoStudioAPIRequest(s, readToken, http.MethodGet, path, nil, map[string]string{"If-None-Match": version}); w.Code != http.StatusNotModified || w.Body.Len() != 0 {
		t.Fatalf("GET with the current If-None-Match = %d (%d bytes)", w.Code, w.Body.Len())
	}
	if w := videoStudioAPIRequest(s, readToken, http.MethodGet, path, nil, map[string]string{"If-None-Match": seeded}); w.Code != http.StatusOK {
		t.Fatalf("GET with an old If-None-Match = %d", w.Code)
	}
	for _, invalid := range []string{`{`, `{"version":1,"bogus":true}`} {
		if w := videoStudioAPIRequest(s, writeToken, http.MethodPut, path, strings.NewReader(invalid), map[string]string{"If-Match": version}); w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "invalid_plan") {
			t.Fatalf("PUT %s = %d %s", invalid, w.Code, w.Body.String())
		}
	}
	videoStudioPlanConfigure(s, func(cfg *config.Config) { cfg.VideoStudio.ReadOnly = true })
	if w := videoStudioAPIRequest(s, writeToken, http.MethodPut, path, bytes.NewReader(body), map[string]string{"If-Match": version}); w.Code != http.StatusForbidden {
		t.Fatalf("PUT on read-only Video Studio = %d %s", w.Code, w.Body.String())
	}
	if w := videoStudioAPIRequest(s, readToken, http.MethodGet, path, nil, nil); w.Code != http.StatusOK {
		t.Fatalf("read-only Video Studio cannot read the plan: %d", w.Code)
	}
}

func TestVideoStudioPlanAgentRouteRefusals(t *testing.T) {
	s, readToken, writeToken, projectID := videoStudioPlanFixture(t)
	path := "/projects/" + projectID + "/plan/agent"
	create := `{"mode":"create","idea":"A fisherman heads out at dawn","target_seconds":30,"language":"de"}`
	expect := func(name, token, body string, status int, code string) *httptest.ResponseRecorder {
		t.Helper()
		w := videoStudioAPIRequest(s, token, http.MethodPost, path, strings.NewReader(body), nil)
		if w.Code != status || !strings.Contains(w.Body.String(), `"`+code+`"`) {
			t.Fatalf("%s = %d %s, want %d %s", name, w.Code, w.Body.String(), status, code)
		}
		return w
	}
	longIdea := strings.Repeat("x", int(storyplan.MaxIdeaChars)+1)
	for _, tc := range []struct {
		name, token, body string
		status            int
		code              string
	}{
		{"read token", readToken, create, http.StatusForbidden, "desktop_scope_required"},
		{"unknown field", writeToken, `{"mode":"create","bogus":1}`, http.StatusBadRequest, "invalid_request"},
		{"unknown mode", writeToken, `{"mode":"run"}`, http.StatusBadRequest, "invalid_mode"},
		{"blank idea", writeToken, `{"mode":"create","idea":"  ","target_seconds":30}`, http.StatusBadRequest, "invalid_idea"},
		{"long idea", writeToken, `{"mode":"create","idea":"` + longIdea + `","target_seconds":30}`, http.StatusBadRequest, "invalid_idea"},
		{"short target", writeToken, `{"mode":"create","idea":"Dawn","target_seconds":4}`, http.StatusBadRequest, "invalid_target_seconds"},
		{"long target", writeToken, `{"mode":"create","idea":"Dawn","target_seconds":121}`, http.StatusBadRequest, "invalid_target_seconds"},
		{"bad language", writeToken, `{"mode":"create","idea":"Dawn","target_seconds":30,"language":"de/x"}`, http.StatusBadRequest, "invalid_language"},
		{"revise without scene", writeToken, `{"mode":"revise","note":"Brighter"}`, http.StatusBadRequest, "invalid_scene_id"},
		{"revise without note", writeToken, `{"mode":"revise","scene_id":"s1"}`, http.StatusBadRequest, "invalid_note"},
		{"revise without plan", writeToken, `{"mode":"revise","scene_id":"s1","note":"Brighter"}`, http.StatusNotFound, "plan_not_found"},
	} {
		expect(tc.name, tc.token, tc.body, tc.status, tc.code)
	}
	videoStudioPlanSeed(t, s, projectID, videoStudioPlanFixturePlan(), "")
	expect("create over a plan", writeToken, create, http.StatusConflict, "plan_exists")
	expect("revise an unknown scene", writeToken, `{"mode":"revise","scene_id":"s9","note":"Brighter"}`, http.StatusBadRequest, "scene_not_found")
	videoStudioPlanConfigure(s, func(cfg *config.Config) { cfg.VideoStudio.Plan.Enabled = false })
	expect("plan mode off", writeToken, create, http.StatusConflict, "plan_disabled")
	videoStudioPlanConfigure(s, func(cfg *config.Config) {
		cfg.VideoStudio.Plan.Enabled = true
		cfg.VideoGeneration.ProviderType = "agnes"
	})
	if w := expect("Agnes", writeToken, create, http.StatusConflict, "plan_unsupported_provider"); !strings.Contains(w.Body.String(), `"reason":"provider_images_public_url_only"`) {
		t.Fatalf("unsupported provider without its reason: %s", w.Body.String())
	}
	videoStudioPlanConfigure(s, func(cfg *config.Config) { cfg.VideoStudio.ReadOnly = true })
	expect("read-only Video Studio", writeToken, create, http.StatusForbidden, "video_studio_readonly")
	if w := videoStudioAPIRequest(s, writeToken, http.MethodGet, path, nil, nil); w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET plan/agent = %d", w.Code)
	}
}

func TestVideoStudioPlanAgentStartErrorMapping(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{errVideoStudioPlanAgentBusy, http.StatusConflict, "plan_agent_running"},
		{fmt.Errorf("start: %w", errVideoStudioPlanAgentDailyLimit), http.StatusTooManyRequests, "plan_daily_limit"},
		{errVideoStudioPlanAgentUnavailable, http.StatusServiceUnavailable, "plan_agent_unavailable"},
		{errors.New("disk full"), http.StatusInternalServerError, "plan_agent_start_failed"},
	} {
		w := httptest.NewRecorder()
		writeVideoStudioPlanAgentStartError(w, tc.err)
		if w.Code != tc.status || !strings.Contains(w.Body.String(), `"code":"`+tc.code+`"`) {
			t.Errorf("%v = %d %s, want %d %s", tc.err, w.Code, w.Body.String(), tc.status, tc.code)
		}
	}
}

// Depends on Part C: the fixture has no LLM, so Start refuses synchronously with errVideoStudioPlanAgentUnavailable.
func TestVideoStudioPlanAgentCreateUndoneWhenStartRefuses(t *testing.T) {
	s, _, writeToken, projectID := videoStudioPlanFixture(t)
	path := "/projects/" + projectID + "/plan/agent"
	create := `{"mode":"create","idea":"A fisherman heads out at dawn","target_seconds":30,"language":"de"}`
	if w := videoStudioAPIRequest(s, writeToken, http.MethodPost, path, strings.NewReader(create), nil); w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), "plan_agent_unavailable") {
		t.Fatalf("create without an LLM = %d %s", w.Code, w.Body.String())
	}
	svc, _, _ := videoStudioPlanDeps(t, s)
	ctx := context.Background()
	if _, _, _, err := readVideoStudioPlan(ctx, svc, projectID); !errors.Is(err, errVideoStudioPlanNotFound) {
		t.Fatalf("a refused create left a plan: %v", err)
	}
	kept := videoStudioPlanSeed(t, s, projectID, videoStudioPlanFixturePlan(), "")
	replace := `{"mode":"create","idea":"Something else","target_seconds":20,"replace":true}`
	if w := videoStudioAPIRequest(s, writeToken, http.MethodPost, path, strings.NewReader(replace), nil); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("replace without an LLM = %d %s", w.Code, w.Body.String())
	}
	plan, _, version, err := readVideoStudioPlan(ctx, svc, projectID)
	if err != nil || version != kept || plan.Idea != "A fisherman heads out at dawn" {
		t.Fatalf("a refused replace changed the plan: version %q (kept %q), idea %q, err %v", version, kept, plan.Idea, err)
	}
}

// Depends on Part C: Stop of a project without a job returns false.
func TestVideoStudioPlanAgentStopWithoutJob(t *testing.T) {
	s, readToken, writeToken, projectID := videoStudioPlanFixture(t)
	path := "/projects/" + projectID + "/plan/agent/stop"
	if w := videoStudioAPIRequest(s, readToken, http.MethodPost, path, nil, nil); w.Code != http.StatusForbidden {
		t.Fatalf("stop with the read token = %d", w.Code)
	}
	if w := videoStudioAPIRequest(s, writeToken, http.MethodPost, path, nil, nil); w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != `{"stopped":false}` {
		t.Fatalf("stop without a job = %d %s", w.Code, w.Body.String())
	}
	if w := videoStudioAPIRequest(s, writeToken, http.MethodGet, path, nil, nil); w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET plan/agent/stop = %d", w.Code)
	}
}

// Depends on Part C (C4 job and its vsp* test helpers): while a planning job runs, GET reports it, a user save is
// refused and the stop route ends the job.
func TestVideoStudioPlanAgentRunningRefusesPut(t *testing.T) {
	reached := make(chan struct{}, 1)
	client := &vspScriptClient{
		turns: map[string][]vspTurn{agent.VideoStudioPlanPhaseOutline: {{block: true}}},
		onBlock: func() {
			select {
			case reached <- struct{}{}:
			default:
			}
		},
	}
	s, writeToken, projectID := vspAgentEnv(t, vspScriptNativeModel, client)
	readToken, _, err := s.TokenManager.Create("video studio plan read fixture", []string{desktopScopeRead}, nil)
	if err != nil {
		t.Fatal(err)
	}
	m := s.videoStudioPlanAgents()
	t.Cleanup(func() { m.Stop(projectID) })
	if _, err := m.Start(vspCreateRequest(projectID)); err != nil {
		t.Fatal(err)
	}
	select {
	case <-reached:
	case <-time.After(60 * time.Second):
		t.Fatal("the model never got the outline request")
	}
	path := "/projects/" + projectID + "/plan"
	got := videoStudioAPIRequest(s, readToken, http.MethodGet, path, nil, nil)
	if got.Code != http.StatusOK {
		t.Fatalf("GET while the agent runs = %d %s", got.Code, got.Body.String())
	}
	reply := decodeVideoStudioPlanReply(t, got.Body.Bytes())
	if reply.Plan.Status != storyplan.StatusPlanning || reply.Agent == nil || reply.Agent.State != videoStudioPlanAgentRunning {
		t.Fatalf("GET while the agent runs = %s", got.Body.String())
	}
	body, err := json.Marshal(reply.Plan)
	if err != nil {
		t.Fatal(err)
	}
	if w := videoStudioAPIRequest(s, writeToken, http.MethodPut, path, bytes.NewReader(body), map[string]string{"If-Match": reply.ETag}); w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), `"plan_agent_running"`) {
		t.Fatalf("PUT while the agent runs = %d %s", w.Code, w.Body.String())
	}
	if w := videoStudioAPIRequest(s, writeToken, http.MethodPost, path+"/agent/stop", nil, nil); w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != `{"stopped":true}` {
		t.Fatalf("stop while the agent runs = %d %s", w.Code, w.Body.String())
	}
	if status := vspWait(t, m, projectID); status.State != videoStudioPlanAgentStopped {
		t.Fatalf("status after the stop route = %+v", status)
	}
}
