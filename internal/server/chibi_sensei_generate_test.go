package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sashabaranov/go-openai"

	"aurago/internal/llm"
)

// chibiFakeLLM is the generator client of the tests: it answers from respond (or with an empty rows list), records
// every request and, when block is set, waits until the request context ends.
type chibiFakeLLM struct {
	mu       sync.Mutex
	requests []openai.ChatCompletionRequest
	respond  func(call int, req openai.ChatCompletionRequest) string
	block    bool
}

func (f *chibiFakeLLM) CreateChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	f.mu.Lock()
	f.requests = append(f.requests, req)
	call := len(f.requests)
	block, respond := f.block, f.respond
	f.mu.Unlock()
	if block {
		<-ctx.Done()
		return openai.ChatCompletionResponse{}, ctx.Err()
	}
	content := `{"rows":[]}`
	if respond != nil {
		content = respond(call, req)
	}
	return openai.ChatCompletionResponse{Model: req.Model,
		Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Content: content}, FinishReason: openai.FinishReasonStop}},
		Usage:   openai.Usage{PromptTokens: 10, CompletionTokens: 10}}, nil
}

func (f *chibiFakeLLM) CreateChatCompletionStream(context.Context, openai.ChatCompletionRequest) (llm.CompletionStream, error) {
	return nil, nil
}

func (f *chibiFakeLLM) seen() []openai.ChatCompletionRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]openai.ChatCompletionRequest(nil), f.requests...)
}

func chibiRowsAnswer(rows ...[2]string) string {
	items := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		items = append(items, map[string]any{"messages": []map[string]string{{"role": "user", "content": r[0]}, {"role": "assistant", "content": r[1]}}})
	}
	raw, _ := json.Marshal(map[string]any{"rows": items})
	return string(raw)
}

// chibiUseFake points the generator at fake and gives the fixture a model name and structured outputs.
func chibiUseFake(s *Server, fake *chibiFakeLLM) {
	s.Cfg.LLM.Model = "main-model"
	s.Cfg.LLM.StructuredOutputs = true
	s.chibiSenseiLLMOverride = fake
}

func chibiRowCount(t *testing.T, s *Server, token, pid, query string) float64 {
	t.Helper()
	w := chibiDo(t, s, token, http.MethodGet, "/projects/"+pid+"/rows?limit=1"+query, nil)
	chibiWant(t, w, http.StatusOK, "")
	return chibiBody(t, w)["total"].(float64)
}

func TestChibiSenseiGenerateJob(t *testing.T) {
	s, _, token := chibiAPIFixture(t)
	pid := chibiNewProject(t, s, token, map[string]any{"name": "Bot", "recipe": "assistant", "goal_text": "Answer shop questions"})
	base := "/projects/" + pid
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, base+"/rows", map[string]any{"rows": []map[string]any{
		{"messages": []map[string]string{{"role": "user", "content": "existing question"}, {"role": "assistant", "content": "existing long answer"}}}}}),
		http.StatusCreated, "")

	// Without a model the buttons are off and the route answers 503.
	meta := chibiBody(t, chibiDo(t, s, token, http.MethodGet, "/meta", nil))["generator"].(map[string]any)
	if meta["available"] != false {
		t.Fatalf("generator.available = %v without a model", meta["available"])
	}
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, base+"/generate", map[string]any{"count": 5}), http.StatusServiceUnavailable, "provider_unavailable")

	fake := &chibiFakeLLM{}
	chibiUseFake(s, fake)
	if meta = chibiBody(t, chibiDo(t, s, token, http.MethodGet, "/meta", nil))["generator"].(map[string]any); meta["available"] != true || meta["model"] != "main-model" {
		t.Fatalf("generator = %v", meta)
	}
	pair := func(i int) [2]string {
		return [2]string{fmt.Sprintf("new question %d", i), fmt.Sprintf("a good answer number %d", i)}
	}
	first := []map[string]any{}
	add := func(m []map[string]string) { first = append(first, map[string]any{"messages": m}) }
	add([]map[string]string{{"role": "user", "content": "existing question"}, {"role": "assistant", "content": "existing long answer"}}) // duplicate
	add([]map[string]string{{"role": "user", "content": "q"}, {"role": "assistant", "content": "a"}, {"role": "user", "content": "q2"}}) // ends with user
	for i := 2; i < 10; i++ {
		add([]map[string]string{{"role": "user", "content": pair(i)[0]}, {"role": "assistant", "content": pair(i)[1]}})
	}
	firstRaw, _ := json.Marshal(map[string]any{"rows": first})
	fake.respond = func(call int, _ openai.ChatCompletionRequest) string {
		switch call {
		case 1:
			return string(firstRaw)
		case 2:
			return chibiRowsAnswer(pair(20), pair(21))
		}
		return `{"rows":[]}`
	}

	chibiWant(t, chibiDo(t, s, token, http.MethodPost, base+"/generate", map[string]any{"count": 501}), http.StatusUnprocessableEntity, "validation_failed")
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, base+"/generate", map[string]any{"count": 5, "language": "xx_YY"}), http.StatusUnprocessableEntity, "validation_failed")
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, base+"/generate", map[string]any{"count": 5, "seed_row_ids": []int64{99999}}), http.StatusUnprocessableEntity, "validation_failed")

	w := chibiDo(t, s, token, http.MethodPost, base+"/generate", map[string]any{"count": 12, "instructions": "Be friendly."})
	chibiWant(t, w, http.StatusAccepted, "")
	job := waitChibiJob(t, s, token, chibiBody(t, w)["job"].(map[string]any)["id"].(string))
	if job.State != "succeeded" || job.Result.RowsAdded != 10 || job.Result.RowsSkipped != 2 || job.Result.BatchID == "" {
		t.Fatalf("job = %+v", job)
	}
	reqs := fake.seen()
	if len(reqs) < 2 || !strings.Contains(reqs[0].Messages[1].Content, "Write 10 new examples") || !strings.Contains(reqs[1].Messages[1].Content, "Write 4 new examples") {
		t.Fatalf("requests = %d, first user message %q", len(reqs), reqs[0].Messages[1].Content)
	}
	if rf := reqs[0].ResponseFormat; rf == nil || rf.Type != openai.ChatCompletionResponseFormatTypeJSONSchema || rf.JSONSchema == nil || rf.JSONSchema.Name != "chibi_sensei_answer" || !rf.JSONSchema.Strict {
		t.Fatalf("response format = %+v", reqs[0].ResponseFormat)
	}
	if n := chibiRowCount(t, s, token, pid, "&source=generated"); n != 10 {
		t.Fatalf("generated rows = %v", n)
	}
	hist := chibiBody(t, chibiDo(t, s, token, http.MethodGet, base+"/history", nil))["batches"].([]any)
	if first := hist[0].(map[string]any); first["kind"] != "generate" || first["rows"] != float64(10) {
		t.Fatalf("history[0] = %v", hist[0])
	}
	undo := chibiDo(t, s, token, http.MethodPost, base+"/history/"+job.Result.BatchID+"/undo", nil)
	chibiWant(t, undo, http.StatusOK, "")
	if n := chibiRowCount(t, s, token, pid, ""); n != 1 {
		t.Fatalf("rows after undo = %v", n)
	}
}

func TestChibiSenseiGenerateDailyLimit(t *testing.T) {
	s, _, token := chibiAPIFixture(t)
	s.Cfg.ChibiSensei.Generator.MaxDailyRows = 100
	pid := chibiNewProject(t, s, token, map[string]any{"name": "Bot", "recipe": "assistant"})
	fake := &chibiFakeLLM{}
	chibiUseFake(s, fake)
	fake.respond = func(call int, _ openai.ChatCompletionRequest) string {
		rows := make([][2]string, 0, 10)
		for i := 0; i < 10; i++ {
			rows = append(rows, [2]string{fmt.Sprintf("question %d-%d", call, i), fmt.Sprintf("answer %d-%d long enough", call, i)})
		}
		return chibiRowsAnswer(rows...)
	}
	base := "/projects/" + pid
	w := chibiDo(t, s, token, http.MethodPost, base+"/generate", map[string]any{"count": 100})
	chibiWant(t, w, http.StatusAccepted, "")
	job := waitChibiJob(t, s, token, chibiBody(t, w)["job"].(map[string]any)["id"].(string))
	if job.State != "succeeded" || job.Result.RowsAdded != 100 {
		t.Fatalf("job = %+v", job)
	}
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, base+"/generate", map[string]any{"count": 10}), http.StatusTooManyRequests, "generator_daily_limit")
}

func TestChibiSenseiGenerateCancel(t *testing.T) {
	s, _, token := chibiAPIFixture(t)
	pid := chibiNewProject(t, s, token, map[string]any{"name": "Bot", "recipe": "assistant"})
	fake := &chibiFakeLLM{block: true}
	chibiUseFake(s, fake)
	w := chibiDo(t, s, token, http.MethodPost, "/projects/"+pid+"/generate", map[string]any{"count": 10})
	chibiWant(t, w, http.StatusAccepted, "")
	id := chibiBody(t, w)["job"].(map[string]any)["id"].(string)
	for deadline := time.Now().Add(3 * time.Second); len(fake.seen()) == 0; time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the job never called the model")
		}
	}
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, "/projects/"+pid+"/generate", map[string]any{"count": 10}), http.StatusConflict, "job_active")
	w = chibiDo(t, s, token, http.MethodPost, "/jobs/"+id+"/cancel", nil)
	chibiWant(t, w, http.StatusOK, "")
	if job := waitChibiJob(t, s, token, id); job.State != "cancelled" {
		t.Fatalf("job state = %s", job.State)
	}
}

func TestChibiSenseiCleanReviewApply(t *testing.T) {
	s, _, token := chibiAPIFixture(t)
	pid := chibiNewProject(t, s, token, map[string]any{"name": "Bot", "recipe": "assistant", "goal_text": "Answer shop questions"})
	base := "/projects/" + pid
	w := chibiDo(t, s, token, http.MethodPost, base+"/rows", map[string]any{"rows": chibiPairs(3, false)})
	chibiWant(t, w, http.StatusCreated, "")
	var ids []int64
	for _, r := range chibiBody(t, w)["rows"].([]any) {
		ids = append(ids, int64(r.(map[string]any)["id"].(float64)))
	}

	fake := &chibiFakeLLM{}
	chibiUseFake(s, fake)
	verdict := func(fixed string) func(int, openai.ChatCompletionRequest) string {
		return func(int, openai.ChatCompletionRequest) string {
			item := func(id int64, action, answer string) map[string]any {
				msgs := []map[string]string{}
				if answer != "" {
					msgs = []map[string]string{{"role": "user", "content": fmt.Sprintf("question %d", id)}, {"role": "assistant", "content": answer}}
				}
				return map[string]any{"id": id, "action": action, "messages": msgs, "text": "", "reason": "because"}
			}
			raw, _ := json.Marshal(map[string]any{"items": []map[string]any{
				item(ids[0], "keep", ""), item(ids[1], "fix", fixed), item(ids[2], "drop", "")}})
			return string(raw)
		}
	}
	fake.respond = verdict("a corrected answer")

	chibiWant(t, chibiDo(t, s, token, http.MethodPost, base+"/clean", map[string]any{"select": map[string]any{"ids": []int64{}}}), http.StatusUnprocessableEntity, "")
	w = chibiDo(t, s, token, http.MethodPost, base+"/clean", map[string]any{"select": map[string]any{"ids": ids}})
	chibiWant(t, w, http.StatusAccepted, "")
	jobID := chibiBody(t, w)["job"].(map[string]any)["id"].(string)
	if job := waitChibiJob(t, s, token, jobID); job.State != "succeeded" || job.Result.Suggestions != 3 {
		t.Fatalf("clean job = %+v", job)
	}
	w = chibiDo(t, s, token, http.MethodGet, "/jobs/"+jobID+"/suggestions?limit=2", nil)
	chibiWant(t, w, http.StatusOK, "")
	got := chibiBody(t, w)
	if got["total"] != float64(3) || len(got["suggestions"].([]any)) != 2 {
		t.Fatalf("suggestions = %s", w.Body.String())
	}
	if first := got["suggestions"].([]any)[1].(map[string]any); first["action"] != "fix" || first["before"] == nil || first["applied"] != false {
		t.Fatalf("suggestion = %v", first)
	}
	chibiWant(t, chibiDo(t, s, token, http.MethodGet, "/jobs/"+jobID+"/suggestions?limit=500", nil), http.StatusUnprocessableEntity, "")

	apply := "/jobs/" + jobID + "/apply"
	chibiWant(t, chibiDo(t, s, token, http.MethodPost, apply, map[string]any{"accept": "some"}), http.StatusUnprocessableEntity, "")
	w = chibiDo(t, s, token, http.MethodPost, apply, map[string]any{"accept": "all"})
	chibiWant(t, w, http.StatusOK, "")
	if body := chibiBody(t, w); body["applied"] != float64(1) || body["batch_id"] == "" {
		t.Fatalf("apply all = %s", w.Body.String())
	}
	row := func(id int64) *httptestRow { return chibiGetRow(t, s, token, pid, id) }
	if r := row(ids[1]); r.Source != "cleaned" || r.Messages[1].Content != "a corrected answer" {
		t.Fatalf("row 2 = %+v", r)
	}
	if row(ids[2]) == nil {
		t.Fatal("accept all applied a drop")
	}
	w = chibiDo(t, s, token, http.MethodPost, apply, map[string]any{"accept": []int64{ids[2]}})
	chibiWant(t, w, http.StatusOK, "")
	if chibiBody(t, w)["applied"] != float64(1) || row(ids[2]) != nil {
		t.Fatalf("apply [drop] = %s", w.Body.String())
	}

	// A row edited after the suggestion is never overwritten.
	fake.respond = verdict("another correction")
	w = chibiDo(t, s, token, http.MethodPost, base+"/clean", map[string]any{"select": map[string]any{"ids": ids[:2]}})
	chibiWant(t, w, http.StatusAccepted, "")
	job2 := chibiBody(t, w)["job"].(map[string]any)["id"].(string)
	if job := waitChibiJob(t, s, token, job2); job.State != "succeeded" {
		t.Fatalf("second clean job = %+v", job)
	}
	edit := row(ids[1])
	put := map[string]any{"messages": []map[string]string{{"role": "user", "content": "edited question"}, {"role": "assistant", "content": "edited by hand"}}}
	chibiWant(t, chibiDo(t, s, token, http.MethodPut, fmt.Sprintf("%s/rows/%d", base, ids[1]), put, "If-Match", fmt.Sprint(edit.Rev)), http.StatusOK, "")
	w = chibiDo(t, s, token, http.MethodPost, "/jobs/"+job2+"/apply", map[string]any{"accept": []int64{ids[1]}})
	chibiWant(t, w, http.StatusOK, "")
	if body := chibiBody(t, w); body["applied"] != float64(0) || body["skipped"] != float64(1) {
		t.Fatalf("stale apply = %s", w.Body.String())
	}
	if r := row(ids[1]); r.Messages[1].Content != "edited by hand" {
		t.Fatalf("the newer edit was overwritten: %+v", r)
	}
}

type httptestRow struct {
	Source   string `json:"source"`
	Rev      int64  `json:"rev"`
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
}

// chibiGetRow returns a row, or nil when it does not exist.
func chibiGetRow(t *testing.T, s *Server, token, pid string, id int64) *httptestRow {
	t.Helper()
	w := chibiDo(t, s, token, http.MethodGet, fmt.Sprintf("/projects/%s/rows/%d", pid, id), nil)
	if w.Code == http.StatusNotFound {
		return nil
	}
	chibiWant(t, w, http.StatusOK, "")
	var body struct {
		Row httptestRow `json:"row"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return &body.Row
}
