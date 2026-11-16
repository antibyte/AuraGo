package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
)

func factsHas(answer map[string]any, key string) bool {
	_, has := answer[key]
	return has
}

// GET {flow} and the save answer carry "tested" and "changes".
func TestFlowDraftFactsTestedAndChanges(t *testing.T) {
	s, token := newFlowsTestServer(t)
	ctx := context.Background()
	rec := createTestFlow(t, s, greetFlowJSON)
	path := "/api/desktop/flows/" + rec.ID
	get := func() map[string]any {
		t.Helper()
		w := flowsCall(t, s, http.MethodGet, path, token, "")
		if w.Code != http.StatusOK {
			t.Fatalf("GET = %d %s", w.Code, w.Body.String())
		}
		return flowsBody(t, w)
	}
	save := func(doc string, revision int) map[string]any {
		t.Helper()
		w := flowsCall(t, s, http.MethodPut, path, token, fmt.Sprintf(`{"doc":%s,"base_revision":%d}`, doc, revision))
		if w.Code != http.StatusOK {
			t.Fatalf("PUT = %d %s", w.Code, w.Body.String())
		}
		return flowsBody(t, w)
	}

	one := get()
	if one["tested"] != false || factsHas(one, "changes") || factsHas(one, "next_run") {
		t.Fatalf("a new flow: tested=%v changes=%v next_run=%v", one["tested"], one["changes"], one["next_run"])
	}

	// A successful simulation of the draft marks it, and a reload (a new GET) still shows it.
	w := flowsCall(t, s, http.MethodPost, path+"/simulate", token, fmt.Sprintf(`{"base_revision":%d}`, rec.DraftRevision))
	if w.Code != http.StatusOK {
		t.Fatalf("simulate = %d %s", w.Code, w.Body.String())
	}
	if one = get(); one["tested"] != true {
		t.Fatalf("after a simulation tested = %v, want true", one["tested"])
	}

	// A save makes a new revision; the flow was never published, so there is no count.
	saved := save(greetFlowJSON, rec.DraftRevision)
	revision := int(saved["draft_revision"].(float64))
	if saved["tested"] != false || factsHas(saved, "changes") {
		t.Fatalf("save of an unpublished flow: tested=%v changes=%v", saved["tested"], saved["changes"])
	}
	if one = get(); one["tested"] != false {
		t.Fatalf("after a save tested = %v, want false", one["tested"])
	}

	// Published and then edited: the count equals the publish preview's diff.
	if _, _, err := s.Flows.Publish(ctx, rec.ID, revision); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if saved = save(strings.Replace(greetFlowJSON, `"label":"Gruss"`, `"label":"Hallo"`, 1), revision); saved["tested"] != false {
		t.Fatalf("save after a publish: tested = %v", saved["tested"])
	}
	changes, ok := saved["changes"].(map[string]any)
	if !ok || changes["steps"] != float64(1) || changes["connections"] != float64(0) {
		t.Fatalf("save answer changes = %v, want 1 step and 0 connections", saved["changes"])
	}
	preview := flowsBody(t, flowsCall(t, s, http.MethodGet, path+"/publish-preview", token, ""))["diff"].(map[string]any)
	steps := preview["added_nodes"].(float64) + preview["changed_nodes"].(float64) + preview["removed_nodes"].(float64)
	if changes["steps"] != steps || changes["connections"] != preview["changed_edges"] {
		t.Fatalf("changes %v differ from the publish preview %v", changes, preview)
	}
	if got := get()["changes"]; !reflect.DeepEqual(got, saved["changes"]) {
		t.Fatalf("GET changes = %v, save answer = %v", got, saved["changes"])
	}
}

// GET {flow} carries "next_run" for an enabled flow with a Date/Time trigger, and not otherwise.
func TestFlowDraftFactsNextRun(t *testing.T) {
	s, token := newFlowsTestServer(t)
	ctx := context.Background()
	at := time.Now().Add(48 * time.Hour).Truncate(time.Second)
	rec := createTestFlow(t, s, timeFlowJSON(false, at))
	path := "/api/desktop/flows/" + rec.ID
	nextRun := func() (map[string]any, bool) {
		t.Helper()
		body := flowsBody(t, flowsCall(t, s, http.MethodGet, path, token, ""))
		value, has := body["next_run"]
		if !has {
			return nil, false
		}
		return value.(map[string]any), true
	}
	if _, has := nextRun(); has {
		t.Fatal("an unpublished flow has a next run")
	}
	if _, _, err := s.Flows.Publish(ctx, rec.ID, rec.DraftRevision); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if _, has := nextRun(); has {
		t.Fatal("a flow that is switched off has a next run")
	}
	if err := s.Flows.SetEnabled(ctx, rec.ID, true); err != nil {
		t.Fatalf("SetEnabled: %v", err)
	}
	next, has := nextRun()
	if !has {
		t.Fatal("an enabled flow with a Date/Time trigger has no next run")
	}
	got, err := time.Parse(time.RFC3339, next["at"].(string))
	if err != nil || !got.Equal(at) || next["node"] != "n_cccccccc" {
		t.Fatalf("next_run = %v (%v), want %s at n_cccccccc", next, err, at.Format(time.RFC3339))
	}
	if err := s.Flows.SetEnabled(ctx, rec.ID, false); err != nil {
		t.Fatal(err)
	}
	if _, has := nextRun(); has {
		t.Fatal("a paused flow has a next run")
	}
}

// The block document answers with "tested" too, never with "next_run", and a block
// simulation marks the draft revision.
func TestBlockDraftFactsTested(t *testing.T) {
	s, token := newFlowsTestServer(t)
	ctx := context.Background()
	block, err := s.Flows.CreateBlock(ctx, "Leer")
	if err != nil {
		t.Fatal(err)
	}
	doc, _ := block.Draft.Clone()
	delete(doc.Nodes[1].Params, "values")
	revision, issues, err := s.Flows.SaveBlockDraft(ctx, block.ID, doc, block.DraftRevision)
	if err != nil {
		t.Fatalf("save: %v %+v", err, issues)
	}
	path := flowBlocksPath + "/" + block.ID
	get := func() map[string]any {
		t.Helper()
		w := flowBlockCall(t, s, http.MethodGet, path, token, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("GET = %d %s", w.Code, w.Body.String())
		}
		return flowsBody(t, w)
	}
	if one := get(); one["tested"] != false || factsHas(one, "next_run") || factsHas(one, "changes") {
		t.Fatalf("a new block draft: %v", one)
	}
	if w := flowBlockCall(t, s, http.MethodPost, path+"/simulate", token, map[string]any{"base_revision": revision}); w.Code != http.StatusOK {
		t.Fatalf("simulate = %d %s", w.Code, w.Body.String())
	}
	if one := get(); one["tested"] != true {
		t.Fatalf("after a simulation tested = %v, want true", one["tested"])
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	w := flowBlockCall(t, s, http.MethodPut, path, token, map[string]any{"doc": json.RawMessage(raw), "base_revision": revision})
	if w.Code != http.StatusOK {
		t.Fatalf("PUT = %d %s", w.Code, w.Body.String())
	}
	if saved := flowsBody(t, w); saved["tested"] != false || factsHas(saved, "next_run") {
		t.Fatalf("block save answer = %v", saved)
	}
}
