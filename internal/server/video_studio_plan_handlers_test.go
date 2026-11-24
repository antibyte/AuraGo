package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

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
