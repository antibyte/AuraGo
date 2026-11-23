package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aurago/internal/chibisensei"
)

const chibiFakeID = "11111111-1111-4111-8111-111111111111"

// chibiDo sends a JSON request to the Chibi Sensei router with a bearer token. body is marshalled
// unless it is nil, a string (sent as is) or a []byte. headers are name, value pairs.
func chibiDo(t *testing.T, s *Server, token, method, path string, body any, headers ...string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	switch b := body.(type) {
	case nil:
		reader = bytes.NewReader(nil)
	case string:
		reader = bytes.NewReader([]byte(b))
	case []byte:
		reader = bytes.NewReader(b)
	default:
		raw, err := json.Marshal(b)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	}
	h := map[string]string{}
	for i := 0; i+1 < len(headers); i += 2 {
		h[headers[i]] = headers[i+1]
	}
	return chibiRequest(s, token, method, path, reader, h)
}

// chibiBody decodes a JSON answer into a generic map.
func chibiBody(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("answer is not a JSON object: %v\n%s", err, w.Body.String())
	}
	return m
}

// chibiWant checks the status and, when code is not empty, the error code of an answer.
func chibiWant(t *testing.T, w *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if w.Code != status {
		t.Fatalf("status = %d, want %d: %s", w.Code, status, w.Body.String())
	}
	if code != "" && chibiErrorCode(t, w) != code {
		t.Fatalf("code = %q, want %q: %s", chibiErrorCode(t, w), code, w.Body.String())
	}
}

// chibiNewProject creates a project and returns its ID.
func chibiNewProject(t *testing.T, s *Server, token string, body map[string]any) string {
	t.Helper()
	w := chibiDo(t, s, token, http.MethodPost, "/projects", body)
	chibiWant(t, w, http.StatusCreated, "")
	return chibiBody(t, w)["project"].(map[string]any)["id"].(string)
}

// chibiPairs builds n assistant rows; a long user text gives about 180 tokens.
func chibiPairs(n int, padded bool) []map[string]any {
	rows := make([]map[string]any, 0, n)
	for i := 0; i < n; i++ {
		q := fmt.Sprintf("question %d", i)
		if padded {
			q += " " + strings.Repeat("alpha ", 100)
		}
		rows = append(rows, map[string]any{"messages": []map[string]string{
			{"role": "user", "content": q}, {"role": "assistant", "content": fmt.Sprintf("answer %d", i)}}})
	}
	return rows
}

// waitChibiJob polls GET /jobs/{id} every 20 ms for up to 5 s until the job is terminal.
func waitChibiJob(t *testing.T, s *Server, token, id string) chibisensei.Job {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		w := chibiDo(t, s, token, http.MethodGet, "/jobs/"+id, nil)
		chibiWant(t, w, http.StatusOK, "")
		var body struct {
			Job chibisensei.Job `json:"job"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		switch body.Job.State {
		case chibisensei.JobSucceeded, chibisensei.JobFailed, chibisensei.JobCancelled:
			return body.Job
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("job did not finish")
	return chibisensei.Job{}
}

func TestChibiSenseiReadOnlyRefusesWrites(t *testing.T) {
	s, _, token := chibiAPIFixture(t)
	pid := chibiNewProject(t, s, token, map[string]any{"name": "Bot", "recipe": "assistant"})
	w := chibiDo(t, s, token, http.MethodPost, "/projects/"+pid+"/rows", map[string]any{"rows": chibiPairs(1, false)})
	chibiWant(t, w, http.StatusCreated, "")

	cfg := *s.ConfigSnapshot()
	cfg.VirtualDesktop.ReadOnly = true
	s.replaceConfigSnapshot(&cfg)

	base := "/projects/" + pid
	writes := []struct{ method, path, body string }{
		{http.MethodPost, "/projects", `{"name":"x","recipe":"assistant"}`},
		{http.MethodPatch, base, `{"name":"y"}`},
		{http.MethodPost, base + "/rows", `{"rows":[]}`},
		{http.MethodPut, base + "/rows/1", `{}`},
		{http.MethodPost, base + "/rows/bulk", `{}`},
		{http.MethodPost, base + "/rows/dedupe", ``},
		{http.MethodPost, base + "/history/" + chibiFakeID + "/undo", ``},
		{http.MethodPost, base + "/checks/run", ``},
		{http.MethodPost, base + "/split/reseed", ``},
		{http.MethodPost, base + "/documents", `{"workspace_path":"a.md"}`},
		{http.MethodPost, base + "/imports", `{}`},
		{http.MethodPost, base + "/exports", `{}`},
		{http.MethodPost, base + "/generate", `{}`},
		{http.MethodPost, base + "/clean", `{}`},
		{http.MethodPost, "/jobs/" + chibiFakeID + "/apply", `{}`},
	}
	for _, c := range writes {
		var body any
		if c.body != "" {
			body = c.body
		}
		chibiWant(t, chibiDo(t, s, token, c.method, c.path, body, "If-Match", "1"), http.StatusForbidden, "desktop_readonly")
	}
	for _, p := range []string{"/projects", base, base + "/rows", base + "/checks", base + "/split", base + "/plan"} {
		chibiWant(t, chibiDo(t, s, token, http.MethodGet, p, nil), http.StatusOK, "")
	}
	// A cancel is a stop operation: the read-only desktop does not refuse it.
	w = chibiDo(t, s, token, http.MethodPost, "/jobs/"+chibiFakeID+"/cancel", nil)
	chibiWant(t, w, http.StatusNotFound, "job_not_found")
}

func TestChibiSenseiProjectFlow(t *testing.T) {
	s, _, token := chibiAPIFixture(t)
	w := chibiDo(t, s, token, http.MethodPost, "/projects", map[string]any{
		"name": "Bot", "recipe": "specialist", "task_kind": "classify", "labels": []string{"pos", "neg"}, "goal_text": "Sort reviews"})
	chibiWant(t, w, http.StatusCreated, "")
	project := chibiBody(t, w)["project"].(map[string]any)
	pid := project["id"].(string)
	if project["profile_id"] != "gemma3-270m" {
		t.Fatalf("profile_id = %v", project["profile_id"])
	}
	if sug := project["suggested_profile"].(map[string]any); sug["reason_key"] != "chibi_sensei.suggest.specialist_short_inputs" {
		t.Fatalf("suggested_profile = %v", sug)
	}
	path := "/projects/" + pid

	chibiWant(t, chibiDo(t, s, token, http.MethodPatch, path, `{"name":"Bot"}`), http.StatusPreconditionRequired, "precondition_required")
	chibiWant(t, chibiDo(t, s, token, http.MethodPatch, path, `{"name":"Bot"}`, "If-Match", "soon"), http.StatusBadRequest, "precondition_invalid")
	chibiWant(t, chibiDo(t, s, token, http.MethodPatch, path, `{"goal_text":"x"}`, "If-Match", "5"), http.StatusPreconditionFailed, "conflict")
	chibiWant(t, chibiDo(t, s, token, http.MethodPatch, path, `{"profile_id":"nope"}`, "If-Match", `"1"`), http.StatusUnprocessableEntity, "profile_unknown")
	chibiWant(t, chibiDo(t, s, token, http.MethodPatch, path, `{"profile_id":"qwen35-0p8b","overrides":{"method":"full"}}`, "If-Match", "1"),
		http.StatusUnprocessableEntity, "validation_failed")
	w = chibiDo(t, s, token, http.MethodPatch, path, `{"goal_text":"Sort all the reviews"}`, "If-Match", "1")
	chibiWant(t, w, http.StatusOK, "")
	if rev := chibiBody(t, w)["project"].(map[string]any)["rev"]; rev != float64(2) {
		t.Fatalf("rev after patch = %v", rev)
	}

	chibiWant(t, chibiDo(t, s, token, http.MethodPost, path+"/delete", `{"confirm_name":"Bot"}`), http.StatusConflict, "not_archived")
	w = chibiDo(t, s, token, http.MethodPost, path+"/archive", `{"archived":true}`)
	chibiWant(t, w, http.StatusOK, "")
	if chibiBody(t, w)["project"].(map[string]any)["archived"] != true {
		t.Fatalf("archive: %s", w.Body.String())
	}
	w = chibiDo(t, s, token, http.MethodGet, "/projects?archived=1", nil)
	if list := chibiBody(t, w)["projects"].([]any); len(list) != 1 {
		t.Fatalf("archived list = %v", list)
	}
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, path+"/delete", `{"confirm_name":"x"}`), http.StatusUnprocessableEntity, "validation_failed")
	if w := chibiDo(t, s, token, http.MethodPost, path+"/delete", `{"confirm_name":"Bot"}`); w.Code != http.StatusNoContent {
		t.Fatalf("delete = %d %s", w.Code, w.Body.String())
	}
	chibiWant(t, chibiDo(t, s, token, http.MethodGet, path, nil), http.StatusNotFound, "not_found")
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, "/projects", `{"name":"x","recipe":"assistant","bogus":1}`), http.StatusBadRequest, "invalid_request")
	chibiWant(t, chibiDo(t, s, token, http.MethodGet, "/projects/not-an-id", nil), http.StatusNotFound, "not_found")
	chibiWant(t, chibiDo(t, s, token, http.MethodDelete, "/projects", nil), http.StatusMethodNotAllowed, "method_not_allowed")
}

func TestChibiSenseiRowsBulkAndUndo(t *testing.T) {
	s, _, token := chibiAPIFixture(t)
	pid := chibiNewProject(t, s, token, map[string]any{"name": "Bot", "recipe": "assistant"})
	rows := "/projects/" + pid + "/rows"

	w := chibiDo(t, s, token, http.MethodPost, rows, map[string]any{"rows": chibiPairs(60, false)})
	chibiWant(t, w, http.StatusCreated, "")
	created := chibiBody(t, w)
	if created["data_rev"] != float64(1) || len(created["rows"].([]any)) != 60 {
		t.Fatalf("add 60 rows: data_rev %v, rows %d", created["data_rev"], len(created["rows"].([]any)))
	}
	ids := make([]int64, 0, 60)
	for _, r := range created["rows"].([]any) {
		ids = append(ids, int64(r.(map[string]any)["id"].(float64)))
	}
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, rows, map[string]any{"rows": chibiPairs(201, false)}), http.StatusUnprocessableEntity, "validation_failed")

	one := fmt.Sprintf("%s/%d", rows, ids[0])
	chibiWant(t, chibiDo(t, s, token, http.MethodPut, one, map[string]any{"messages": chibiPairs(1, false)[0]["messages"]}, "If-Match", "9"),
		http.StatusPreconditionFailed, "conflict")
	chibiWant(t, chibiDo(t, s, token, http.MethodPut, one, `{}`), http.StatusPreconditionRequired, "precondition_required")
	chibiWant(t, chibiDo(t, s, token, http.MethodGet, rows+"/0", nil), http.StatusNotFound, "not_found")

	w = chibiDo(t, s, token, http.MethodGet, rows+"?limit=500", nil)
	chibiWant(t, w, http.StatusOK, "")
	if limit := chibiBody(t, w)["limit"]; limit != float64(200) {
		t.Fatalf("limit = %v, want 200", limit)
	}
	chibiWant(t, chibiDo(t, s, token, http.MethodGet, rows+"?flag=duplicates&sort=est_tokens&dir=desc", nil), http.StatusOK, "")
	chibiWant(t, chibiDo(t, s, token, http.MethodGet, rows+"?flag=bogus", nil), http.StatusUnprocessableEntity, "validation_failed")

	sel := map[string]any{"ids": ids}
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, rows+"/bulk", map[string]any{"select": sel, "action": "delete"}),
		http.StatusPreconditionRequired, "confirmation_required")
	w = chibiDo(t, s, token, http.MethodPost, rows+"/delete-preview", map[string]any{"select": sel})
	chibiWant(t, w, http.StatusOK, "")
	preview := chibiBody(t, w)
	token60, _ := preview["token"].(string)
	if preview["count"] != float64(60) || token60 == "" || preview["expires_in_s"] != float64(300) {
		t.Fatalf("delete preview = %v", preview)
	}
	w = chibiDo(t, s, token, http.MethodPost, rows+"/bulk", map[string]any{"select": sel, "action": "delete", "token": token60})
	chibiWant(t, w, http.StatusOK, "")
	if affected := chibiBody(t, w)["affected"]; affected != float64(60) {
		t.Fatalf("bulk delete: %s", w.Body.String())
	}
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, rows+"/bulk", map[string]any{"select": sel, "action": "delete", "token": token60}),
		http.StatusConflict, "confirmation_invalid")

	w = chibiDo(t, s, token, http.MethodGet, "/projects/"+pid+"/history", nil)
	chibiWant(t, w, http.StatusOK, "")
	batches := chibiBody(t, w)["batches"].([]any)
	first := batches[0].(map[string]any)
	if first["kind"] != "bulk_delete" {
		t.Fatalf("history[0] = %v", first)
	}
	undo := "/projects/" + pid + "/history/" + first["batch_id"].(string) + "/undo"
	w = chibiDo(t, s, token, http.MethodPost, undo, nil)
	chibiWant(t, w, http.StatusOK, "")
	if restored := chibiBody(t, w)["restored"]; restored != float64(60) {
		t.Fatalf("undo: %s", w.Body.String())
	}
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, undo, nil), http.StatusConflict, "already_undone")

	// A dedupe of unique rows removes nothing; a duplicate is removed once.
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, rows, map[string]any{"rows": chibiPairs(1, false)}), http.StatusCreated, "")
	w = chibiDo(t, s, token, http.MethodPost, rows+"/dedupe", nil)
	chibiWant(t, w, http.StatusOK, "")
	if removed := chibiBody(t, w)["removed"]; removed != float64(1) {
		t.Fatalf("dedupe: %s", w.Body.String())
	}
}

func TestChibiSenseiChecksSplitPlan(t *testing.T) {
	s, _, token := chibiAPIFixture(t)
	pid := chibiNewProject(t, s, token, map[string]any{"name": "Bot", "recipe": "assistant"})
	base := "/projects/" + pid
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, base+"/rows", map[string]any{"rows": chibiPairs(40, true)}), http.StatusCreated, "")

	w := chibiDo(t, s, token, http.MethodPost, base+"/checks/run", nil)
	chibiWant(t, w, http.StatusOK, "")
	result := chibiBody(t, w)
	if result["ready"] != true {
		t.Fatalf("ready = %v: %s", result["ready"], w.Body.String())
	}
	level := ""
	for _, c := range result["checks"].([]any) {
		if c := c.(map[string]any); c["id"] == "count_min" {
			level, _ = c["level"].(string)
		}
	}
	if level != "warn" {
		t.Fatalf("count_min level = %q", level)
	}
	w = chibiDo(t, s, token, http.MethodGet, base+"/checks", nil)
	chibiWant(t, w, http.StatusOK, "")
	if got := chibiBody(t, w); got["stale"] != false || got["result"] == nil {
		t.Fatalf("checks = %s", w.Body.String())
	}
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, base+"/rows", map[string]any{"rows": chibiPairs(1, false)}), http.StatusCreated, "")
	if got := chibiBody(t, chibiDo(t, s, token, http.MethodGet, base+"/checks", nil)); got["stale"] != true {
		t.Fatalf("checks after an add are not stale: %v", got)
	}

	split := chibiBody(t, chibiDo(t, s, token, http.MethodGet, base+"/split", nil))
	counts := split["counts"].(map[string]any)
	if sum := counts["train"].(float64) + counts["val"].(float64) + counts["eval"].(float64) + counts["excluded"].(float64); sum != 41 {
		t.Fatalf("split counts = %v, want 41 rows", counts)
	}
	w = chibiDo(t, s, token, http.MethodPost, base+"/split/reseed", nil)
	chibiWant(t, w, http.StatusOK, "")
	if chibiBody(t, w)["seed"] == split["seed"] {
		t.Fatal("reseed kept the seed")
	}

	w = chibiDo(t, s, token, http.MethodGet, base+"/plan?preset=quick", nil)
	chibiWant(t, w, http.StatusOK, "")
	plan := chibiBody(t, w)
	if plan["trainable"] != true || plan["reason_key"] != "" || plan["max_train_seconds"] == nil ||
		plan["hyper"].(map[string]any)["epochs"] != float64(1) {
		t.Fatalf("plan = %s", w.Body.String())
	}
	chibiWant(t, chibiDo(t, s, token, http.MethodGet, base+"/plan?preset=x", nil), http.StatusUnprocessableEntity, "validation_failed")
}
