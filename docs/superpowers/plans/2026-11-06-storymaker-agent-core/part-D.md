# Part D

Tasks of batch D. Rules, commands and the order are in the plan index.

<!-- group D1 -->

### Task D1: Start, revise, make-silent routes, status

**Batch:** D  **Depends on:** C7
**Files:**
Create `internal/server/storymaker_agent_handlers.go`
Modify `internal/server/storymaker_handlers.go`
Test `internal/server/storymaker_agent_handlers_test.go`

- [ ] **Step 1: Impact.** Run impact on `handleStorymaker`, `handleStorymakerStatus` and `handleStorymakerProject`. Report callers and risk, and warn on HIGH or CRITICAL.

- [ ] **Step 2: Implement the routes and shared gates.**

**Create** `internal/server/storymaker_agent_handlers.go`:
```go
package server

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"strings"
	"unicode/utf8"

	"aurago/internal/config"
	"aurago/internal/i18n"
	"aurago/internal/storymaker"
	"aurago/internal/storymaker/render"
)

const (
	storymakerStartTopicRunes       = 2000
	storymakerStartInstructionRunes = 2000
	storymakerStartAudienceRunes    = 120
	storymakerStartTitleRunes       = 120
	storymakerStartDefaultSeconds   = 60
	storymakerStartMinSeconds       = 15
)

// storymakerAgentRefusal is why an agent job was not started: the HTTP status, the stable code and,
// for busy, the job in the way.
type storymakerAgentRefusal struct {
	Status  int
	Code    string
	Message string
	JobID   string
}

func (e *storymakerAgentRefusal) Error() string { return e.Code + ": " + e.Message }

// writeStorymakerAgentStartError answers a refusal, or a project store error.
func writeStorymakerAgentStartError(w http.ResponseWriter, err error) {
	var refusal *storymakerAgentRefusal
	if errors.As(err, &refusal) {
		extra := map[string]any{}
		if refusal.JobID != "" {
			extra["job_id"] = refusal.JobID
		}
		writeStorymakerErrorWith(w, refusal.Status, refusal.Code, refusal.Message, extra)
		return
	}
	mapStorymakerError(w, err, "project_not_found")
}

// storymakerAgentModelTooSmall reports whether the pinned or main model has too little context for a job.
func storymakerAgentModelTooSmall(cfg *config.Config) bool {
	_, blocked := gameMakerModelLimitBlocked(cfg, strings.TrimSpace(cfg.Storymaker.Agent.Provider), strings.TrimSpace(cfg.Storymaker.Agent.Model), false)
	return blocked
}

// storymakerAgentStatus is the agent block of GET /status. The route is the one a job would use.
func (s *Server) storymakerAgentStatus(cfg *config.Config) map[string]any {
	provider := strings.TrimSpace(cfg.Storymaker.Agent.Provider)
	if provider == "" {
		provider = cfg.LLM.Provider
	}
	route, err := s.storymakerAgentResolveRoute(cfg.Storymaker.Agent.Provider, cfg.Storymaker.Agent.Model)
	status := map[string]any{
		"enabled":            cfg.Storymaker.Agent.Enabled,
		"available":          cfg.Storymaker.Agent.Enabled && err == nil && !storymakerAgentModelTooSmall(cfg),
		"provider":           provider,
		"model":              "",
		"compact":            false,
		"text_mode":          false,
		"max_length_seconds": cfg.Storymaker.Agent.MaxLengthSeconds,
		"max_daily_jobs":     cfg.Storymaker.Agent.MaxDailyJobs,
	}
	if err == nil {
		status["model"] = route.cfg.LLM.Model
		status["compact"] = route.compact
		status["text_mode"] = route.text
	}
	return status
}

// storymakerAgentReady refuses when the agent is off, no usable model route exists or the model is too small.
func (s *Server) storymakerAgentReady(acc *storymakerAccess) error {
	if !acc.cfg.Storymaker.Agent.Enabled {
		return &storymakerAgentRefusal{Status: http.StatusForbidden, Code: "agent_disabled", Message: "The Storymaker agent is disabled in the configuration."}
	}
	if _, err := s.storymakerAgentResolveRoute(acc.cfg.Storymaker.Agent.Provider, acc.cfg.Storymaker.Agent.Model); err != nil {
		return &storymakerAgentRefusal{Status: http.StatusConflict, Code: "llm_unavailable", Message: "No usable language model is configured for the Storymaker agent."}
	}
	if storymakerAgentModelTooSmall(acc.cfg) {
		return &storymakerAgentRefusal{Status: http.StatusUnprocessableEntity, Code: "model_too_small", Message: "The selected model has too little context for the Storymaker agent. Choose a larger model."}
	}
	return nil
}

// storymakerAgentReserveDaily counts one job against storymaker.agent.max_daily_jobs. The handler calls it
// before storymakerStartAgentJob. A full day, or a count that cannot be read, is a 429 refusal.
func storymakerAgentReserveDaily(acc *storymakerAccess) error {
	if err := acc.rt.agentUsage.reserve(acc.cfg.Storymaker.Agent.MaxDailyJobs); err != nil {
		return &storymakerAgentRefusal{Status: http.StatusTooManyRequests, Code: storymakerCodeDailyLimit, Message: "The daily limit of Storymaker agent jobs is reached."}
	}
	return nil
}

// storymakerAgentReleaseDaily gives the count back for a job that was not queued.
func storymakerAgentReleaseDaily(acc *storymakerAccess) {
	acc.rt.agentUsage.release(acc.cfg.Storymaker.Agent.MaxDailyJobs)
}

// storymakerProjectActiveJob finds a queued or running job of the project in any queue.
func storymakerProjectActiveJob(rt *storymakerRuntime, projectID string) (render.Job, bool) {
	for _, job := range rt.listJobs(projectID) {
		if !job.Terminal() {
			return job, true
		}
	}
	return render.Job{}, false
}

// storymakerAgentBusy is the refusal for a project that already has a job.
func storymakerAgentBusy(jobID string) *storymakerAgentRefusal {
	return &storymakerAgentRefusal{Status: http.StatusConflict, Code: render.CodeBusy, Message: "This project already has a job. Wait until it has finished.", JobID: jobID}
}

// refuseStorymakerWhileAgentRuns answers 409 agent_job_active while an agent job of the project is
// queued or running, and reports whether it answered.
func refuseStorymakerWhileAgentRuns(w http.ResponseWriter, rt *storymakerRuntime, projectID string) bool {
	for _, job := range rt.agent.List(projectID) {
		if !job.Terminal() {
			writeStorymakerErrorWith(w, http.StatusConflict, "agent_job_active", "The agent is working on this story. Wait until it has finished or cancel it.", map[string]any{"job_id": job.ID})
			return true
		}
	}
	return false
}

// storymakerStartSeconds applies the default and clamps instead of refusing, so a chat call does not fail.
func storymakerStartSeconds(cfg *config.Config, requested int) int {
	if requested <= 0 {
		requested = storymakerStartDefaultSeconds
	}
	limit := cfg.Storymaker.Agent.MaxLengthSeconds
	if renderLimit := cfg.Storymaker.Render.MaxDurationSeconds; renderLimit > 0 && (limit <= 0 || renderLimit < limit) {
		limit = renderLimit
	}
	if requested < storymakerStartMinSeconds {
		requested = storymakerStartMinSeconds
	}
	if limit > 0 && requested > limit {
		requested = limit
	}
	return requested
}

// storymakerStartAgentJob runs the shared gates and queues the job. The project must exist. A refusal
// is a *storymakerAgentRefusal; any other error comes from the project store. It never counts the job
// against the daily limit: the caller reserves that count and releases it when this function refuses.
func (s *Server) storymakerStartAgentJob(ctx context.Context, acc *storymakerAccess, p storymakerAgentPayload) (render.Job, error) {
	if err := s.storymakerAgentReady(acc); err != nil {
		return render.Job{}, err
	}
	if _, err := acc.rt.projects.Get(ctx, p.ProjectID); err != nil {
		return render.Job{}, err
	}
	if active, busy := storymakerProjectActiveJob(acc.rt, p.ProjectID); busy {
		return render.Job{}, storymakerAgentBusy(active.ID)
	}
	if p.Kind == storymaker.AgentKindRevise {
		data, _, err := acc.rt.projects.LoadStory(ctx, p.ProjectID)
		if err != nil {
			return render.Job{}, err
		}
		story, _, err := storymaker.DecodeStory(data, storymakerLimits(acc.cfg))
		if err != nil {
			return render.Job{}, err
		}
		if storymaker.StoryIsBlank(&story) {
			return render.Job{}, &storymakerAgentRefusal{Status: http.StatusUnprocessableEntity, Code: "invalid_story", Message: "This story is empty. Start a new film instead."}
		}
	}
	p.Provider = strings.TrimSpace(acc.cfg.Storymaker.Agent.Provider)
	p.Model = strings.TrimSpace(acc.cfg.Storymaker.Agent.Model)
	p.LengthSeconds = storymakerStartSeconds(acc.cfg, p.LengthSeconds)
	p.Language = strings.TrimSpace(p.Language)
	if p.Language == "" {
		p.Language = i18n.NormalizeLang(acc.cfg.Server.UILanguage)
	}
	job, replayed, err := acc.rt.agent.Enqueue(render.EnqueueRequest{ProjectID: p.ProjectID, Kind: render.KindAgent, Coalesce: true, Payload: p})
	switch {
	case errors.Is(err, render.ErrBusy):
		return render.Job{}, &storymakerAgentRefusal{Status: http.StatusTooManyRequests, Code: render.CodeBusy, Message: "Eight agent jobs are already waiting. Try again when one has finished."}
	case err != nil:
		return render.Job{}, &storymakerAgentRefusal{Status: http.StatusInternalServerError, Code: "job_failed", Message: "The agent job could not be queued."}
	case replayed:
		return render.Job{}, storymakerAgentBusy(job.ID)
	}
	return job, nil
}

// storymakerAgentCreateProject makes the project of a new job, titled after the topic.
func storymakerAgentCreateProject(ctx context.Context, acc *storymakerAccess, topic, aspect string) (string, error) {
	title := strings.Join(strings.Fields(topic), " ")
	if utf8.RuneCountInString(title) > storymakerStartTitleRunes {
		title = strings.TrimSpace(string([]rune(title)[:storymakerStartTitleRunes]))
	}
	project, _, _, err := acc.rt.projects.CreateWithVoice(ctx, title, aspect, 0, nil)
	if err != nil {
		return "", err
	}
	return project.ID, nil
}

// decodeStorymakerAgentBody decodes a small strict JSON body; on a failure it has answered.
func decodeStorymakerAgentBody(w http.ResponseWriter, r *http.Request, v any, shape string) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, storymakerJSONLimit))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeStorymakerError(w, http.StatusBadRequest, "invalid_request", "The request body must be "+shape+".", nil)
		return false
	}
	return true
}

type storymakerAgentStartRequest struct {
	Topic         string            `json:"topic"`
	Audience      string            `json:"audience"`
	LengthSeconds int               `json:"length_seconds"`
	Aspect        string            `json:"aspect"`
	Language      string            `json:"language"`
	Voice         *storymaker.Voice `json:"voice"`
	ProjectID     string            `json:"project_id"`
}

// handleStorymakerAgentJobs answers POST /agent/jobs. Without project_id it creates the project and
// removes it again when the start is refused. With project_id the story must still be blank.
func handleStorymakerAgentJobs(s *Server, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeStorymakerMethodError(w, "POST")
		return
	}
	acc, ok := storymakerAuthorize(s, w, r, desktopScopeWrite, desktopWrite, true)
	if !ok {
		return
	}
	var req storymakerAgentStartRequest
	if !decodeStorymakerAgentBody(w, r, &req, `{"topic": ..., "audience"?: ..., "length_seconds"?: ..., "aspect"?: ..., "language"?: ..., "voice"?: ..., "project_id"?: ...}`) {
		return
	}
	topic := strings.TrimSpace(req.Topic)
	if topic == "" || utf8.RuneCountInString(topic) > storymakerStartTopicRunes {
		writeStorymakerError(w, http.StatusBadRequest, "invalid_topic", "The topic must be 1 to 2000 characters.", nil)
		return
	}
	if err := s.storymakerAgentReady(acc); err != nil {
		writeStorymakerAgentStartError(w, err)
		return
	}
	ctx := r.Context()
	projectID := strings.TrimSpace(req.ProjectID)
	created := false
	if projectID == "" {
		id, err := storymakerAgentCreateProject(ctx, acc, topic, req.Aspect)
		if err != nil {
			mapStorymakerError(w, err, "project_not_found")
			return
		}
		projectID, created = id, true
	} else if !storymakerAgentProjectAcceptsCreate(ctx, w, acc, projectID) {
		return
	}
	audience := strings.TrimSpace(req.Audience)
	if utf8.RuneCountInString(audience) > storymakerStartAudienceRunes {
		audience = strings.TrimSpace(string([]rune(audience)[:storymakerStartAudienceRunes]))
	}
	payload := storymakerAgentPayload{
		Kind: storymaker.AgentKindCreate, ProjectID: projectID, Topic: topic, Audience: audience,
		Language: req.Language, LengthSeconds: req.LengthSeconds, Voice: req.Voice,
	}
	// The handler counts the job before the start and gives the count back when the start refuses.
	if err := storymakerAgentReserveDaily(acc); err != nil {
		if created {
			_ = acc.rt.projects.Delete(ctx, projectID)
		}
		writeStorymakerAgentStartError(w, err)
		return
	}
	job, err := s.storymakerStartAgentJob(ctx, acc, payload)
	if err != nil {
		storymakerAgentReleaseDaily(acc)
		if created {
			_ = acc.rt.projects.Delete(ctx, projectID)
		}
		writeStorymakerAgentStartError(w, err)
		return
	}
	writeStorymakerJSON(w, http.StatusAccepted, map[string]any{"project_id": projectID, "job": job})
}

// storymakerAgentProjectAcceptsCreate checks a named project in the order 404, busy, story_not_empty;
// on a refusal it has answered.
func storymakerAgentProjectAcceptsCreate(ctx context.Context, w http.ResponseWriter, acc *storymakerAccess, projectID string) bool {
	if !storymaker.ValidProjectID(projectID) {
		writeStorymakerError(w, http.StatusNotFound, "project_not_found", "Project not found.", nil)
		return false
	}
	if _, err := acc.rt.projects.Get(ctx, projectID); err != nil {
		mapStorymakerError(w, err, "project_not_found")
		return false
	}
	if active, busy := storymakerProjectActiveJob(acc.rt, projectID); busy {
		writeStorymakerAgentStartError(w, storymakerAgentBusy(active.ID))
		return false
	}
	data, _, err := acc.rt.projects.LoadStory(ctx, projectID)
	if err != nil {
		mapStorymakerError(w, err, "project_not_found")
		return false
	}
	story, _, err := storymaker.DecodeStory(data, storymakerLimits(acc.cfg))
	if err != nil {
		mapStorymakerError(w, err, "project_not_found")
		return false
	}
	if !storymaker.StoryIsBlank(&story) {
		writeStorymakerError(w, http.StatusConflict, "story_not_empty", "This project already has a story. Use revise instead.", nil)
		return false
	}
	return true
}

// handleStorymakerAgentRevise answers POST /projects/{id}/agent/revise.
func handleStorymakerAgentRevise(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeStorymakerMethodError(w, "POST")
		return
	}
	acc, ok := storymakerAuthorize(s, w, r, desktopScopeWrite, desktopWrite, true)
	if !ok {
		return
	}
	var req struct {
		Instruction string `json:"instruction"`
	}
	if !decodeStorymakerAgentBody(w, r, &req, `{"instruction": ...}`) {
		return
	}
	instruction := strings.TrimSpace(req.Instruction)
	if instruction == "" || utf8.RuneCountInString(instruction) > storymakerStartInstructionRunes {
		writeStorymakerError(w, http.StatusBadRequest, "invalid_instruction", "The instruction must be 1 to 2000 characters.", nil)
		return
	}
	if err := storymakerAgentReserveDaily(acc); err != nil {
		writeStorymakerAgentStartError(w, err)
		return
	}
	job, err := s.storymakerStartAgentJob(r.Context(), acc, storymakerAgentPayload{Kind: storymaker.AgentKindRevise, ProjectID: id, Instruction: instruction})
	if err != nil {
		storymakerAgentReleaseDaily(acc)
		writeStorymakerAgentStartError(w, err)
		return
	}
	writeStorymakerJSON(w, http.StatusAccepted, map[string]any{"job": job})
}

// storymakerMakeSilent turns the speech of a story into on-screen text (decision 11). Before it saves, it keeps the
// story as the speech backup, which restore-speech puts back. ifMatch is the etag the caller read. It returns the new
// etag, the story and the number of scenes changed. Nothing is written when no scene has speech, so an older backup
// stays. The make-silent route and chat render with silent both call it.
func storymakerMakeSilent(ctx context.Context, acc *storymakerAccess, projectID, ifMatch string) (string, storymaker.Story, int, error) {
	data, etag, err := acc.rt.projects.LoadStory(ctx, projectID)
	if err != nil {
		return "", storymaker.Story{}, 0, err
	}
	if etag != ifMatch {
		return "", storymaker.Story{}, 0, storymaker.ErrConflict
	}
	story, _, err := storymaker.DecodeStory(data, storymakerLimits(acc.cfg))
	if err != nil {
		return "", storymaker.Story{}, 0, err
	}
	storymaker.ApplyDefaults(&story)
	changed := storymaker.MakeSilent(&story)
	if changed == 0 {
		return etag, story, 0, nil
	}
	canonical, err := storymaker.CanonicalJSON(story)
	if err != nil {
		return "", storymaker.Story{}, 0, err
	}
	if err := acc.rt.projects.SaveSilentBackup(ctx, projectID, data); err != nil {
		return "", storymaker.Story{}, 0, err
	}
	saved, err := acc.rt.projects.SaveStory(ctx, projectID, canonical, ifMatch)
	if err != nil {
		_ = acc.rt.projects.RemoveSilentBackup(ctx, projectID)
		return "", storymaker.Story{}, 0, err
	}
	return saved, story, changed, nil
}

// storymakerStoryFindings validates a story the way PUT story does.
func storymakerStoryFindings(ctx context.Context, acc *storymakerAccess, projectID string, story *storymaker.Story) []storymaker.Issue {
	issues := storymaker.Validate(story, acc.rt.catalog, storymakerLimits(acc.cfg))
	if status, err := acc.rt.projects.CodeSceneStatus(ctx, projectID); err == nil {
		issues = append(issues, storymaker.CodeSceneIssues(story, status)...)
	}
	return issues
}

// writeStorymakerStoryConflict answers a failed save and names the current version on a conflict.
func writeStorymakerStoryConflict(ctx context.Context, w http.ResponseWriter, acc *storymakerAccess, projectID string, err error) {
	if errors.Is(err, storymaker.ErrConflict) {
		if _, current, readErr := acc.rt.projects.LoadStory(ctx, projectID); readErr == nil {
			w.Header().Set("ETag", current)
		}
	}
	mapStorymakerError(w, err, "project_not_found")
}

// handleStorymakerMakeSilent answers POST /projects/{id}/make-silent. A story with nothing to change
// keeps its older backup, so a second press cannot overwrite the speech with a silent copy.
func handleStorymakerMakeSilent(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeStorymakerMethodError(w, "POST")
		return
	}
	acc, ok := storymakerAuthorize(s, w, r, desktopScopeWrite, desktopWrite, true)
	if !ok {
		return
	}
	match, ok := requireStorymakerMatch(w, r)
	if !ok {
		return
	}
	if refuseStorymakerWhileAgentRuns(w, acc.rt, id) {
		return
	}
	ctx := r.Context()
	etag, story, changed, err := storymakerMakeSilent(ctx, acc, id, match)
	if err != nil {
		writeStorymakerStoryConflict(ctx, w, acc, id, err)
		return
	}
	w.Header().Set("ETag", etag)
	writeStorymakerJSON(w, http.StatusOK, map[string]any{
		"etag": etag, "issues": issuesOrEmpty(storymakerStoryFindings(ctx, acc, id, &story)), "story": story, "changed": changed,
	})
}

// handleStorymakerRestoreSpeech answers POST /projects/{id}/restore-speech: it puts the backup of
// make-silent back and deletes it.
func handleStorymakerRestoreSpeech(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeStorymakerMethodError(w, "POST")
		return
	}
	acc, ok := storymakerAuthorize(s, w, r, desktopScopeWrite, desktopWrite, true)
	if !ok {
		return
	}
	match, ok := requireStorymakerMatch(w, r)
	if !ok {
		return
	}
	if refuseStorymakerWhileAgentRuns(w, acc.rt, id) {
		return
	}
	ctx := r.Context()
	if _, err := acc.rt.projects.Get(ctx, id); err != nil {
		mapStorymakerError(w, err, "project_not_found")
		return
	}
	backup, err := acc.rt.projects.LoadSilentBackup(ctx, id)
	if errors.Is(err, fs.ErrNotExist) {
		writeStorymakerError(w, http.StatusNotFound, "no_silent_backup", "There is no copy with speech to restore.", nil)
		return
	}
	if err != nil {
		mapStorymakerError(w, err, "project_not_found")
		return
	}
	story, _, err := storymaker.DecodeStory(backup, storymakerLimits(acc.cfg))
	if err != nil {
		mapStorymakerError(w, err, "project_not_found")
		return
	}
	etag, err := acc.rt.projects.SaveStory(ctx, id, backup, match)
	if err != nil {
		writeStorymakerStoryConflict(ctx, w, acc, id, err)
		return
	}
	_ = acc.rt.projects.RemoveSilentBackup(ctx, id)
	storymaker.ApplyDefaults(&story)
	w.Header().Set("ETag", etag)
	writeStorymakerJSON(w, http.StatusOK, map[string]any{
		"etag": etag, "issues": issuesOrEmpty(storymakerStoryFindings(ctx, acc, id, &story)), "story": story,
	})
}
```

- [ ] **Step 3: Wire the routes, the status block and `silent_backup`.**

**Edit** `internal/server/storymaker_handlers.go`
Old:
```go
			case len(parts) == 3 && parts[2] == "narrate":
```
New:
```go
			case len(parts) == 3 && parts[2] == "make-silent":
				handleStorymakerMakeSilent(s, w, r, id)
			case len(parts) == 3 && parts[2] == "restore-speech":
				handleStorymakerRestoreSpeech(s, w, r, id)
			case len(parts) == 4 && parts[2] == "agent" && parts[3] == "revise":
				handleStorymakerAgentRevise(s, w, r, id)
			case len(parts) == 3 && parts[2] == "narrate":
```

**Edit** `internal/server/storymaker_handlers.go`
Old:
```go
		case len(parts) == 2 && parts[0] == "jobs":
```
New:
```go
		case len(parts) == 2 && parts[0] == "agent" && parts[1] == "jobs":
			handleStorymakerAgentJobs(s, w, r)
		case len(parts) == 2 && parts[0] == "jobs":
```

**Edit** `internal/server/storymaker_handlers.go`
Old:
```go
	status["catalog_version"] = rt.catalog.Version()
```
New:
```go
	status["catalog_version"] = rt.catalog.Version()
	status["agent"] = s.storymakerAgentStatus(cfg)
```

**Edit** `internal/server/storymaker_handlers.go`
Old:
```go
		body := map[string]any{"project": project, "etag": etag, "code_scenes": status}
```
New:
```go
		body := map[string]any{"project": project, "etag": etag, "code_scenes": status}
		if has, err := acc.rt.projects.HasSilentBackup(ctx, id); err == nil && has {
			body["silent_backup"] = true
		}
```

- [ ] **Step 4: Test.** The fixture sets the agent queue's run function through `storymakerAgentRunOverride`, so no model is called. D2 and D7 reuse this fixture.

**Create** `internal/server/storymaker_agent_handlers_test.go`:
```go
package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/storymaker"
	"aurago/internal/storymaker/render"

	"github.com/sashabaranov/go-openai"
)

// storymakerAgentRouteFixture makes a server whose agent jobs run the given function. Render and audio
// jobs block until they are cancelled.
func storymakerAgentRouteFixture(t *testing.T, run render.RunFunc) (*Server, *storymakerRuntime, string) {
	t.Helper()
	s, _, writeToken := storymakerAPIFixture(t)
	s.Cfg.Storymaker.Agent.Enabled = true
	s.Cfg.Server.UILanguage = "en"
	s.Cfg.LLM.Model = "test-model"
	s.LLMClient = llm.WrapOpenAIClient(openai.NewClientWithConfig(openai.DefaultConfig("test-only")))
	s.storymakerRunOverride = storymakerBlockingRun
	s.storymakerAudioRunOverride = storymakerBlockingRun
	s.storymakerAgentRunOverride = run
	rt, err := s.storymakerRuntime()
	if err != nil {
		t.Fatal(err)
	}
	return s, rt, writeToken
}

func TestStorymakerAgentRoutesRefuseDisabledReadOnlyAndBusy(t *testing.T) {
	s, rt, token := storymakerAgentRouteFixture(t, storymakerBlockingRun)
	start := func(body string) *httptest.ResponseRecorder {
		return storymakerRequest(s, token, http.MethodPost, "/agent/jobs", strings.NewReader(body), nil)
	}
	want := func(name string, w *httptest.ResponseRecorder, status int, code string) {
		t.Helper()
		if w.Code != status || !strings.Contains(w.Body.String(), code) {
			t.Fatalf("%s = %d %s, want %d %s", name, w.Code, w.Body.String(), status, code)
		}
	}
	withConfig := func(change func(*config.Config), check func()) {
		t.Helper()
		before := *s.ConfigSnapshot()
		next := before
		change(&next)
		s.replaceConfigSnapshot(&next)
		check()
		s.replaceConfigSnapshot(&before)
	}

	want("blank topic", start(`{"topic":"   "}`), http.StatusBadRequest, "invalid_topic")
	withConfig(func(c *config.Config) { c.VirtualDesktop.ReadOnly = true }, func() {
		want("read-only", start(`{"topic":"Backups"}`), http.StatusForbidden, "desktop_readonly")
	})
	withConfig(func(c *config.Config) { c.Storymaker.Enabled = false }, func() {
		want("storymaker off", start(`{"topic":"Backups"}`), http.StatusForbidden, "storymaker_disabled")
	})
	withConfig(func(c *config.Config) { c.Storymaker.Agent.Enabled = false }, func() {
		want("agent off", start(`{"topic":"Backups"}`), http.StatusForbidden, "agent_disabled")
	})

	w := start(`{"topic":"Was ist eine Firewall?"}`)
	if w.Code != http.StatusAccepted {
		t.Fatalf("start = %d %s", w.Code, w.Body.String())
	}
	var started struct {
		ProjectID string     `json:"project_id"`
		Job       render.Job `json:"job"`
	}
	decodeBody(t, w, &started)
	if !storymaker.ValidProjectID(started.ProjectID) || started.Job.Kind != render.KindAgent {
		t.Fatalf("start answer = %+v", started)
	}

	// A second job of the same project is refused and names the job in the way.
	var busy struct {
		Code  string `json:"code"`
		JobID string `json:"job_id"`
	}
	w = start(`{"topic":"Noch einmal","project_id":"` + started.ProjectID + `"}`)
	want("second start", w, http.StatusConflict, render.CodeBusy)
	decodeBody(t, w, &busy)
	if busy.JobID != started.Job.ID {
		t.Fatalf("busy job_id = %q, want %q", busy.JobID, started.Job.ID)
	}

	// A narrate job of another project keeps the agent out of that project.
	other := createStorymakerProject(t, s, token)
	enqueueStorymakerJob(t, rt.narrate, other.Project.ID, render.KindNarrate)
	w = storymakerRequest(s, token, http.MethodPost, "/projects/"+other.Project.ID+"/agent/revise", strings.NewReader(`{"instruction":"Kürzer"}`), nil)
	want("revise during narration", w, http.StatusConflict, render.CodeBusy)

	var status struct {
		Agent struct {
			Enabled   bool   `json:"enabled"`
			Available bool   `json:"available"`
			Model     string `json:"model"`
		} `json:"agent"`
	}
	decodeBody(t, storymakerRequest(s, token, http.MethodGet, "/status", nil, nil), &status)
	if !status.Agent.Enabled || !status.Agent.Available || status.Agent.Model != "test-model" {
		t.Fatalf("status agent = %+v", status.Agent)
	}
}
```

- [ ] **Step 5: Run.** Run this command.
`GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./internal/server -run '^TestStorymakerAgentRoutesRefuseDisabledReadOnlyAndBusy$' -count=1`
Expected: `ok  	aurago/internal/server`.

- [ ] **Step 6: Commit.** Stage `internal/server/storymaker_agent_handlers.go`, `internal/server/storymaker_handlers.go` and `internal/server/storymaker_agent_handlers_test.go`. Message: `feat(storymaker): add agent start, revise, make-silent and restore-speech routes`

### Task D2: Guards during a job

**Batch:** D  **Depends on:** D1
**Files:**
Modify `internal/server/storymaker_handlers.go`
Modify `internal/server/storymaker_audio_handlers.go`
Test `internal/server/storymaker_agent_guards_test.go`

- [ ] **Step 1: Impact.** Run impact on `handleStorymakerStory`, `handleStorymakerCode`, `handleStorymakerRender` and `handleStorymakerNarrate`. Expect HIGH or CRITICAL on `handleStorymakerStory`, and warn.

- [ ] **Step 2: Implement.** The agent's own writes and its narration step do not pass through these routes, so the guards do not block the job itself.

**Edit** `internal/server/storymaker_handlers.go`
Old:
```go
	body, err := io.ReadAll(io.LimitReader(r.Body, int64(lim.MaxStoryBytes)+1))
```
New:
```go
	if refuseStorymakerWhileAgentRuns(w, acc.rt, id) {
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, int64(lim.MaxStoryBytes)+1))
```

**Edit** `internal/server/storymaker_handlers.go`
Old:
```go
		ifMatch := ""
```
New:
```go
		if refuseStorymakerWhileAgentRuns(w, acc.rt, id) {
			return
		}
		ifMatch := ""
```

**Edit** `internal/server/storymaker_handlers.go`
Old:
```go
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
```
New:
```go
	if refuseStorymakerWhileAgentRuns(w, acc.rt, id) {
		return
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
```

**Edit** `internal/server/storymaker_audio_handlers.go`
Old:
```go
	story, answer, ok := s.storymakerStoryAudio(w, r, acc, id)
```
New:
```go
	if refuseStorymakerWhileAgentRuns(w, acc.rt, id) {
		return
	}
	story, answer, ok := s.storymakerStoryAudio(w, r, acc, id)
```

- [ ] **Step 3: Test.**

**Create** `internal/server/storymaker_agent_guards_test.go`:
```go
package server

import (
	"net/http"
	"strings"
	"testing"

	"aurago/internal/storymaker"
	"aurago/internal/storymaker/render"
)

func TestStoryWritesRefusedWhileAgentJobActive(t *testing.T) {
	s, rt, token := storymakerAgentRouteFixture(t, storymakerBlockingRun)
	p := createStorymakerProject(t, s, token)
	job, _, err := rt.agent.Enqueue(render.EnqueueRequest{ProjectID: p.Project.ID, Kind: render.KindAgent})
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := storymaker.CanonicalJSON(p.Story)
	if err != nil {
		t.Fatal(err)
	}
	base := "/projects/" + p.Project.ID
	headers := map[string]string{"If-Match": p.ETag}
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodPut, base + "/story", string(canonical)},
		{http.MethodPut, base + "/scenes/s1/code", "export default {};"},
		{http.MethodPost, base + "/narrate", ""},
		{http.MethodPost, base + "/render", "{}"},
		{http.MethodPost, base + "/make-silent", ""},
		{http.MethodPost, base + "/restore-speech", ""},
	} {
		w := storymakerRequest(s, token, tc.method, tc.path, strings.NewReader(tc.body), headers)
		if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "agent_job_active") {
			t.Errorf("%s %s = %d %s", tc.method, tc.path, w.Code, w.Body.String())
		}
	}

	// The guard ends with the job.
	if _, found := rt.agent.Cancel(job.ID); !found {
		t.Fatal("the job is gone")
	}
	waitStorymakerManagerJob(t, rt.agent, job.ID)
	w := storymakerRequest(s, token, http.MethodPut, base+"/story", strings.NewReader(string(canonical)), headers)
	if w.Code != http.StatusOK {
		t.Fatalf("save after the job = %d %s", w.Code, w.Body.String())
	}
}
```

- [ ] **Step 4: Run.** Run this command.
`GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./internal/server -run '^TestStoryWritesRefusedWhileAgentJobActive$' -count=1`
Expected: `ok  	aurago/internal/server`.

- [ ] **Step 5: Commit.** Stage `internal/server/storymaker_handlers.go`, `internal/server/storymaker_audio_handlers.go` and `internal/server/storymaker_agent_guards_test.go`. Message: `feat(storymaker): refuse story, code, narrate and render writes during an agent job`

### Task D3: Project launch

**Batch:** D  **Depends on:** none
**Files:**
Modify `ui/js/desktop/core/sdk-events-bootstrap.js`
Modify `ui/js/desktop/core/window-shell-runtime.js`
Modify `ui/js/desktop/apps/storymaker.js`
Modify `ui/js/desktop/bundles/main.bundle.js`
Test `ui/desktop_storymaker_registration_test.go`

- [ ] **Step 1: Impact.** Run impact on `handleDesktopEvent` (the function that holds the `open_app` branch; if the graph names it differently, use the name it reports), `openApp`, `createInstance` and `start`. Report callers and risk.

- [ ] **Step 2: Implement.** These JS files are CRLF in this checkout, so every Old below is one line. `* text=auto` normalises the line endings on commit.

**Edit** `ui/js/desktop/core/sdk-events-bootstrap.js`
Old:
```js
            if (event.payload.app_id === 'ecalc') {
```
New:
```js
            if (event.payload.app_id === 'storymaker' && typeof event.payload.project_id === 'string' && /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/.test(event.payload.project_id)) launch.projectId = event.payload.project_id;
            if (event.payload.app_id === 'ecalc') {
```

**Edit** `ui/js/desktop/core/window-shell-runtime.js`
Old:
```js
            if (appId === 'retro-games' && context && context.game && window.RetroGamesApp && typeof window.RetroGamesApp.open === 'function') window.RetroGamesApp.open(existing.id, context);
```
New:
```js
            if (appId === 'retro-games' && context && context.game && window.RetroGamesApp && typeof window.RetroGamesApp.open === 'function') window.RetroGamesApp.open(existing.id, context);
            if (appId === 'storymaker' && context && typeof context.projectId === 'string' && context.projectId) window.StorymakerApp?.open?.(existing.id, context.projectId);
```

**Edit** `ui/js/desktop/apps/storymaker.js`
Old:
```js
// event stream and the window disposal. Loaded last; publishes window.StorymakerApp = {render, dispose}. Classic script.
```
New:
```js
// event stream and the window disposal. Loaded last; publishes window.StorymakerApp = {render, dispose, open}. Classic script.
```

**Edit** `ui/js/desktop/apps/storymaker.js`
Old:
```js
            view: 'loading', parts: {}, root: null, content: null, liveTimer: 0
```
New:
```js
            view: 'loading', parts: {}, root: null, content: null, liveTimer: 0, launchProjectId: typeof c.projectId === 'string' ? c.projectId : ''
```

**Edit** `ui/js/desktop/apps/storymaker.js`
Old:
```js
            inst.bus.emit('status');
```
New:
```js
            inst.bus.emit('status');
            if ((await openLaunchProject(inst)) || inst.disposed) return;
```

**Edit** `ui/js/desktop/apps/storymaker.js`
Old:
```js
    async function showProjects(inst) {
```
New:
```js
    // A launch that names a project opens it directly. A project that is gone falls back to the list.
    async function openLaunchProject(inst) {
        const id = inst.launchProjectId;
        inst.launchProjectId = '';
        return id ? openProject(inst, id) : false;
    }

    async function showProjects(inst) {
```

**Edit** `ui/js/desktop/apps/storymaker.js`
Old:
```js
    window.StorymakerApp = { render, dispose };
```
New:
```js
    // A running window opens the project a launch names. Typed edits are saved first; if that fails the
    // user decides, and a refusal keeps the current project open.
    async function open(windowId, projectId) {
        const inst = instances.get(windowId);
        if (!inst || typeof projectId !== 'string' || !projectId) return false;
        if (inst.view === 'loading') { inst.launchProjectId = projectId; return true; }
        if (inst.view === 'editor' && inst.projectId === projectId) return true;
        if (!(await beforeClose(inst)) || inst.disposed) return false;
        return openProject(inst, projectId);
    }

    window.StorymakerApp = { render, dispose, open };
```

- [ ] **Step 3: Test markers.**

**Edit** `ui/desktop_storymaker_registration_test.go`
Old:
```go
	shell := rawDesktopAssetText(t, "js/desktop/core/window-shell-runtime.js")
```
New:
```go
	shell := rawDesktopAssetText(t, "js/desktop/core/window-shell-runtime.js")
	bootstrap := rawDesktopAssetText(t, "js/desktop/core/sdk-events-bootstrap.js")
```

**Edit** `ui/desktop_storymaker_registration_test.go`
Old:
```go
		{"min size", shell, "if (appId === 'storymaker') return { width: 640, height: 480 };"},
```
New:
```go
		{"min size", shell, "if (appId === 'storymaker') return { width: 640, height: 480 };"},
		{"running window launch", shell, "if (appId === 'storymaker' && context && typeof context.projectId === 'string' && context.projectId) window.StorymakerApp?.open?.(existing.id, context.projectId);"},
		{"launch event", bootstrap, "launch.projectId = event.payload.project_id;"},
```

**Edit** `ui/desktop_storymaker_registration_test.go`
Old:
```go
	// The window hands the app what it needs: translation, dialogs, read-only state, the way to open Files.
```
New:
```go
	// A launch names the project to open, for a new window and for a running one.
	root := rawDesktopAssetText(t, "js/desktop/apps/storymaker.js")
	for _, marker := range []string{"launchProjectId: typeof c.projectId === 'string' ? c.projectId : ''", "window.StorymakerApp = { render, dispose, open };"} {
		if !strings.Contains(root, marker) {
			t.Errorf("storymaker.js lacks %q", marker)
		}
	}
	// The window hands the app what it needs: translation, dialogs, read-only state, the way to open Files.
```

- [ ] **Step 4: Run.** Run these commands in order.
`npm run build:ui`
Expected: exit code 0, and `ui/js/desktop/bundles/main.bundle.js` is rewritten.
`npm run check:ui`
Expected: exit code 0.
`GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./ui -run '^(TestDesktopStorymakerRegistration|TestStorymakerSourcesAreSafeAndSmall)$' -count=1`
Expected: `ok  	aurago/ui`.

- [ ] **Step 5: Commit.** Stage `ui/js/desktop/core/sdk-events-bootstrap.js`, `ui/js/desktop/core/window-shell-runtime.js`, `ui/js/desktop/apps/storymaker.js`, `ui/js/desktop/bundles/main.bundle.js` and `ui/desktop_storymaker_registration_test.go`. Message: `feat(storymaker): open a project from a desktop launch`

<!-- group D2 -->

### Task D4: Panel, API, side tab, lock, reload, conflict
**Batch:** D  **Depends on:** D1, D3
**Files:**
Create `ui/js/desktop/apps/storymaker-agent.js`
Modify `ui/js/desktop/apps/storymaker.js`
Modify `ui/js/desktop/apps/storymaker-api.js`
Modify `ui/css/desktop-app-storymaker.css`
Modify `ui/js/desktop/core/module-loader.js`
Modify `ui/desktop_storymaker_registration_test.go`
Test `internal/server/storymaker_app_browser_test.go`

- [ ] **Step 1: Impact.** Run impact for `createInstance`, `closeEditor`, `openProject`, `buildEditor`, `paintBanners`, `handleSaveError`, `reloadProject`, `createApi` and `TestStorymakerAppRealServerBrowser`. Report callers and risk. Warn on HIGH or CRITICAL.

- [ ] **Step 2: Implement the panel.** Keep it free of template strings and HTML strings, because the source guard greps for them.

**Create** `ui/js/desktop/apps/storymaker-agent.js`:
```js
// Storymaker AI author panel: asks for a topic or a change, starts the agent job, follows it on the project's event stream and shows
// the outcome. While a job of this project is active the editor is locked (inst.agentLocked). The panel reloads the story when the
// job reports a new story_etag, at most once per 2 s and once more at the end; edits that were not saved stay as a conflict.
// Everything from the server reaches the page as text only. Classic script, publishes S.createAgentPanel.
(function () {
    'use strict';

    const S = window.StorymakerParts = window.StorymakerParts || {};
    const ACTIVE = ['queued', 'running'];
    const LANGUAGES = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];
    const MIN_SECONDS = 15;
    const DEFAULT_SECONDS = 60;
    const FALLBACK_MAX_SECONDS = 180;
    const MAX_TEXT = '2000';
    const RELOAD_EVERY_MS = 2000;
    const PHASE_ORDER = {
        create: ['outline', 'cast', 'fill', 'check', 'narrating'],
        revise: ['revise', 'check', 'narrating']
    };
    const PHASE_KEYS = {
        outline: 'storymaker.agent.phase_outline',
        cast: 'storymaker.agent.phase_cast',
        fill: 'storymaker.agent.phase_fill',
        check: 'storymaker.agent.phase_check',
        revise: 'storymaker.agent.phase_revise',
        narrating: 'storymaker.agent.phase_narrating'
    };
    const OUTCOME_KEYS = {
        ready_unverified: { title: 'storymaker.agent.outcome_ready_title', text: 'storymaker.agent.outcome_ready_text' },
        incomplete: { title: 'storymaker.agent.outcome_incomplete_title', text: 'storymaker.agent.outcome_incomplete_text' },
        failed: { title: 'storymaker.agent.outcome_failed_title', text: 'storymaker.agent.outcome_failed_text' }
    };
    const CHECK_KEYS = {
        layout_unverified: 'storymaker.agent.check_layout_unverified',
        compile_unverified: 'storymaker.agent.check_compile_unverified',
        narration_unavailable: 'storymaker.agent.check_narration_unavailable',
        narration_failed: 'storymaker.agent.check_narration_failed',
        narration_budget: 'storymaker.agent.check_narration_budget',
        narration_pending: 'storymaker.agent.check_narration_pending',
        corrections_exhausted: 'storymaker.agent.check_corrections_exhausted',
        model_error: 'storymaker.agent.check_model_error',
        scene_unfilled: 'storymaker.agent.check_scene_unfilled',
        compile_failed: 'storymaker.agent.check_compile_failed',
        over_target: 'storymaker.agent.check_over_target',
        over_budget: 'storymaker.agent.check_over_budget'
    };

    function isActive(job) {
        return !!job && ACTIVE.indexOf(job.status) >= 0;
    }

    function agentBlock(inst) {
        return inst.status && inst.status.agent ? inst.status.agent : null;
    }

    function uiLanguage() {
        const tag = String(window.SYSTEM_LANG || document.documentElement.lang || 'en').toLowerCase().split(/[-_]/)[0];
        return LANGUAGES.indexOf(tag) >= 0 ? tag : 'en';
    }

    function languageName(code) {
        try {
            return new Intl.DisplayNames([uiLanguage()], { type: 'language' }).of(code) || code;
        } catch (_) {
            return code;
        }
    }

    // A story the agent may still write from scratch: one scene with nothing in it. The server has the final word (story_not_empty).
    function storyIsBlank(story) {
        const scenes = S.model.scenesOf(story);
        if (scenes.length !== 1) return false;
        const scene = scenes[0] || {};
        const empty = list => !Array.isArray(list) || !list.length;
        return scene.kind !== 'code' && !String(scene.narration || '').trim() && empty(scene.actors) && empty(scene.props) && empty(scene.beats);
    }

    // The length and language controls of a new film. prefix makes the ids and the data-stm names.
    function createFields(inst, prefix, info) {
        const { el } = S;
        const t = inst.t;
        const max = info && Number(info.max_length_seconds) >= MIN_SECONDS ? Number(info.max_length_seconds) : FALLBACK_MAX_SECONDS;
        const lengthId = inst.domId(prefix + '-length');
        const languageId = inst.domId(prefix + '-language');
        const length = el('input', { class: 'stm-input', type: 'number', id: lengthId, min: String(MIN_SECONDS), max: String(max), step: '5', value: String(Math.min(DEFAULT_SECONDS, max)), dataset: { stm: prefix + '-length' } });
        const language = el('select', { class: 'stm-select', id: languageId, dataset: { stm: prefix + '-language' } }, LANGUAGES.map(code => el('option', { value: code, text: languageName(code) })));
        language.value = uiLanguage();
        const lengthField = el('div', { class: 'stm-field' }, el('label', { class: 'stm-label', for: lengthId, text: t('storymaker.agent.length') }), length);
        const languageField = el('div', { class: 'stm-field' }, el('label', { class: 'stm-label', for: languageId, text: t('storymaker.agent.language') }), language);
        return {
            lengthField,
            languageField,
            seconds: () => Math.round(S.clamp(length.value, MIN_SECONDS, max)),
            language: () => language.value,
            setDisabled(off) { length.disabled = off; language.disabled = off; }
        };
    }

    // inst: {t, bus, api, story, status, etag, dirty, conflict, projectId, readonly, agentLocked, silentBackup, domId(name), notify(text, kind), flushEdits(), reloadProject()}
    function createAgentPanel(inst) {
        const { el, icon } = S;
        const t = inst.t;
        let job = null;
        let starting = false;
        let cancelling = false;
        let working = false;       // a make-silent or restore-speech call is under way
        let disposed = false;
        let errorText = '';
        let reloadTimer = 0;
        let lastReload = 0;
        let prefilledFor = '';
        let eventsJob = '';
        let lastSeq = -1;
        let retrying = false;

        const fields = createFields(inst, 'agent', agentBlock(inst));
        const offLine = el('p', { class: 'stm-hint', dataset: { stm: 'agent-off' }, text: t('storymaker.agent.off') });
        const unavailableLine = el('p', { class: 'stm-hint', hidden: true, dataset: { stm: 'agent-unavailable' }, text: t('storymaker.agent.unavailable') });
        const promptId = inst.domId('agent-prompt');
        const promptLabel = el('label', { class: 'stm-label', for: promptId });
        const promptBox = el('textarea', { class: 'stm-textarea', id: promptId, rows: '4', maxlength: MAX_TEXT, dataset: { stm: 'agent-prompt' } });
        const startLabel = el('span');
        const startButton = el('button', { type: 'submit', class: 'stm-btn stm-btn-primary', dataset: { stm: 'agent-start', mode: 'start' } }, icon('sparkle'), startLabel);
        const row = el('div', { class: 'stm-agent-row' }, fields.lengthField, fields.languageField);
        const form = el('form', { class: 'stm-agent-form', onSubmit: event => { event.preventDefault(); start(); } },
            el('div', { class: 'stm-field' }, promptLabel, promptBox), row, el('div', { class: 'stm-actions' }, startButton));
        const errorLine = el('p', { class: 'stm-form-error', role: 'alert', hidden: true, dataset: { stm: 'agent-error' } });
        const routeLine = el('p', { class: 'stm-hint', hidden: true, dataset: { stm: 'agent-route' } });
        const stateLine = el('p', { class: 'stm-hint', role: 'status', hidden: true, dataset: { stm: 'agent-state', state: 'idle' } });
        const steps = el('ol', { class: 'stm-agent-steps', hidden: true, dataset: { stm: 'agent-progress' } });
        const scenesLine = el('p', { class: 'stm-hint', hidden: true, dataset: { stm: 'agent-scenes' } });
        const retryLine = el('p', { class: 'stm-hint', hidden: true, dataset: { stm: 'agent-retry' }, text: t('storymaker.agent.model_retry') });
        const cancelButton = el('button', { type: 'button', class: 'stm-btn', hidden: true, dataset: { stm: 'agent-cancel' }, onClick: () => cancel() }, icon('stop'), el('span', { text: t('storymaker.agent.cancel') }));
        const outcomeCard = el('div', { class: 'stm-agent-outcome', hidden: true, dataset: { stm: 'agent-outcome' } });
        const silentButton = el('button', { type: 'button', class: 'stm-btn', hidden: true, dataset: { stm: 'agent-make-silent' }, onClick: () => makeSilent() }, el('span', { text: t('storymaker.agent.make_silent') }));
        const restoreButton = el('button', { type: 'button', class: 'stm-btn', hidden: true, dataset: { stm: 'agent-restore-speech' }, onClick: () => restoreSpeech() }, el('span', { text: t('storymaker.agent.restore_speech') }));
        const root = el('section', { class: 'stm-agent', ariaLabel: t('storymaker.side_agent'), dataset: { stm: 'agent' } },
            offLine, unavailableLine, form, errorLine, routeLine, stateLine, steps, scenesLine, retryLine, cancelButton, outcomeCard,
            el('div', { class: 'stm-actions' }, silentButton, restoreButton));

        function setError(text) {
            errorText = text || '';
        }

        function refusalText(err) {
            const e = S.errors.normalize(err);
            if (e.code === 'invalid_story') return t('storymaker.agent.refused_invalid_story');
            return S.errors.describe(t, e);
        }

        async function start() {
            if (starting || inst.readonly || isActive(job) || disposed) return;
            const text = promptBox.value.trim();
            if (!text) { setError(t('storymaker.agent.prompt_required')); paint(); promptBox.focus(); return; }
            starting = true;
            setError('');
            paint();
            try {
                if (!(await inst.flushEdits())) { setError(t('storymaker.agent.save_first')); return; }
                if (disposed) return;
                const answer = storyIsBlank(inst.story)
                    ? await inst.api.agentStart({ topic: text, length_seconds: fields.seconds(), language: fields.language(), project_id: inst.projectId })
                    : await inst.api.agentRevise(inst.projectId, text);
                if (disposed) return;
                promptBox.value = '';
                applyJob(answer.job, true);
            } catch (err) {
                if (!(err && err.aborted)) setError(refusalText(err));
            } finally {
                starting = false;
                paint();
            }
        }

        // Cancel stays open while the lock makes the editor read-only: it only stops the job.
        async function cancel() {
            if (!isActive(job) || cancelling) return;
            cancelling = true;
            paint();
            try {
                applyJob(await inst.api.cancelJob(job.id), true);
            } catch (err) {
                if (!(err && err.aborted)) inst.notify(S.errors.describe(t, err), 'error');
            } finally {
                cancelling = false;
                paint();
            }
        }

        // make-silent and restore-speech change the story on the server, so the answer is read back through the normal reload.
        async function storyAction(call, doneKey) {
            if (working || inst.readonly || disposed) return;
            working = true;
            setError('');
            paint();
            try {
                if (!(await inst.flushEdits())) { setError(t('storymaker.agent.save_first')); return; }
                await call(inst.projectId, inst.etag);
                if (disposed) return;
                await inst.reloadProject();
                if (doneKey) inst.notify(t(doneKey));
            } catch (err) {
                if (!(err && err.aborted)) setError(refusalText(err));
            } finally {
                working = false;
                paint();
            }
        }

        function makeSilent() {
            return storyAction((id, etag) => inst.api.makeSilent(id, etag), 'storymaker.agent.make_silent_done');
        }

        async function restoreSpeech() {
            if (working || inst.readonly || disposed) return;
            const ok = await inst.confirm(t('storymaker.agent.restore_speech'), t('storymaker.agent.restore_confirm'));
            if (!ok) return;
            return storyAction((id, etag) => inst.api.restoreSpeech(id, etag), '');
        }

        function reloadNow() {
            clearTimeout(reloadTimer);
            reloadTimer = 0;
            if (disposed || inst.dirty || inst.conflict) return;
            lastReload = Date.now();
            inst.reloadProject();
        }

        // A new story_etag while the job runs: reload at most once per RELOAD_EVERY_MS. Unsaved local text is never replaced.
        function maybeReload() {
            const etag = job.agent && job.agent.story_etag;
            if (!etag || etag === inst.etag || inst.dirty || inst.conflict) return;
            const wait = lastReload + RELOAD_EVERY_MS - Date.now();
            if (wait <= 0) { reloadNow(); return; }
            if (!reloadTimer) reloadTimer = setTimeout(reloadNow, wait);
        }

        // The job ended. Without local edits the story is read once more; with them the conflict banner offers reload or overwrite.
        function finishReload() {
            clearTimeout(reloadTimer);
            reloadTimer = 0;
            if (inst.conflict) return;
            if (inst.dirty) { inst.bus.emit('conflict'); return; }
            lastReload = Date.now();
            inst.reloadProject();
        }

        function syncLock() {
            const locked = isActive(job);
            if (locked === !!inst.agentLocked) return;
            inst.agentLocked = locked;
            // runSave stops at a read-only editor, so edits that are still unsaved when the lock arrives become a conflict here.
            if (locked && inst.dirty) inst.bus.emit('conflict');
            inst.bus.emit('readonly');
        }

        // The stream replays jobs newest first on every connection, so an older job never replaces a newer one, and a stale answer
        // (a POST that returns after the stream has already moved on) never moves a job backwards.
        function applyJob(next, mine) {
            if (disposed || !next || typeof next !== 'object' || next.kind !== 'agent') return;
            if (next.project_id && inst.projectId && next.project_id !== inst.projectId) return;
            const same = !!job && job.id === next.id;
            if (same && Date.parse(next.updated_at) < Date.parse(job.updated_at)) return;
            if (!same && job && !mine && (isActive(job) || Date.parse(next.created_at) < Date.parse(job.created_at))) return;
            const was = same ? job.status : '';
            job = next;
            syncLock();
            if (isActive(job)) maybeReload();
            else if (ACTIVE.indexOf(was) >= 0) finishReload();
            prefillAfterIncomplete();
            paint();
        }

        function prefillAfterIncomplete() {
            const outcome = job.agent ? job.agent.outcome : '';
            if (outcome !== 'incomplete' || prefilledFor === job.id) return;
            prefilledFor = job.id;
            if (!promptBox.value.trim()) promptBox.value = t('storymaker.agent.finish_instruction');
        }

        function takeEvent(event) {
            if (disposed || !event) return;
            if (event.job_id !== eventsJob) {
                eventsJob = event.job_id;
                lastSeq = -1;
                retrying = false;
            }
            const seq = Number(event.seq);
            if (seq <= lastSeq) return;
            lastSeq = seq;
            if (event.type !== 'model') return;
            retrying = event.status === 'retrying' || event.status === 'recovering';
            paint();
        }

        function checksOf() {
            return job && job.agent && Array.isArray(job.agent.open_checks) ? job.agent.open_checks : [];
        }

        function paintRoute() {
            const info = isActive(job) && job.agent ? job.agent : (agentBlock(inst) || {});
            const parts = [];
            if (info.model) parts.push(info.provider ? info.provider + ' / ' + info.model : info.model);
            if (info.compact) parts.push(t('storymaker.agent.mode_compact'));
            if (info.text_mode) parts.push(t('storymaker.agent.mode_text'));
            routeLine.textContent = parts.join(' · ');
            routeLine.hidden = !parts.length;
        }

        function paintState() {
            const status = job ? job.status : 'idle';
            stateLine.dataset.state = status;
            let text = '';
            if (status === 'queued') text = t('storymaker.export.queued');
            else if (status === 'cancelled') text = t('storymaker.agent.state_cancelled');
            else if (status === 'interrupted') text = t('storymaker.agent.state_interrupted');
            stateLine.textContent = text;
            stateLine.hidden = !text;
        }

        function paintProgress() {
            const active = isActive(job);
            const agent = active && job.agent ? job.agent : {};
            S.clear(steps);
            steps.hidden = !active;
            if (active) {
                const order = PHASE_ORDER[agent.kind === 'revise' ? 'revise' : 'create'];
                const current = order.indexOf(agent.phase);
                order.forEach((name, index) => {
                    const state = current < 0 || index > current ? 'pending' : index < current ? 'done' : 'current';
                    steps.appendChild(el('li', { class: 'stm-agent-step', dataset: { stm: 'agent-step', phase: name, state } }, icon(state === 'done' ? 'check' : 'info'), el('span', { text: t(PHASE_KEYS[name]) })));
                });
            }
            const total = Number(agent.scenes_total) || 0;
            scenesLine.hidden = !total;
            scenesLine.textContent = total ? t('storymaker.agent.scenes', { done: Number(agent.scenes_done) || 0, total }) : '';
            retryLine.hidden = !(active && retrying && eventsJob === job.id);
            cancelButton.hidden = !active;
            cancelButton.disabled = cancelling;
        }

        function checkLine(check) {
            const key = CHECK_KEYS[check.code] || 'storymaker.agent.check_unknown';
            return el('li', { class: 'stm-agent-check', dataset: { stm: 'agent-check', code: String(check.code || '') } },
                check.scene_id ? el('code', { text: String(check.scene_id) }) : null,
                el('span', { text: t(key) }));
        }

        function outcomeText(outcome, checks) {
            if (outcome === 'failed') return job.error && S.JOB_ERROR_CODES.indexOf(job.error.code) >= 0 ? S.errors.describeJob(t, job.error) : t(OUTCOME_KEYS.failed.text);
            if (outcome !== 'incomplete') return t(OUTCOME_KEYS.ready_unverified.text);
            const open = checks.filter(check => check.code === 'scene_unfilled').length;
            return open ? t(OUTCOME_KEYS.incomplete.text, { n: open }) : t('storymaker.agent.outcome_incomplete_some');
        }

        function paintOutcome() {
            const finished = !!job && (job.status === 'succeeded' || job.status === 'failed');
            const outcome = !finished ? '' : (job.agent && job.agent.outcome) || (job.status === 'failed' ? 'failed' : '');
            outcomeCard.hidden = !outcome;
            S.clear(outcomeCard);
            if (!outcome) return;
            const checks = checksOf();
            const keys = OUTCOME_KEYS[outcome] || OUTCOME_KEYS.incomplete;
            outcomeCard.dataset.outcome = outcome;
            outcomeCard.appendChild(el('h3', { text: t(keys.title) }));
            outcomeCard.appendChild(el('p', { text: outcomeText(outcome, checks) }));
            if (job.agent && job.agent.summary) outcomeCard.appendChild(el('p', { dataset: { stm: 'agent-summary' }, text: String(job.agent.summary) }));
            if (checks.length) outcomeCard.appendChild(el('ul', { class: 'stm-agent-checks' }, checks.map(checkLine)));
        }

        function paintVoiceActions() {
            const noVoice = !isActive(job) && checksOf().some(check => check.code === 'narration_unavailable');
            silentButton.hidden = !noVoice || !!inst.silentBackup;
            restoreButton.hidden = !inst.silentBackup;
            silentButton.disabled = inst.readonly || working;
            restoreButton.disabled = inst.readonly || working;
        }

        function paint() {
            if (disposed) return;
            const info = agentBlock(inst) || {};
            const enabled = info.enabled === true;
            const usable = enabled && info.available !== false;
            const revise = !storyIsBlank(inst.story);
            offLine.hidden = enabled;
            unavailableLine.hidden = !enabled || usable;
            form.hidden = !enabled;
            row.hidden = revise;
            promptLabel.textContent = t(revise ? 'storymaker.agent.prompt_revise' : 'storymaker.agent.prompt_start');
            promptBox.placeholder = revise ? '' : t('storymaker.projects.new_title_placeholder');
            promptBox.disabled = inst.readonly;
            fields.setDisabled(inst.readonly);
            startLabel.textContent = t(revise ? 'storymaker.agent.revise' : 'storymaker.agent.start');
            startButton.dataset.mode = revise ? 'revise' : 'start';
            startButton.disabled = inst.readonly || starting || isActive(job) || !usable;
            errorLine.textContent = errorText;
            errorLine.hidden = !errorText;
            paintRoute();
            paintState();
            paintProgress();
            paintOutcome();
            paintVoiceActions();
        }

        const off = [
            inst.bus.on('job', next => applyJob(next, false)),
            inst.bus.on('agent', takeEvent),
            inst.bus.on('story', paint),
            inst.bus.on('status', paint),
            inst.bus.on('readonly', paint)
        ];
        paint();

        return {
            el: root,
            paint,
            dispose() {
                disposed = true;
                clearTimeout(reloadTimer);
                off.forEach(fn => fn());
            }
        };
    }

    S.createAgentPanel = createAgentPanel;
})();
```

- [ ] **Step 3: Implement the API client.** Each edit uses a single-line anchor because these files are CRLF in the working tree.

**Edit** `ui/js/desktop/apps/storymaker-api.js`
Old:
```js
            async listJobs(id) {
```
New:
```js
            // {project_id, job}: a new agent job. With project_id it writes that blank project, without it the server creates one.
            agentStart(spec) { return json('POST', '/agent/jobs', { headers: jsonHeaders(), body: JSON.stringify(spec) }); },
            // {job}: an agent job that changes the story of this project
            agentRevise(id, instruction) {
                return json('POST', '/projects/' + encodeURIComponent(id) + '/agent/revise', { headers: jsonHeaders(), body: JSON.stringify({ instruction }) });
            },
            // {etag, issues, story, changed}: the scenes lose their speech, and the old story stays as a backup on the server
            makeSilent(id, etag) { return json('POST', '/projects/' + encodeURIComponent(id) + '/make-silent', { headers: { 'If-Match': etag } }); },
            // {etag, issues, story}: the backup of make-silent comes back
            restoreSpeech(id, etag) { return json('POST', '/projects/' + encodeURIComponent(id) + '/restore-speech', { headers: { 'If-Match': etag } }); },
            async listJobs(id) {
```

**Edit** `ui/js/desktop/apps/storymaker-api.js`
Old:
```js
            // events(projectId, onJob, onState) -> {close()}; onState('open' | 'error')
```
New:
```js
            // events(projectId, onJob, onState, onAgent) -> {close()}; onState('open' | 'error'); onAgent gets the body of each agent event
```

**Edit** `ui/js/desktop/apps/storymaker-api.js`
Old:
```js
            events(projectId, onJob, onState) {
```
New:
```js
            events(projectId, onJob, onState, onAgent) {
```

**Edit** `ui/js/desktop/apps/storymaker-api.js`
Old:
```js
                source.addEventListener('open', () => onState && onState('open'));
```
New:
```js
                source.addEventListener('agent', event => {
                    let body = null;
                    try { body = JSON.parse(event.data); } catch (_) { return; }
                    if (onAgent && body && typeof body === 'object' && typeof body.job_id === 'string') onAgent(body);
                });
                source.addEventListener('open', () => onState && onState('open'));
```

- [ ] **Step 4: Implement the composition root.**

**Edit** `ui/js/desktop/apps/storymaker.js`
Old:
```js
        Object.defineProperty(inst, 'readonly', { get() { return inst.ctxReadonly || inst.forcedReadonly || !!(inst.status && inst.status.read_only); } });
```
New:
```js
        Object.defineProperty(inst, 'readonly', { get() { return inst.agentLocked || desktopReadonly(inst); } });
```

**Edit** `ui/js/desktop/apps/storymaker.js`
Old:
```js
            handoffWish: new Set(),
```
New:
```js
            handoffWish: new Set(),
            agentLocked: false, silentBackup: false,
```

**Edit** `ui/js/desktop/apps/storymaker.js`
Old:
```js
    function notify(inst, text, kind) {
```
New:
```js
    // Read-only for a reason outside this project. The lock of an agent job comes on top of this and has its own banner.
    function desktopReadonly(inst) {
        return inst.ctxReadonly || inst.forcedReadonly || !!(inst.status && inst.status.read_only);
    }

    function notify(inst, text, kind) {
```

**Edit** `ui/js/desktop/apps/storymaker.js`
Old:
```js
        inst.bus.clear();
```
New:
```js
        inst.agentLocked = false;
        inst.silentBackup = false;
        inst.bus.clear();
```

**Edit** `ui/js/desktop/apps/storymaker.js`
Old:
```js
            buildEditor(inst);
```
New:
```js
            inst.silentBackup = !!data.silent_backup;
            buildEditor(inst);
```

**Edit** `ui/js/desktop/apps/storymaker.js`
Old:
```js
            inst.events = inst.api.events(inst.projectId, job => inst.bus.emit('job', job));
```
New:
```js
            inst.events = inst.api.events(inst.projectId, job => inst.bus.emit('job', job), null, event => inst.bus.emit('agent', event));
```

**Edit** `ui/js/desktop/apps/storymaker.js`
Old:
```js
            reloadPlayer: () => reloadPlayer(inst)
```
New:
```js
            reloadPlayer: () => reloadPlayer(inst),
            reloadProject: () => reloadProject(inst)
```

**Edit** `ui/js/desktop/apps/storymaker.js`
Old:
```js
        const voice = S.createVoicePanel(inst);
```
New:
```js
        const voice = S.createVoicePanel(inst);
        const agent = S.createAgentPanel(inst);
```

**Edit** `ui/js/desktop/apps/storymaker.js`
Old:
```js
inst.parts.voice = voice;
```
New:
```js
inst.parts.voice = voice; inst.parts.agent = agent;
```

**Edit** `ui/js/desktop/apps/storymaker.js`
Old:
```js
        }, icon('sparkle'), el('span', { text: t('storymaker.side_music') })));
```
New:
```js
        }, icon('sparkle'), el('span', { text: t('storymaker.side_music') })));
        sideTabs.push(el('button', {
            type: 'button', class: 'stm-tab', role: 'tab', dataset: { stm: 'side-tab', tab: 'agent' }, ariaSelected: String(inst.sideTab === 'agent'),
            onClick: () => setSideTab(inst, 'agent')
        }, icon('film'), el('span', { text: t('storymaker.side_agent') })));
```

**Edit** `ui/js/desktop/apps/storymaker.js`
Old:
```js
        side.appendChild(music.el);
```
New:
```js
        side.appendChild(music.el);
        side.appendChild(agent.el);
```

**Edit** `ui/js/desktop/apps/storymaker.js`
Old:
```js
        const add = (key, tone, text, actions) => area.appendChild(el('div', { class: 'stm-banner', role: 'status', dataset: { banner: key, tone } }, icon(tone === 'bad' ? 'alert' : 'info'), el('span', { text }), actions || null));
```
New:
```js
        const add = (key, tone, text, actions) => {
            // While only the agent's lock makes the editor read-only, the read-only banner would name the wrong reason.
            if (key === 'readonly' && !desktopReadonly(inst)) return;
            area.appendChild(el('div', { class: 'stm-banner', role: 'status', dataset: { banner: key, tone } }, icon(tone === 'bad' ? 'alert' : 'info'), el('span', { text }), actions || null));
        };
        if (inst.agentLocked) add('agent_locked', 'info', t('storymaker.banner.agent_locked'));
```

**Edit** `ui/js/desktop/apps/storymaker.js`
Old:
```js
        if (e.status === 412) { inst.conflict = true; inst.saveState = 'error'; }
```
New:
```js
        if (e.status === 412 || e.code === 'agent_job_active') { inst.conflict = true; inst.saveState = 'error'; }
```

**Edit** `ui/js/desktop/apps/storymaker.js`
Old:
```js
            inst.selected = S.clamp(inst.selected, 0, Math.max(0, S.model.scenesOf(inst.story).length - 1));
```
New:
```js
            inst.silentBackup = !!data.silent_backup;
            // The agent sets the title of the film, and the toolbar should show it.
            if (data.project) {
                inst.project = data.project;
                if (inst.parts.title) inst.parts.title.textContent = data.project.title || inst.projectId;
            }
            inst.selected = S.clamp(inst.selected, 0, Math.max(0, S.model.scenesOf(inst.story).length - 1));
```

- [ ] **Step 5: Implement the styles, the loader entry and the registration list.**

**Edit** `ui/css/desktop-app-storymaker.css`
Old:
```css
.stm-music-bad { color: var(--stm-bad-text); }
```
New:
```css
.stm-music-bad { color: var(--stm-bad-text); }
.stm-agent { display: none; padding: 0 var(--ds-space-3) var(--ds-space-3); }
.stm-app [data-sidetab="agent"] .stm-agent { display: grid; gap: var(--ds-space-3); }
.stm-app [data-sidetab="agent"] .stm-side > :not(.stm-tabs, .stm-agent) { display: none; }
.stm-agent .stm-field { margin: 0; }
.stm-agent-form { display: grid; gap: var(--ds-space-3); }
.stm-agent-row { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); gap: var(--ds-space-2); }
.stm-agent-steps, .stm-agent-checks { display: grid; gap: 4px; margin: 0; padding: 0; list-style: none; }
.stm-agent-step { display: flex; align-items: center; gap: var(--ds-space-2); color: var(--vd-theme-muted); }
.stm-agent-step[data-state="current"] { color: var(--vd-text); font-weight: 600; }
.stm-agent-step[data-state="done"] { color: var(--stm-ok-text); }
.stm-agent-outcome { display: grid; gap: var(--ds-space-2); padding: var(--ds-space-3); border: 1px solid var(--vd-mat-card-border, var(--vd-theme-border)); border-radius: var(--vd-mat-card-radius, var(--ds-radius-md)); background: var(--vd-mat-card, var(--vd-theme-panel-bg)); }
.stm-agent-outcome[data-outcome="ready_unverified"] { border-color: var(--stm-ok); }
.stm-agent-outcome[data-outcome="incomplete"] { border-color: var(--stm-warn); }
.stm-agent-outcome[data-outcome="failed"] { border-color: var(--stm-bad); }
.stm-agent-outcome h3, .stm-agent-outcome p { margin: 0; }
.stm-agent-outcome h3 { color: var(--vd-mat-heading-color, var(--vd-text)); font-size: 14px; }
.stm-agent-check { display: flex; flex-wrap: wrap; align-items: baseline; gap: var(--ds-space-2); font-size: 12px; }
.stm-agent-check code { font-family: var(--stm-mono); }
```

**Edit** `ui/js/desktop/core/module-loader.js`
Old:
```js
'/js/desktop/apps/storymaker-music.js', 
```
New:
```js
'/js/desktop/apps/storymaker-music.js', '/js/desktop/apps/storymaker-agent.js', 
```

**Edit** `ui/desktop_storymaker_registration_test.go`
Old:
```go
"voice", "music", "dialog", "handoff"}
```
New:
```go
"voice", "music", "agent", "dialog", "handoff"}
```

- [ ] **Step 6: Test, the tour step.** The stub replaces the run function of the agent queue, so no model is called and no money is spent. It runs in the existing tour, with the code scene selected.

**Edit** `internal/server/storymaker_app_browser_test.go`
Old:
```go
	"aurago/internal/storymaker"
```
New:
```go
	"aurago/internal/llm"
	"aurago/internal/storymaker"
	"aurago/internal/storymaker/render"
```

**Edit** `internal/server/storymaker_app_browser_test.go`
Old:
```go
	"github.com/go-rod/rod/lib/launcher"
```
New:
```go
	"github.com/go-rod/rod/lib/launcher"
	"github.com/sashabaranov/go-openai"
```

**Edit** `internal/server/storymaker_app_browser_test.go`
Old:
```go
func TestStorymakerAppRealServerBrowser(t *testing.T) {
```
New:
```go
// smappInstallAgentStub switches the agent on and gives the agent queue a stub run function: no model call and no cost. The stub
// sets the transition of the last scene, reports the new story etag, and waits until the job is cancelled. The client is never
// called; it only makes the main route count as configured.
func smappInstallAgentStub(t *testing.T, s *Server) {
	t.Helper()
	s.Cfg.Storymaker.Agent.Enabled = true
	s.Cfg.LLM.Model = "tour-model"
	s.LLMClient = llm.WrapOpenAIClient(openai.NewClient("test-only"))
	rt, err := s.storymakerRuntime()
	if err != nil {
		t.Fatal(err)
	}
	rt.agent.Close()
	stub, err := render.NewManager(render.ManagerOptions{
		Dir:       filepath.Join(storymakerDir(rt.dataDir), "agent-tour"),
		Kinds:     []string{render.KindAgent},
		MaxQueued: 8,
		Workers:   1,
		Run: func(ctx context.Context, job render.Job, payload any, h *render.Handle) (render.Output, error) {
			data, etag, err := rt.projects.LoadStory(ctx, job.ProjectID)
			if err != nil {
				return render.Output{}, err
			}
			var doc map[string]any
			if err := json.Unmarshal(data, &doc); err != nil {
				return render.Output{}, err
			}
			scenes, _ := doc["scenes"].([]any)
			if len(scenes) == 0 {
				return render.Output{}, fmt.Errorf("the story of %s has no scene", job.ProjectID)
			}
			scenes[len(scenes)-1].(map[string]any)["transition"] = "cut"
			next, err := json.Marshal(doc)
			if err != nil {
				return render.Output{}, err
			}
			saved, err := rt.projects.SaveStory(ctx, job.ProjectID, next, etag)
			if err != nil {
				return render.Output{}, err
			}
			h.SetAgent(storymaker.AgentJobInfo{Kind: "revise", Phase: storymaker.PhaseRevise, ScenesDone: 1, ScenesTotal: len(scenes), StoryETag: saved})
			<-ctx.Done()
			return render.Output{}, ctx.Err()
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	rt.agent = stub
}

func TestStorymakerAppRealServerBrowser(t *testing.T) {
```

**Edit** `internal/server/storymaker_app_browser_test.go`
Old:
```go
	page, origin := smappPage(t, s)
```
New:
```go
	smappInstallAgentStub(t, s)
	page, origin := smappPage(t, s)
```

**Edit** `internal/server/storymaker_app_browser_test.go`
Old:
```go
	// Four side tabs stay inside the side panel: the strip wraps instead of clipping the last tab.
```
New:
```go
	// The AI author: a revise job from the panel. The stub writes the story while the job runs, so the editor locks and shows the
	// lock banner, and the panel reloads by itself: the transition select of the selected code scene shows the stub's value.
	// Cancel ends the job, the lock goes and the written scene stays.
	smappClick(t, page, `[data-stm="side-tab"][data-tab="agent"]`)
	smappWait(t, page, `()=>{const b=document.querySelector('[data-stm="agent-start"]');return !!b&&b.dataset.mode==='revise'&&!b.disabled;}`)
	page.MustEval(setField, `[data-stm="agent-prompt"]`, "Use a cut into the code scene.")
	smappClick(t, page, `[data-stm="agent-start"]`)
	smappWait(t, page, `()=>!!document.querySelector('[data-banner="agent_locked"]') && document.querySelector('[data-stm="transition"]')?.value==='cut' && !!document.querySelector('[data-stm="agent-step"][data-phase="revise"][data-state="current"]')`)
	smappClick(t, page, `[data-stm="agent-cancel"]`)
	smappWait(t, page, `()=>!document.querySelector('[data-banner="agent_locked"]') && document.querySelector('[data-stm="agent-state"]')?.dataset.state==='cancelled'`)
	if got := smappStored(t, origin, id)["scenes"].([]any)[1].(map[string]any)["transition"]; got != "cut" {
		t.Fatalf("the transition after the cancelled job = %v, want the stub's cut", got)
	}
	smappNoErrors(t, page)

	// Five side tabs stay inside the side panel: the strip wraps instead of clipping the last tab.
```

- [ ] **Step 7: Run.**

`npm run build:ui`, then `npm run check:ui`. Expected: both exit 0.

`GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./ui -run '^(TestDesktopStorymakerRegistration|TestStorymakerSourcesAreSafeAndSmall|TestStorymakerLoaderListsEveryScript|TestStorymakerStylesheetUsesOnlyTokens|TestStorymakerAppLoadsNothingExternalAndAvoidsFilteredClasses)$' -count=1`. Expected: PASS.

`TestStorymakerAppUsesOnlyKeysItHas` stays red until task D6 adds the keys. This is expected, so do not add keys here.

With the shell sandbox off: `GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' AURAGO_RUN_BROWSER_SMOKE=1 go test ./internal/server -run '^TestStorymakerAppRealServerBrowser$' -count=1 -timeout 15m`. Expected: PASS.

- [ ] **Step 8: Commit.** Stage `ui/js/desktop/apps/storymaker-agent.js ui/js/desktop/apps/storymaker.js ui/js/desktop/apps/storymaker-api.js ui/css/desktop-app-storymaker.css ui/js/desktop/core/module-loader.js ui/desktop_storymaker_registration_test.go internal/server/storymaker_app_browser_test.go`, plus `ui/js/desktop/bundles/main.bundle.js` only if `build:ui` changed it. Message: `feat(storymaker): add the AI author panel with lock, reload and restore speech`

### Task D5: Project-list start
**Batch:** D  **Depends on:** D4
**Files:**
Modify `ui/js/desktop/apps/storymaker-agent.js`
Modify `ui/js/desktop/apps/storymaker-projects.js`
Modify `ui/js/desktop/apps/storymaker.js`
Modify `ui/css/desktop-app-storymaker.css`
Test `internal/server/storymaker_app_browser_test.go`

- [ ] **Step 1: Impact.** Run impact for `createProjects`, `showProjects` and `TestStorymakerAppRealServerBrowser`. Report callers and risk. Warn on HIGH or CRITICAL.

- [ ] **Step 2: Implement the start card.**

**Edit** `ui/js/desktop/apps/storymaker-agent.js`
Old:
```js
// Everything from the server reaches the page as text only. Classic script, publishes S.createAgentPanel.
```
New:
```js
// It also builds the start card of the project list. Everything from the server reaches the page as text only. Classic script,
// publishes S.createAgentPanel and S.createAgentStart.
```

**Edit** `ui/js/desktop/apps/storymaker-agent.js`
Old:
```js
    S.createAgentPanel = createAgentPanel;
```
New:
```js
    // The start card of the project list: a topic becomes a new project with a running agent job. It stays hidden while the agent is
    // switched off. inst: {t, domId(name), readonly, agentStatus, startAgent(spec)}
    function createAgentStart(inst) {
        const { el, icon } = S;
        const t = inst.t;
        let busy = false;
        const fields = createFields(inst, 'agent-new', inst.agentStatus);
        const topicId = inst.domId('agent-new-topic');
        const aspectId = inst.domId('agent-new-aspect');
        const topic = el('textarea', { class: 'stm-textarea stm-agent-start-topic', id: topicId, rows: '2', maxlength: MAX_TEXT, placeholder: t('storymaker.projects.new_title_placeholder'), dataset: { stm: 'agent-new-topic' } });
        const aspect = el('select', { class: 'stm-select', id: aspectId, dataset: { stm: 'agent-new-aspect' } },
            el('option', { value: '16:9', text: t('storymaker.aspect_16_9') }),
            el('option', { value: '9:16', text: t('storymaker.aspect_9_16') }),
            el('option', { value: '1:1', text: t('storymaker.aspect_1_1') }));
        const startButton = el('button', { type: 'submit', class: 'stm-btn stm-btn-primary', dataset: { stm: 'agent-new-start' } }, icon('sparkle'), el('span', { text: t('storymaker.agent.start') }));
        const note = el('p', { class: 'stm-hint stm-agent-start-wide', hidden: true, dataset: { stm: 'agent-new-note' }, text: t('storymaker.agent.unavailable') });
        const errorLine = el('p', { class: 'stm-form-error', role: 'alert', hidden: true, dataset: { stm: 'agent-new-error' } });
        const root = el('form', { class: 'stm-new stm-agent-start', hidden: true, onSubmit: event => { event.preventDefault(); submit(); } },
            el('h3', { class: 'stm-agent-start-wide', text: t('storymaker.agent.new_title') }),
            el('div', { class: 'stm-field stm-agent-start-wide' }, el('label', { class: 'stm-label', for: topicId, text: t('storymaker.agent.prompt_start') }), topic),
            fields.lengthField, fields.languageField,
            el('div', { class: 'stm-field' }, el('label', { class: 'stm-label', for: aspectId, text: t('storymaker.projects.new_format') }), aspect),
            el('div', { class: 'stm-actions' }, startButton),
            note, errorLine);

        function setError(text) {
            errorLine.textContent = text || '';
            errorLine.hidden = !text;
        }

        function refresh() {
            const info = inst.agentStatus || {};
            const usable = info.enabled === true && info.available !== false;
            root.hidden = info.enabled !== true;
            note.hidden = info.enabled !== true || usable;
            topic.disabled = inst.readonly;
            aspect.disabled = inst.readonly;
            fields.setDisabled(inst.readonly);
            startButton.disabled = inst.readonly || busy || !usable;
        }

        async function submit() {
            if (busy || inst.readonly) return;
            const text = topic.value.trim();
            if (!text) { setError(t('storymaker.agent.prompt_required')); topic.focus(); return; }
            busy = true;
            setError('');
            refresh();
            try {
                await inst.startAgent({ topic: text, length_seconds: fields.seconds(), language: fields.language(), aspect: aspect.value });
            } catch (err) {
                if (!(err && err.aborted)) setError(S.errors.describe(t, err));
            } finally {
                busy = false;
                refresh();
            }
        }

        refresh();
        return { el: root, refresh };
    }

    S.createAgentPanel = createAgentPanel;
    S.createAgentStart = createAgentStart;
```

**Edit** `ui/js/desktop/apps/storymaker-projects.js`
Old:
```js
    // inst: {t, readonly, language, createProject(spec), openProject(id), deleteProject(project), confirm(title, message)}
```
New:
```js
    // inst: {t, readonly, language, agentStatus, createProject(spec), startAgent(spec), openProject(id), deleteProject(project), confirm(title, message)}
```

**Edit** `ui/js/desktop/apps/storymaker-projects.js`
Old:
```js
        const root = el('section', { class: 'stm-projects', ariaLabel: t('storymaker.projects.title') },
```
New:
```js
        const agentStart = S.createAgentStart(inst);
        const root = el('section', { class: 'stm-projects', ariaLabel: t('storymaker.projects.title') },
```

**Edit** `ui/js/desktop/apps/storymaker-projects.js`
Old:
```js
            form, loading, empty, list);
```
New:
```js
            agentStart.el, form, loading, empty, list);
```

**Edit** `ui/js/desktop/apps/storymaker-projects.js`
Old:
```js
            refresh() { syncDisabled(); },
```
New:
```js
            refresh() { syncDisabled(); agentStart.refresh(); },
```

**Edit** `ui/js/desktop/apps/storymaker.js`
Old:
```js
            createProject: spec => createProject(inst, spec),
```
New:
```js
            get agentStatus() { return inst.status && inst.status.agent ? inst.status.agent : null; },
            createProject: spec => createProject(inst, spec),
            startAgent: spec => startAgent(inst, spec),
```

**Edit** `ui/js/desktop/apps/storymaker.js`
Old:
```js
    async function deleteProject(inst, project) {
```
New:
```js
    // A film from a topic: the server creates the project and queues the job, and the editor opens on the panel that follows it.
    async function startAgent(inst, spec) {
        const result = await inst.api.agentStart(spec);
        if (inst.disposed) return;
        inst.sideTab = 'agent';
        await openProject(inst, result.project_id);
    }

    async function deleteProject(inst, project) {
```

**Edit** `ui/css/desktop-app-storymaker.css`
Old:
```css
.stm-music-bad { color: var(--stm-bad-text); }
```
New:
```css
.stm-music-bad { color: var(--stm-bad-text); }
.stm-agent-start { grid-template-columns: repeat(3, minmax(0, 1fr)) auto; }
.stm-agent-start-wide { grid-column: 1 / -1; }
.stm-agent-start h3 { margin: 0; color: var(--vd-mat-heading-color, var(--vd-text)); font-size: 14px; }
.stm-agent-start .stm-agent-start-topic { min-height: 64px; }
```

- [ ] **Step 3: Test, the tour step extended.** The list is empty here, because the tour has just deleted its project. The stub from the existing tour step handles this job too.

**Edit** `internal/server/storymaker_app_browser_test.go`
Old:
```go
	// Closing the window tears the app down without a script error.
```
New:
```go
	// The start card of the project list: a topic creates a project, and the editor opens on the AI author panel with the job running.
	smappWait(t, page, `()=>{const b=document.querySelector('[data-stm="agent-new-start"]');return !!b&&!b.disabled;}`)
	page.MustEval(setField, `[data-stm="agent-new-topic"]`, "What is a firewall?")
	smappClick(t, page, `[data-stm="agent-new-start"]`)
	smappWait(t, page, `()=>document.querySelector('[data-stm="side-tab"][data-tab="agent"]')?.getAttribute('aria-selected')==='true' && !!document.querySelector('[data-banner="agent_locked"]')`)
	if left := smappProjectIDs(t, origin); len(left) != 1 {
		t.Fatalf("projects after the start card = %v, want one new project", left)
	}
	smappClick(t, page, `[data-stm="agent-cancel"]`)
	smappWait(t, page, `()=>!document.querySelector('[data-banner="agent_locked"]') && document.querySelector('[data-stm="agent-state"]')?.dataset.state==='cancelled'`)
	smappNoErrors(t, page)

	// Closing the window tears the app down without a script error.
```

The Node harness of the new-project test loads only the core and the projects file, so it needs a stub of the start card.

**Edit** `ui/desktop_storymaker_model_test.go`
Old:
```js
  const view = S.createProjects(inst);
  view.setProjects([]);
  view.el.find('new-title').value = 'Film';
```
New:
```js
  S.createAgentStart = () => ({ el: document.createElement('div'), refresh() {} });
  const view = S.createProjects(inst);
  view.setProjects([]);
  view.el.find('new-title').value = 'Film';
```

- [ ] **Step 4: Run.**

`npm run build:ui`, then `npm run check:ui`. Expected: both exit 0.

`GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./ui -run '^(TestStorymakerSourcesAreSafeAndSmall|TestStorymakerLoaderListsEveryScript|TestStorymakerStylesheetUsesOnlyTokens|TestStorymakerAppLoadsNothingExternalAndAvoidsFilteredClasses)$' -count=1`. Expected: PASS.

`TestStorymakerAppUsesOnlyKeysItHas` stays red until task D6.

With the shell sandbox off: `GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' AURAGO_RUN_BROWSER_SMOKE=1 go test ./internal/server -run '^TestStorymakerAppRealServerBrowser$' -count=1 -timeout 15m`. Expected: PASS.

- [ ] **Step 5: Commit.** Stage `ui/js/desktop/apps/storymaker-agent.js ui/js/desktop/apps/storymaker-projects.js ui/js/desktop/apps/storymaker.js ui/css/desktop-app-storymaker.css internal/server/storymaker_app_browser_test.go`, plus `ui/js/desktop/bundles/main.bundle.js` only if `build:ui` changed it. Message: `feat(storymaker): start an AI author film from the project list`

### Task D6: Locales and `model_error`
**Batch:** D  **Depends on:** D5
**Files:**
Modify `ui/js/desktop/apps/storymaker-core.js`
Modify `ui/lang/storymaker/cs.json`
Modify `ui/lang/storymaker/da.json`
Modify `ui/lang/storymaker/de.json`
Modify `ui/lang/storymaker/el.json`
Modify `ui/lang/storymaker/en.json`
Modify `ui/lang/storymaker/es.json`
Modify `ui/lang/storymaker/fr.json`
Modify `ui/lang/storymaker/hi.json`
Modify `ui/lang/storymaker/it.json`
Modify `ui/lang/storymaker/ja.json`
Modify `ui/lang/storymaker/nl.json`
Modify `ui/lang/storymaker/no.json`
Modify `ui/lang/storymaker/pl.json`
Modify `ui/lang/storymaker/pt.json`
Modify `ui/lang/storymaker/sv.json`
Modify `ui/lang/storymaker/zh.json`

- [ ] **Step 1: Impact.** Run impact for `describeError`. Report callers and risk. Warn on HIGH or CRITICAL.

- [ ] **Step 2: Implement the error mapping.** Without the table, `daily_limit_reached` (429) would read as "busy" and `model_too_small` (422) as an invalid story.

**Edit** `ui/js/desktop/apps/storymaker-core.js`
Old:
```js
        else if (e.code === 'story_unreadable') key = 'storymaker.error.story_unreadable';
```
New:
```js
        else if (AGENT_ERROR_KEYS.has(e.code)) key = AGENT_ERROR_KEYS.get(e.code);
        else if (e.code === 'story_unreadable') key = 'storymaker.error.story_unreadable';
```

**Edit** `ui/js/desktop/apps/storymaker-core.js`
Old:
```js
    // The job error codes of the spec (Error handling) that a render or a narrate job can end with.
```
New:
```js
    // The refusals of the agent routes. They come before the general status rules, because a 429 or a 422 here means something else.
    const AGENT_ERROR_KEYS = new Map([
        ['agent_disabled', 'storymaker.agent.off'],
        ['agent_job_active', 'storymaker.error.agent_job_active'],
        ['story_not_empty', 'storymaker.error.story_not_empty'],
        ['model_too_small', 'storymaker.error.model_too_small'],
        ['llm_unavailable', 'storymaker.error.llm_unavailable'],
        ['daily_limit_reached', 'storymaker.error.daily_limit_reached'],
        ['invalid_topic', 'storymaker.agent.prompt_required'],
        ['invalid_instruction', 'storymaker.agent.prompt_required']
    ]);

    // The job error codes of the spec (Error handling) that a render, a narrate or an agent job can end with.
```

**Edit** `ui/js/desktop/apps/storymaker-core.js`
Old:
```js
'music_failed', 'music_incomplete'];
```
New:
```js
'music_failed', 'music_incomplete', 'model_error'];
```

- [ ] **Step 3: Implement the 16 locales.** Each edit replaces the last key of the file. These JSON files are `eol=lf`, so keep them LF and never convert them.

**Edit** `ui/lang/storymaker/cs.json`
Old:
```json
  "storymaker.inspector.silent": "Němá scéna: tento komentář se nečte ani nezobrazuje"
```
New:
```json
  "storymaker.inspector.silent": "Němá scéna: tento komentář se nečte ani nezobrazuje",
  "storymaker.side_agent": "AI autor",
  "storymaker.banner.agent_locked": "AI autor pracuje na tomto příběhu. Úpravy jsou uzamčené, dokud neskončí.",
  "storymaker.agent.new_title": "Nechte AI autora napsat film",
  "storymaker.agent.prompt_start": "Co má film vysvětlit?",
  "storymaker.agent.prompt_revise": "Co se má změnit?",
  "storymaker.agent.prompt_required": "Popište prosím, co chcete.",
  "storymaker.agent.length": "Délka v sekundách",
  "storymaker.agent.language": "Jazyk filmu",
  "storymaker.agent.start": "Napsat příběh",
  "storymaker.agent.revise": "Upravit příběh",
  "storymaker.agent.cancel": "Zastavit autora",
  "storymaker.agent.off": "AI autor je v konfiguraci vypnutý.",
  "storymaker.agent.unavailable": "Pro autora teď není k dispozici žádný model AI.",
  "storymaker.agent.save_first": "Poslední úpravy se nepodařilo uložit, proto se nic nespustilo.",
  "storymaker.agent.state_cancelled": "Zastaveno. Platné scény v příběhu zůstávají.",
  "storymaker.agent.state_interrupted": "Autora přerušil restart. Platné scény zůstávají a zbytek může dokončit úprava.",
  "storymaker.agent.phase_outline": "Plánování scén",
  "storymaker.agent.phase_cast": "Výběr postav",
  "storymaker.agent.phase_fill": "Inscenace scén",
  "storymaker.agent.phase_check": "Kontrola filmu",
  "storymaker.agent.phase_revise": "Provádění změny",
  "storymaker.agent.phase_narrating": "Generování komentáře",
  "storymaker.agent.scenes": "Scéna {done} z {total}",
  "storymaker.agent.model_retry": "Model dostává dotaz znovu.",
  "storymaker.agent.mode_compact": "Kompaktní režim pro malý model",
  "storymaker.agent.mode_text": "Volání nástrojů jako text",
  "storymaker.agent.outcome_ready_title": "Váš příběh je hotový. Podívejte se na něj prosím jednou.",
  "storymaker.agent.outcome_ready_text": "Scény, prvky a načasování byly zkontrolovány. Překryvy a umístění textu se automaticky nekontrolovaly.",
  "storymaker.agent.outcome_incomplete_title": "Váš příběh je hotový jen částečně.",
  "storymaker.agent.outcome_incomplete_text": "Počet scén, které ještě potřebují práci: {n}. Platné scény jsou uložené.",
  "storymaker.agent.outcome_incomplete_some": "Některé části ještě potřebují práci. Platné scény jsou uložené.",
  "storymaker.agent.outcome_failed_title": "Agent tento příběh nedokázal napsat.",
  "storymaker.agent.outcome_failed_text": "Zkuste to znovu s kratším tématem, nebo zkontrolujte nastavení modelu.",
  "storymaker.agent.finish_instruction": "Dokonči scény, které ještě potřebují práci.",
  "storymaker.agent.check_layout_unverified": "Překryvy a umístění textu nebyly zkontrolovány. Podívejte se na film jednou.",
  "storymaker.agent.check_compile_unverified": "K měření načasování nebyl k dispozici žádný prohlížeč. Zkontrolujte délku v náhledu.",
  "storymaker.agent.check_narration_unavailable": "Pro tento jazyk není připraven žádný hlas. Nastavte hlas a vygenerujte komentář, nebo exportujte bez hlasu.",
  "storymaker.agent.check_narration_failed": "Komentář se nepodařilo vygenerovat. Zkuste to znovu v panelu Hlas.",
  "storymaker.agent.check_narration_budget": "Limit znaků komentáře je vyčerpán. Zkuste to zítra, nebo limit zvyšte.",
  "storymaker.agent.check_narration_pending": "Komentář se stále generuje. Exportujte, až bude hotový.",
  "storymaker.agent.check_corrections_exhausted": "Scéně došly pokusy o opravu. Požádejte autora, ať ji dokončí.",
  "storymaker.agent.check_model_error": "Model přestal odpovídat. Požádejte autora, ať příběh dokončí.",
  "storymaker.agent.check_scene_unfilled": "Tato scéna ještě potřebuje práci. Požádejte autora, ať ji dokončí.",
  "storymaker.agent.check_compile_failed": "Náhled nedokázal tento film připravit. Požádejte autora o opravu, nebo se podívejte do seznamu problémů.",
  "storymaker.agent.check_over_target": "Film je delší, než jste chtěli. Požádejte autora, ať ho zkrátí.",
  "storymaker.agent.check_over_budget": "Scéna má více slov, než bylo plánováno. Požádejte autora, ať ji zkrátí.",
  "storymaker.agent.check_unknown": "Něco ještě potřebuje pozornost.",
  "storymaker.agent.make_silent": "Exportovat bez hlasu",
  "storymaker.agent.make_silent_done": "Řeč je teď text na obrazovce a film lze exportovat.",
  "storymaker.agent.restore_speech": "Obnovit řeč",
  "storymaker.agent.refused_invalid_story": "Příběh obsahuje chyby, takže ho autor nemůže upravit. Nejprve je opravte.",
  "storymaker.error.agent_job_active": "AI autor pracuje na tomto příběhu. Zkuste to znovu, až skončí.",
  "storymaker.error.story_not_empty": "Tento příběh už má obsah. Popište místo toho změnu.",
  "storymaker.error.model_too_small": "Nastavený model je pro AI autora příliš malý. V konfiguraci zvolte větší.",
  "storymaker.error.llm_unavailable": "Teď není k dispozici žádný model AI.",
  "storymaker.error.daily_limit_reached": "Denní limit AI autora je vyčerpán. Zkuste to zítra.",
  "storymaker.job_error.model_error": "Model AI nedal použitelnou odpověď."
```

**Edit** `ui/lang/storymaker/da.json`
Old:
```json
  "storymaker.inspector.silent": "Tavs scene: denne fortællertekst bliver hverken læst op eller vist"
```
New:
```json
  "storymaker.inspector.silent": "Tavs scene: denne fortællertekst bliver hverken læst op eller vist",
  "storymaker.side_agent": "AI-forfatter",
  "storymaker.banner.agent_locked": "AI-forfatteren arbejder på denne historie. Redigering er låst, indtil den er færdig.",
  "storymaker.agent.new_title": "Lad AI-forfatteren skrive en film",
  "storymaker.agent.prompt_start": "Hvad skal filmen forklare?",
  "storymaker.agent.prompt_revise": "Hvad skal ændres?",
  "storymaker.agent.prompt_required": "Beskriv venligst, hvad du ønsker.",
  "storymaker.agent.length": "Længde i sekunder",
  "storymaker.agent.language": "Filmens sprog",
  "storymaker.agent.start": "Skriv historien",
  "storymaker.agent.revise": "Ret historien",
  "storymaker.agent.cancel": "Stop forfatteren",
  "storymaker.agent.off": "AI-forfatteren er slået fra i konfigurationen.",
  "storymaker.agent.unavailable": "Ingen AI-model er tilgængelig for forfatteren lige nu.",
  "storymaker.agent.save_first": "Dine seneste ændringer kunne ikke gemmes, så intet blev startet.",
  "storymaker.agent.state_cancelled": "Stoppet. Gyldige scener bliver i historien.",
  "storymaker.agent.state_interrupted": "En genstart afbrød forfatteren. Gyldige scener bliver, og en rettelse kan gøre resten færdig.",
  "storymaker.agent.phase_outline": "Planlægger scenerne",
  "storymaker.agent.phase_cast": "Vælger personerne",
  "storymaker.agent.phase_fill": "Iscenesætter scenerne",
  "storymaker.agent.phase_check": "Kontrollerer filmen",
  "storymaker.agent.phase_revise": "Udfører ændringen",
  "storymaker.agent.phase_narrating": "Genererer fortællingen",
  "storymaker.agent.scenes": "Scene {done} af {total}",
  "storymaker.agent.model_retry": "Modellen bliver spurgt igen.",
  "storymaker.agent.mode_compact": "Kompakt tilstand til en lille model",
  "storymaker.agent.mode_text": "Værktøjskald som tekst",
  "storymaker.agent.outcome_ready_title": "Din historie er klar. Se den venligst igennem én gang.",
  "storymaker.agent.outcome_ready_text": "Scener, elementer og timing er kontrolleret. Overlap og tekstens pladsforhold blev ikke kontrolleret automatisk.",
  "storymaker.agent.outcome_incomplete_title": "Din historie er delvist færdig.",
  "storymaker.agent.outcome_incomplete_text": "{n} scener mangler stadig arbejde. Gyldige scener er gemt.",
  "storymaker.agent.outcome_incomplete_some": "Nogle dele mangler stadig arbejde. Gyldige scener er gemt.",
  "storymaker.agent.outcome_failed_title": "Agenten kunne ikke skrive denne historie.",
  "storymaker.agent.outcome_failed_text": "Prøv igen med et kortere emne, eller kontrollér modelindstillingerne.",
  "storymaker.agent.finish_instruction": "Gør de scener færdige, der stadig mangler arbejde.",
  "storymaker.agent.check_layout_unverified": "Overlap og tekstens pladsforhold blev ikke kontrolleret. Se filmen igennem én gang.",
  "storymaker.agent.check_compile_unverified": "Ingen browser var tilgængelig til at måle timingen. Kontrollér længden i forhåndsvisningen.",
  "storymaker.agent.check_narration_unavailable": "Ingen stemme er klar til dette sprog. Opsæt en og generér fortællingen, eller eksportér uden stemme.",
  "storymaker.agent.check_narration_failed": "Fortællingen kunne ikke genereres. Prøv igen i panelet Stemme.",
  "storymaker.agent.check_narration_budget": "Tegngrænsen for fortælling er nået. Prøv igen i morgen, eller hæv grænsen.",
  "storymaker.agent.check_narration_pending": "Fortællingen genereres stadig. Eksportér, når den er færdig.",
  "storymaker.agent.check_corrections_exhausted": "En scene har brugt alle rettelsesforsøg. Bed forfatteren om at gøre den færdig.",
  "storymaker.agent.check_model_error": "Modellen holdt op med at svare. Bed forfatteren om at gøre historien færdig.",
  "storymaker.agent.check_scene_unfilled": "Denne scene mangler stadig arbejde. Bed forfatteren om at gøre den færdig.",
  "storymaker.agent.check_compile_failed": "Forhåndsvisningen kunne ikke forberede filmen. Bed forfatteren om at rette den, eller se listen over problemer.",
  "storymaker.agent.check_over_target": "Filmen er længere, end du bad om. Bed forfatteren om at forkorte den.",
  "storymaker.agent.check_over_budget": "En scene har flere ord end planlagt. Bed forfatteren om at forkorte den.",
  "storymaker.agent.check_unknown": "Noget skal stadig ses efter.",
  "storymaker.agent.make_silent": "Eksportér uden stemme",
  "storymaker.agent.make_silent_done": "Talen er nu tekst på skærmen, og filmen kan eksporteres.",
  "storymaker.agent.restore_speech": "Gendan tale",
  "storymaker.agent.refused_invalid_story": "Historien har fejl, så forfatteren kan ikke rette den. Ret dem først.",
  "storymaker.error.agent_job_active": "AI-forfatteren arbejder på denne historie. Prøv igen, når den er færdig.",
  "storymaker.error.story_not_empty": "Denne historie har allerede indhold. Beskriv en ændring i stedet.",
  "storymaker.error.model_too_small": "Den valgte model er for lille til AI-forfatteren. Vælg en større i konfigurationen.",
  "storymaker.error.llm_unavailable": "Ingen AI-model er tilgængelig lige nu.",
  "storymaker.error.daily_limit_reached": "Den daglige grænse for AI-forfatteren er nået. Prøv igen i morgen.",
  "storymaker.job_error.model_error": "AI-modellen gav ikke et brugbart svar."
```

**Edit** `ui/lang/storymaker/de.json`
Old:
```json
  "storymaker.inspector.silent": "Stumme Szene: Dieser Sprechtext wird weder gesprochen noch angezeigt"
```
New:
```json
  "storymaker.inspector.silent": "Stumme Szene: Dieser Sprechtext wird weder gesprochen noch angezeigt",
  "storymaker.side_agent": "KI-Autor",
  "storymaker.banner.agent_locked": "Der KI-Autor arbeitet an dieser Geschichte. Das Bearbeiten ist gesperrt, bis er fertig ist.",
  "storymaker.agent.new_title": "Lass den KI-Autor einen Film schreiben",
  "storymaker.agent.prompt_start": "Was soll der Film erklären?",
  "storymaker.agent.prompt_revise": "Was soll sich ändern?",
  "storymaker.agent.prompt_required": "Bitte beschreibe, was du möchtest.",
  "storymaker.agent.length": "Länge in Sekunden",
  "storymaker.agent.language": "Sprache des Films",
  "storymaker.agent.start": "Geschichte schreiben",
  "storymaker.agent.revise": "Geschichte überarbeiten",
  "storymaker.agent.cancel": "Autor stoppen",
  "storymaker.agent.off": "Der KI-Autor ist in der Konfiguration ausgeschaltet.",
  "storymaker.agent.unavailable": "Für den Autor ist gerade kein KI-Modell verfügbar.",
  "storymaker.agent.save_first": "Deine letzten Änderungen konnten nicht gespeichert werden, deshalb wurde nichts gestartet.",
  "storymaker.agent.state_cancelled": "Gestoppt. Gültige Szenen bleiben in der Geschichte.",
  "storymaker.agent.state_interrupted": "Ein Neustart hat den Autor unterbrochen. Gültige Szenen bleiben, und eine Überarbeitung kann den Rest fertigstellen.",
  "storymaker.agent.phase_outline": "Szenen planen",
  "storymaker.agent.phase_cast": "Personen auswählen",
  "storymaker.agent.phase_fill": "Szenen inszenieren",
  "storymaker.agent.phase_check": "Film prüfen",
  "storymaker.agent.phase_revise": "Änderung ausführen",
  "storymaker.agent.phase_narrating": "Sprechtext erzeugen",
  "storymaker.agent.scenes": "Szene {done} von {total}",
  "storymaker.agent.model_retry": "Das Modell wird erneut gefragt.",
  "storymaker.agent.mode_compact": "Kompaktmodus für ein kleines Modell",
  "storymaker.agent.mode_text": "Werkzeugaufrufe als Text",
  "storymaker.agent.outcome_ready_title": "Deine Geschichte ist fertig. Bitte sieh sie dir einmal an.",
  "storymaker.agent.outcome_ready_text": "Szenen, Assets und Timing wurden geprüft. Überlappungen und die Textpassung wurden nicht automatisch geprüft.",
  "storymaker.agent.outcome_incomplete_title": "Deine Geschichte ist teilweise fertig.",
  "storymaker.agent.outcome_incomplete_text": "{n} Szenen brauchen noch Arbeit. Gültige Szenen sind gespeichert.",
  "storymaker.agent.outcome_incomplete_some": "Einige Teile brauchen noch Arbeit. Gültige Szenen sind gespeichert.",
  "storymaker.agent.outcome_failed_title": "Der Agent konnte diese Geschichte nicht schreiben.",
  "storymaker.agent.outcome_failed_text": "Versuche es mit einem kürzeren Thema noch einmal, oder prüfe die Modelleinstellungen.",
  "storymaker.agent.finish_instruction": "Stelle die Szenen fertig, die noch Arbeit brauchen.",
  "storymaker.agent.check_layout_unverified": "Überlappungen und die Textpassung wurden nicht geprüft. Sieh dir den Film einmal an.",
  "storymaker.agent.check_compile_unverified": "Es war kein Browser verfügbar, um das Timing zu messen. Prüfe die Länge in der Vorschau.",
  "storymaker.agent.check_narration_unavailable": "Für diese Sprache ist keine Stimme bereit. Richte eine ein und erzeuge den Sprechtext, oder exportiere ohne Stimme.",
  "storymaker.agent.check_narration_failed": "Der Sprechtext konnte nicht erzeugt werden. Versuche es im Stimme-Panel noch einmal.",
  "storymaker.agent.check_narration_budget": "Das Zeichenlimit für den Sprechtext ist erreicht. Versuche es morgen noch einmal, oder erhöhe das Limit.",
  "storymaker.agent.check_narration_pending": "Der Sprechtext wird noch erzeugt. Exportiere, wenn er fertig ist.",
  "storymaker.agent.check_corrections_exhausted": "Eine Szene hat alle Korrekturversuche verbraucht. Bitte den Autor, sie fertigzustellen.",
  "storymaker.agent.check_model_error": "Das Modell hat aufgehört zu antworten. Bitte den Autor, die Geschichte fertigzustellen.",
  "storymaker.agent.check_scene_unfilled": "Diese Szene braucht noch Arbeit. Bitte den Autor, sie fertigzustellen.",
  "storymaker.agent.check_compile_failed": "Die Vorschau konnte diesen Film nicht vorbereiten. Bitte den Autor um eine Korrektur, oder sieh in die Problemliste.",
  "storymaker.agent.check_over_target": "Der Film ist länger, als du wolltest. Bitte den Autor, ihn zu kürzen.",
  "storymaker.agent.check_over_budget": "Eine Szene hat mehr Wörter als geplant. Bitte den Autor, sie zu kürzen.",
  "storymaker.agent.check_unknown": "Etwas sollte noch geprüft werden.",
  "storymaker.agent.make_silent": "Ohne Stimme exportieren",
  "storymaker.agent.make_silent_done": "Der Sprechtext steht jetzt als Text im Bild, und der Film kann exportiert werden.",
  "storymaker.agent.restore_speech": "Sprache wiederherstellen",
  "storymaker.agent.refused_invalid_story": "Die Geschichte hat Fehler, deshalb kann der Autor sie nicht überarbeiten. Behebe sie zuerst.",
  "storymaker.error.agent_job_active": "Der KI-Autor arbeitet an dieser Geschichte. Versuche es noch einmal, wenn er fertig ist.",
  "storymaker.error.story_not_empty": "Diese Geschichte hat schon Inhalt. Beschreibe stattdessen eine Änderung.",
  "storymaker.error.model_too_small": "Das eingestellte Modell ist für den KI-Autor zu klein. Wähle in der Konfiguration ein größeres.",
  "storymaker.error.llm_unavailable": "Gerade ist kein KI-Modell verfügbar.",
  "storymaker.error.daily_limit_reached": "Das Tageslimit für den KI-Autor ist erreicht. Versuche es morgen noch einmal.",
  "storymaker.job_error.model_error": "Das KI-Modell hat keine brauchbare Antwort gegeben."
```

**Edit** `ui/lang/storymaker/el.json`
Old:
```json
  "storymaker.inspector.silent": "Βουβή σκηνή: αυτή η αφήγηση δεν εκφωνείται ούτε εμφανίζεται"
```
New:
```json
  "storymaker.inspector.silent": "Βουβή σκηνή: αυτή η αφήγηση δεν εκφωνείται ούτε εμφανίζεται",
  "storymaker.side_agent": "Συγγραφέας AI",
  "storymaker.banner.agent_locked": "Ο συγγραφέας AI δουλεύει σε αυτή την ιστορία. Η επεξεργασία είναι κλειδωμένη μέχρι να τελειώσει.",
  "storymaker.agent.new_title": "Αφήστε τον συγγραφέα AI να γράψει μια ταινία",
  "storymaker.agent.prompt_start": "Τι πρέπει να εξηγεί η ταινία;",
  "storymaker.agent.prompt_revise": "Τι πρέπει να αλλάξει;",
  "storymaker.agent.prompt_required": "Περιγράψτε τι θέλετε.",
  "storymaker.agent.length": "Διάρκεια σε δευτερόλεπτα",
  "storymaker.agent.language": "Γλώσσα της ταινίας",
  "storymaker.agent.start": "Γράψε την ιστορία",
  "storymaker.agent.revise": "Αναθεώρησε την ιστορία",
  "storymaker.agent.cancel": "Διακοπή συγγραφέα",
  "storymaker.agent.off": "Ο συγγραφέας AI είναι απενεργοποιημένος στις ρυθμίσεις.",
  "storymaker.agent.unavailable": "Δεν υπάρχει διαθέσιμο μοντέλο AI για τον συγγραφέα αυτή τη στιγμή.",
  "storymaker.agent.save_first": "Οι τελευταίες αλλαγές σας δεν αποθηκεύτηκαν, οπότε δεν ξεκίνησε τίποτα.",
  "storymaker.agent.state_cancelled": "Διακόπηκε. Οι έγκυρες σκηνές μένουν στην ιστορία.",
  "storymaker.agent.state_interrupted": "Μια επανεκκίνηση διέκοψε τον συγγραφέα. Οι έγκυρες σκηνές μένουν και μια αναθεώρηση μπορεί να ολοκληρώσει τα υπόλοιπα.",
  "storymaker.agent.phase_outline": "Σχεδιασμός σκηνών",
  "storymaker.agent.phase_cast": "Επιλογή προσώπων",
  "storymaker.agent.phase_fill": "Στήσιμο σκηνών",
  "storymaker.agent.phase_check": "Έλεγχος ταινίας",
  "storymaker.agent.phase_revise": "Εφαρμογή αλλαγής",
  "storymaker.agent.phase_narrating": "Δημιουργία αφήγησης",
  "storymaker.agent.scenes": "Σκηνή {done} από {total}",
  "storymaker.agent.model_retry": "Το μοντέλο ρωτιέται ξανά.",
  "storymaker.agent.mode_compact": "Συμπαγής λειτουργία για μικρό μοντέλο",
  "storymaker.agent.mode_text": "Κλήσεις εργαλείων ως κείμενο",
  "storymaker.agent.outcome_ready_title": "Η ιστορία σας είναι έτοιμη. Δείτε την μία φορά.",
  "storymaker.agent.outcome_ready_text": "Οι σκηνές, τα στοιχεία και ο χρονισμός ελέγχθηκαν. Οι επικαλύψεις και η χωρητικότητα του κειμένου δεν ελέγχθηκαν αυτόματα.",
  "storymaker.agent.outcome_incomplete_title": "Η ιστορία σας είναι εν μέρει έτοιμη.",
  "storymaker.agent.outcome_incomplete_text": "{n} σκηνές χρειάζονται ακόμη δουλειά. Οι έγκυρες σκηνές έχουν αποθηκευτεί.",
  "storymaker.agent.outcome_incomplete_some": "Κάποια μέρη χρειάζονται ακόμη δουλειά. Οι έγκυρες σκηνές έχουν αποθηκευτεί.",
  "storymaker.agent.outcome_failed_title": "Ο πράκτορας δεν μπόρεσε να γράψει αυτή την ιστορία.",
  "storymaker.agent.outcome_failed_text": "Δοκιμάστε ξανά με πιο σύντομο θέμα ή ελέγξτε τις ρυθμίσεις του μοντέλου.",
  "storymaker.agent.finish_instruction": "Ολοκλήρωσε τις σκηνές που χρειάζονται ακόμη δουλειά.",
  "storymaker.agent.check_layout_unverified": "Οι επικαλύψεις και η χωρητικότητα του κειμένου δεν ελέγχθηκαν. Δείτε την ταινία μία φορά.",
  "storymaker.agent.check_compile_unverified": "Δεν υπήρχε διαθέσιμο πρόγραμμα περιήγησης για μέτρηση του χρονισμού. Ελέγξτε τη διάρκεια στην προεπισκόπηση.",
  "storymaker.agent.check_narration_unavailable": "Δεν υπάρχει έτοιμη φωνή για αυτή τη γλώσσα. Ρυθμίστε μία και δημιουργήστε την αφήγηση ή κάντε εξαγωγή χωρίς φωνή.",
  "storymaker.agent.check_narration_failed": "Η αφήγηση δεν δημιουργήθηκε. Δοκιμάστε ξανά στον πίνακα Φωνή.",
  "storymaker.agent.check_narration_budget": "Το όριο χαρακτήρων αφήγησης εξαντλήθηκε. Δοκιμάστε αύριο ή αυξήστε το όριο.",
  "storymaker.agent.check_narration_pending": "Η αφήγηση δημιουργείται ακόμη. Κάντε εξαγωγή όταν τελειώσει.",
  "storymaker.agent.check_corrections_exhausted": "Μια σκηνή εξάντλησε τις προσπάθειες διόρθωσης. Ζητήστε από τον συγγραφέα να την ολοκληρώσει.",
  "storymaker.agent.check_model_error": "Το μοντέλο σταμάτησε να απαντά. Ζητήστε από τον συγγραφέα να ολοκληρώσει την ιστορία.",
  "storymaker.agent.check_scene_unfilled": "Αυτή η σκηνή χρειάζεται ακόμη δουλειά. Ζητήστε από τον συγγραφέα να την ολοκληρώσει.",
  "storymaker.agent.check_compile_failed": "Η προεπισκόπηση δεν μπόρεσε να προετοιμάσει την ταινία. Ζητήστε διόρθωση από τον συγγραφέα ή δείτε τη λίστα προβλημάτων.",
  "storymaker.agent.check_over_target": "Η ταινία είναι μεγαλύτερη από όσο ζητήσατε. Ζητήστε από τον συγγραφέα να τη συντομεύσει.",
  "storymaker.agent.check_over_budget": "Μια σκηνή έχει περισσότερες λέξεις από το σχέδιο. Ζητήστε από τον συγγραφέα να τη συντομεύσει.",
  "storymaker.agent.check_unknown": "Κάτι χρειάζεται ακόμη έλεγχο.",
  "storymaker.agent.make_silent": "Εξαγωγή χωρίς φωνή",
  "storymaker.agent.make_silent_done": "Ο λόγος είναι τώρα κείμενο στην οθόνη και η ταινία μπορεί να εξαχθεί.",
  "storymaker.agent.restore_speech": "Επαναφορά ομιλίας",
  "storymaker.agent.refused_invalid_story": "Η ιστορία έχει σφάλματα, οπότε ο συγγραφέας δεν μπορεί να την αναθεωρήσει. Διορθώστε τα πρώτα.",
  "storymaker.error.agent_job_active": "Ο συγγραφέας AI δουλεύει σε αυτή την ιστορία. Δοκιμάστε ξανά όταν τελειώσει.",
  "storymaker.error.story_not_empty": "Αυτή η ιστορία έχει ήδη περιεχόμενο. Περιγράψτε μια αλλαγή.",
  "storymaker.error.model_too_small": "Το ρυθμισμένο μοντέλο είναι πολύ μικρό για τον συγγραφέα AI. Επιλέξτε μεγαλύτερο στις ρυθμίσεις.",
  "storymaker.error.llm_unavailable": "Δεν υπάρχει διαθέσιμο μοντέλο AI αυτή τη στιγμή.",
  "storymaker.error.daily_limit_reached": "Το ημερήσιο όριο του συγγραφέα AI εξαντλήθηκε. Δοκιμάστε αύριο.",
  "storymaker.job_error.model_error": "Το μοντέλο AI δεν έδωσε χρήσιμη απάντηση."
```

**Edit** `ui/lang/storymaker/en.json`
Old:
```json
  "storymaker.inspector.silent": "Silent scene: this narration is not spoken or shown"
```
New:
```json
  "storymaker.inspector.silent": "Silent scene: this narration is not spoken or shown",
  "storymaker.side_agent": "AI author",
  "storymaker.banner.agent_locked": "The AI author is working on this story. Editing is locked until it finishes.",
  "storymaker.agent.new_title": "Let the AI author write a film",
  "storymaker.agent.prompt_start": "What should the film explain?",
  "storymaker.agent.prompt_revise": "What should change?",
  "storymaker.agent.prompt_required": "Please describe what you want.",
  "storymaker.agent.length": "Length in seconds",
  "storymaker.agent.language": "Language of the film",
  "storymaker.agent.start": "Write the story",
  "storymaker.agent.revise": "Revise the story",
  "storymaker.agent.cancel": "Stop the author",
  "storymaker.agent.off": "The AI author is switched off in the configuration.",
  "storymaker.agent.unavailable": "No AI model is available for the author right now.",
  "storymaker.agent.save_first": "Your latest edits could not be saved, so nothing was started.",
  "storymaker.agent.state_cancelled": "Stopped. Valid scenes stay in the story.",
  "storymaker.agent.state_interrupted": "A restart interrupted the author. Valid scenes stay, and a revision can finish the rest.",
  "storymaker.agent.phase_outline": "Planning the scenes",
  "storymaker.agent.phase_cast": "Choosing the people",
  "storymaker.agent.phase_fill": "Staging the scenes",
  "storymaker.agent.phase_check": "Checking the film",
  "storymaker.agent.phase_revise": "Making the change",
  "storymaker.agent.phase_narrating": "Generating the narration",
  "storymaker.agent.scenes": "Scene {done} of {total}",
  "storymaker.agent.model_retry": "The model is being asked again.",
  "storymaker.agent.mode_compact": "Compact mode for a small model",
  "storymaker.agent.mode_text": "Tool calls as text",
  "storymaker.agent.outcome_ready_title": "Your story is ready. Please watch it once.",
  "storymaker.agent.outcome_ready_text": "Scenes, assets and timing were checked. Overlaps and text fit were not checked automatically.",
  "storymaker.agent.outcome_incomplete_title": "Your story is partly done.",
  "storymaker.agent.outcome_incomplete_text": "{n} scenes still need work. Valid scenes are saved.",
  "storymaker.agent.outcome_incomplete_some": "Some parts still need work. Valid scenes are saved.",
  "storymaker.agent.outcome_failed_title": "The agent could not write this story.",
  "storymaker.agent.outcome_failed_text": "Try again with a shorter topic, or check the model settings.",
  "storymaker.agent.finish_instruction": "Finish the scenes that still need work.",
  "storymaker.agent.check_layout_unverified": "Overlaps and text fit were not checked. Watch the film once.",
  "storymaker.agent.check_compile_unverified": "No browser was available to measure the timing. Check the length in the preview.",
  "storymaker.agent.check_narration_unavailable": "No voice is ready for this language. Set one up and generate the narration, or export without voice.",
  "storymaker.agent.check_narration_failed": "The narration could not be generated. Try again in the Voice panel.",
  "storymaker.agent.check_narration_budget": "The narration character limit is reached. Try again tomorrow, or raise the limit.",
  "storymaker.agent.check_narration_pending": "The narration is still being generated. Export when it is done.",
  "storymaker.agent.check_corrections_exhausted": "A scene ran out of correction attempts. Ask the author to finish it.",
  "storymaker.agent.check_model_error": "The model stopped answering. Ask the author to finish the story.",
  "storymaker.agent.check_scene_unfilled": "This scene still needs work. Ask the author to finish it.",
  "storymaker.agent.check_compile_failed": "The preview could not prepare this film. Ask the author to fix it, or check the issues list.",
  "storymaker.agent.check_over_target": "The film is longer than you asked for. Ask the author to shorten it.",
  "storymaker.agent.check_over_budget": "A scene has more words than planned. Ask the author to shorten it.",
  "storymaker.agent.check_unknown": "Something still needs a look.",
  "storymaker.agent.make_silent": "Export without voice",
  "storymaker.agent.make_silent_done": "The speech is now on-screen text, and the film can be exported.",
  "storymaker.agent.restore_speech": "Restore speech",
  "storymaker.agent.refused_invalid_story": "The story has errors, so the author cannot revise it. Fix them first.",
  "storymaker.error.agent_job_active": "The AI author is working on this story. Try again when it has finished.",
  "storymaker.error.story_not_empty": "This story already has content. Describe a change instead.",
  "storymaker.error.model_too_small": "The configured model is too small for the AI author. Choose a larger one in the configuration.",
  "storymaker.error.llm_unavailable": "No AI model is available right now.",
  "storymaker.error.daily_limit_reached": "The daily limit for the AI author is reached. Try again tomorrow.",
  "storymaker.job_error.model_error": "The AI model did not give a usable answer."
```

**Edit** `ui/lang/storymaker/es.json`
Old:
```json
  "storymaker.inspector.silent": "Escena muda: esta narración no se locuta ni se muestra"
```
New:
```json
  "storymaker.inspector.silent": "Escena muda: esta narración no se locuta ni se muestra",
  "storymaker.side_agent": "Autor IA",
  "storymaker.banner.agent_locked": "El autor IA está trabajando en esta historia. La edición está bloqueada hasta que termine.",
  "storymaker.agent.new_title": "Deja que el autor IA escriba una película",
  "storymaker.agent.prompt_start": "¿Qué debe explicar la película?",
  "storymaker.agent.prompt_revise": "¿Qué debe cambiar?",
  "storymaker.agent.prompt_required": "Describe lo que quieres.",
  "storymaker.agent.length": "Duración en segundos",
  "storymaker.agent.language": "Idioma de la película",
  "storymaker.agent.start": "Escribir la historia",
  "storymaker.agent.revise": "Revisar la historia",
  "storymaker.agent.cancel": "Detener al autor",
  "storymaker.agent.off": "El autor IA está desactivado en la configuración.",
  "storymaker.agent.unavailable": "Ahora mismo no hay ningún modelo de IA disponible para el autor.",
  "storymaker.agent.save_first": "Tus últimos cambios no se pudieron guardar, así que no se inició nada.",
  "storymaker.agent.state_cancelled": "Detenido. Las escenas válidas permanecen en la historia.",
  "storymaker.agent.state_interrupted": "Un reinicio interrumpió al autor. Las escenas válidas permanecen y una revisión puede terminar el resto.",
  "storymaker.agent.phase_outline": "Planificando las escenas",
  "storymaker.agent.phase_cast": "Eligiendo a las personas",
  "storymaker.agent.phase_fill": "Montando las escenas",
  "storymaker.agent.phase_check": "Comprobando la película",
  "storymaker.agent.phase_revise": "Aplicando el cambio",
  "storymaker.agent.phase_narrating": "Generando la narración",
  "storymaker.agent.scenes": "Escena {done} de {total}",
  "storymaker.agent.model_retry": "Se está volviendo a consultar al modelo.",
  "storymaker.agent.mode_compact": "Modo compacto para un modelo pequeño",
  "storymaker.agent.mode_text": "Llamadas a herramientas como texto",
  "storymaker.agent.outcome_ready_title": "Tu historia está lista. Mírala una vez, por favor.",
  "storymaker.agent.outcome_ready_text": "Se comprobaron las escenas, los recursos y los tiempos. Los solapamientos y el ajuste del texto no se comprobaron automáticamente.",
  "storymaker.agent.outcome_incomplete_title": "Tu historia está terminada en parte.",
  "storymaker.agent.outcome_incomplete_text": "{n} escenas aún necesitan trabajo. Las escenas válidas están guardadas.",
  "storymaker.agent.outcome_incomplete_some": "Algunas partes aún necesitan trabajo. Las escenas válidas están guardadas.",
  "storymaker.agent.outcome_failed_title": "El agente no pudo escribir esta historia.",
  "storymaker.agent.outcome_failed_text": "Inténtalo de nuevo con un tema más corto o revisa los ajustes del modelo.",
  "storymaker.agent.finish_instruction": "Termina las escenas que aún necesitan trabajo.",
  "storymaker.agent.check_layout_unverified": "Los solapamientos y el ajuste del texto no se comprobaron. Mira la película una vez.",
  "storymaker.agent.check_compile_unverified": "No había ningún navegador para medir los tiempos. Comprueba la duración en la vista previa.",
  "storymaker.agent.check_narration_unavailable": "No hay ninguna voz lista para este idioma. Configura una y genera la narración, o exporta sin voz.",
  "storymaker.agent.check_narration_failed": "No se pudo generar la narración. Inténtalo de nuevo en el panel Voz.",
  "storymaker.agent.check_narration_budget": "Se alcanzó el límite de caracteres de narración. Inténtalo mañana o aumenta el límite.",
  "storymaker.agent.check_narration_pending": "La narración aún se está generando. Exporta cuando termine.",
  "storymaker.agent.check_corrections_exhausted": "Una escena agotó sus intentos de corrección. Pide al autor que la termine.",
  "storymaker.agent.check_model_error": "El modelo dejó de responder. Pide al autor que termine la historia.",
  "storymaker.agent.check_scene_unfilled": "Esta escena aún necesita trabajo. Pide al autor que la termine.",
  "storymaker.agent.check_compile_failed": "La vista previa no pudo preparar esta película. Pide al autor que la corrija o mira la lista de problemas.",
  "storymaker.agent.check_over_target": "La película es más larga de lo que pediste. Pide al autor que la acorte.",
  "storymaker.agent.check_over_budget": "Una escena tiene más palabras de las previstas. Pide al autor que la acorte.",
  "storymaker.agent.check_unknown": "Algo aún necesita una revisión.",
  "storymaker.agent.make_silent": "Exportar sin voz",
  "storymaker.agent.make_silent_done": "El discurso ahora es texto en pantalla y la película se puede exportar.",
  "storymaker.agent.restore_speech": "Restaurar el discurso",
  "storymaker.agent.refused_invalid_story": "La historia tiene errores, así que el autor no puede revisarla. Corrígelos primero.",
  "storymaker.error.agent_job_active": "El autor IA está trabajando en esta historia. Inténtalo de nuevo cuando termine.",
  "storymaker.error.story_not_empty": "Esta historia ya tiene contenido. Describe un cambio en su lugar.",
  "storymaker.error.model_too_small": "El modelo configurado es demasiado pequeño para el autor IA. Elige uno mayor en la configuración.",
  "storymaker.error.llm_unavailable": "Ahora mismo no hay ningún modelo de IA disponible.",
  "storymaker.error.daily_limit_reached": "Se alcanzó el límite diario del autor IA. Inténtalo mañana.",
  "storymaker.job_error.model_error": "El modelo de IA no dio una respuesta útil."
```

**Edit** `ui/lang/storymaker/fr.json`
Old:
```json
  "storymaker.inspector.silent": "Scène muette : ce commentaire n'est ni lu ni affiché"
```
New:
```json
  "storymaker.inspector.silent": "Scène muette : ce commentaire n'est ni lu ni affiché",
  "storymaker.side_agent": "Auteur IA",
  "storymaker.banner.agent_locked": "L'auteur IA travaille sur cette histoire. La modification est verrouillée jusqu'à ce qu'il ait terminé.",
  "storymaker.agent.new_title": "Laissez l'auteur IA écrire un film",
  "storymaker.agent.prompt_start": "Que doit expliquer le film ?",
  "storymaker.agent.prompt_revise": "Que faut-il changer ?",
  "storymaker.agent.prompt_required": "Décrivez ce que vous voulez.",
  "storymaker.agent.length": "Durée en secondes",
  "storymaker.agent.language": "Langue du film",
  "storymaker.agent.start": "Écrire l'histoire",
  "storymaker.agent.revise": "Réviser l'histoire",
  "storymaker.agent.cancel": "Arrêter l'auteur",
  "storymaker.agent.off": "L'auteur IA est désactivé dans la configuration.",
  "storymaker.agent.unavailable": "Aucun modèle d'IA n'est disponible pour l'auteur pour le moment.",
  "storymaker.agent.save_first": "Vos dernières modifications n'ont pas pu être enregistrées, donc rien n'a démarré.",
  "storymaker.agent.state_cancelled": "Arrêté. Les scènes valides restent dans l'histoire.",
  "storymaker.agent.state_interrupted": "Un redémarrage a interrompu l'auteur. Les scènes valides restent, et une révision peut terminer le reste.",
  "storymaker.agent.phase_outline": "Planification des scènes",
  "storymaker.agent.phase_cast": "Choix des personnages",
  "storymaker.agent.phase_fill": "Mise en scène",
  "storymaker.agent.phase_check": "Vérification du film",
  "storymaker.agent.phase_revise": "Application du changement",
  "storymaker.agent.phase_narrating": "Génération du commentaire",
  "storymaker.agent.scenes": "Scène {done} sur {total}",
  "storymaker.agent.model_retry": "Le modèle est de nouveau sollicité.",
  "storymaker.agent.mode_compact": "Mode compact pour un petit modèle",
  "storymaker.agent.mode_text": "Appels d'outils sous forme de texte",
  "storymaker.agent.outcome_ready_title": "Votre histoire est prête. Regardez-la une fois, s'il vous plaît.",
  "storymaker.agent.outcome_ready_text": "Les scènes, les ressources et le minutage ont été vérifiés. Les chevauchements et la place du texte n'ont pas été vérifiés automatiquement.",
  "storymaker.agent.outcome_incomplete_title": "Votre histoire est en partie terminée.",
  "storymaker.agent.outcome_incomplete_text": "{n} scènes demandent encore du travail. Les scènes valides sont enregistrées.",
  "storymaker.agent.outcome_incomplete_some": "Certaines parties demandent encore du travail. Les scènes valides sont enregistrées.",
  "storymaker.agent.outcome_failed_title": "L'agent n'a pas pu écrire cette histoire.",
  "storymaker.agent.outcome_failed_text": "Réessayez avec un sujet plus court, ou vérifiez les réglages du modèle.",
  "storymaker.agent.finish_instruction": "Termine les scènes qui demandent encore du travail.",
  "storymaker.agent.check_layout_unverified": "Les chevauchements et la place du texte n'ont pas été vérifiés. Regardez le film une fois.",
  "storymaker.agent.check_compile_unverified": "Aucun navigateur n'était disponible pour mesurer le minutage. Vérifiez la durée dans l'aperçu.",
  "storymaker.agent.check_narration_unavailable": "Aucune voix n'est prête pour cette langue. Configurez-en une et générez le commentaire, ou exportez sans voix.",
  "storymaker.agent.check_narration_failed": "Le commentaire n'a pas pu être généré. Réessayez dans le panneau Voix.",
  "storymaker.agent.check_narration_budget": "La limite de caractères du commentaire est atteinte. Réessayez demain, ou augmentez la limite.",
  "storymaker.agent.check_narration_pending": "Le commentaire est encore en cours de génération. Exportez quand il sera prêt.",
  "storymaker.agent.check_corrections_exhausted": "Une scène a épuisé ses tentatives de correction. Demandez à l'auteur de la terminer.",
  "storymaker.agent.check_model_error": "Le modèle a cessé de répondre. Demandez à l'auteur de terminer l'histoire.",
  "storymaker.agent.check_scene_unfilled": "Cette scène demande encore du travail. Demandez à l'auteur de la terminer.",
  "storymaker.agent.check_compile_failed": "L'aperçu n'a pas pu préparer ce film. Demandez une correction à l'auteur, ou consultez la liste des problèmes.",
  "storymaker.agent.check_over_target": "Le film est plus long que demandé. Demandez à l'auteur de le raccourcir.",
  "storymaker.agent.check_over_budget": "Une scène contient plus de mots que prévu. Demandez à l'auteur de la raccourcir.",
  "storymaker.agent.check_unknown": "Un point reste à vérifier.",
  "storymaker.agent.make_silent": "Exporter sans voix",
  "storymaker.agent.make_silent_done": "La parole est maintenant du texte à l'écran, et le film peut être exporté.",
  "storymaker.agent.restore_speech": "Rétablir la parole",
  "storymaker.agent.refused_invalid_story": "L'histoire contient des erreurs, l'auteur ne peut donc pas la réviser. Corrigez-les d'abord.",
  "storymaker.error.agent_job_active": "L'auteur IA travaille sur cette histoire. Réessayez quand il aura terminé.",
  "storymaker.error.story_not_empty": "Cette histoire a déjà du contenu. Décrivez plutôt un changement.",
  "storymaker.error.model_too_small": "Le modèle configuré est trop petit pour l'auteur IA. Choisissez-en un plus grand dans la configuration.",
  "storymaker.error.llm_unavailable": "Aucun modèle d'IA n'est disponible pour le moment.",
  "storymaker.error.daily_limit_reached": "La limite quotidienne de l'auteur IA est atteinte. Réessayez demain.",
  "storymaker.job_error.model_error": "Le modèle d'IA n'a pas donné de réponse exploitable."
```

**Edit** `ui/lang/storymaker/hi.json`
Old:
```json
  "storymaker.inspector.silent": "मौन दृश्य: यह वर्णन न बोला जाता है, न दिखाया जाता है"
```
New:
```json
  "storymaker.inspector.silent": "मौन दृश्य: यह वर्णन न बोला जाता है, न दिखाया जाता है",
  "storymaker.side_agent": "एआई लेखक",
  "storymaker.banner.agent_locked": "एआई लेखक इस कहानी पर काम कर रहा है। उसके पूरा होने तक संपादन बंद है।",
  "storymaker.agent.new_title": "एआई लेखक से फ़िल्म लिखवाएँ",
  "storymaker.agent.prompt_start": "फ़िल्म को क्या समझाना चाहिए?",
  "storymaker.agent.prompt_revise": "क्या बदलना चाहिए?",
  "storymaker.agent.prompt_required": "कृपया बताएँ कि आप क्या चाहते हैं।",
  "storymaker.agent.length": "सेकंड में अवधि",
  "storymaker.agent.language": "फ़िल्म की भाषा",
  "storymaker.agent.start": "कहानी लिखें",
  "storymaker.agent.revise": "कहानी संशोधित करें",
  "storymaker.agent.cancel": "लेखक को रोकें",
  "storymaker.agent.off": "एआई लेखक कॉन्फ़िगरेशन में बंद है।",
  "storymaker.agent.unavailable": "अभी लेखक के लिए कोई एआई मॉडल उपलब्ध नहीं है।",
  "storymaker.agent.save_first": "आपके हाल के बदलाव सहेजे नहीं जा सके, इसलिए कुछ शुरू नहीं हुआ।",
  "storymaker.agent.state_cancelled": "रोक दिया गया। मान्य दृश्य कहानी में बने रहते हैं।",
  "storymaker.agent.state_interrupted": "पुनरारंभ ने लेखक को रोक दिया। मान्य दृश्य बने रहते हैं, और एक संशोधन बाकी काम पूरा कर सकता है।",
  "storymaker.agent.phase_outline": "दृश्यों की योजना",
  "storymaker.agent.phase_cast": "लोगों का चयन",
  "storymaker.agent.phase_fill": "दृश्यों का मंचन",
  "storymaker.agent.phase_check": "फ़िल्म की जाँच",
  "storymaker.agent.phase_revise": "बदलाव लागू करना",
  "storymaker.agent.phase_narrating": "वर्णन बनाना",
  "storymaker.agent.scenes": "दृश्य {done} / {total}",
  "storymaker.agent.model_retry": "मॉडल से फिर पूछा जा रहा है।",
  "storymaker.agent.mode_compact": "छोटे मॉडल के लिए संक्षिप्त मोड",
  "storymaker.agent.mode_text": "टूल कॉल पाठ के रूप में",
  "storymaker.agent.outcome_ready_title": "आपकी कहानी तैयार है। कृपया इसे एक बार देखें।",
  "storymaker.agent.outcome_ready_text": "दृश्य, सामग्री और समय की जाँच हो गई। ओवरलैप और पाठ का फ़िट होना अपने आप नहीं जाँचा गया।",
  "storymaker.agent.outcome_incomplete_title": "आपकी कहानी आंशिक रूप से पूरी है।",
  "storymaker.agent.outcome_incomplete_text": "{n} दृश्यों पर अभी काम बाकी है। मान्य दृश्य सहेजे गए हैं।",
  "storymaker.agent.outcome_incomplete_some": "कुछ हिस्सों पर अभी काम बाकी है। मान्य दृश्य सहेजे गए हैं।",
  "storymaker.agent.outcome_failed_title": "एजेंट यह कहानी नहीं लिख सका।",
  "storymaker.agent.outcome_failed_text": "छोटे विषय के साथ फिर कोशिश करें, या मॉडल की सेटिंग जाँचें।",
  "storymaker.agent.finish_instruction": "जिन दृश्यों पर काम बाकी है, उन्हें पूरा करो।",
  "storymaker.agent.check_layout_unverified": "ओवरलैप और पाठ का फ़िट होना नहीं जाँचा गया। फ़िल्म को एक बार देखें।",
  "storymaker.agent.check_compile_unverified": "समय मापने के लिए कोई ब्राउज़र उपलब्ध नहीं था। पूर्वावलोकन में अवधि जाँचें।",
  "storymaker.agent.check_narration_unavailable": "इस भाषा के लिए कोई आवाज़ तैयार नहीं है। एक आवाज़ सेट करके वर्णन बनाएँ, या बिना आवाज़ निर्यात करें।",
  "storymaker.agent.check_narration_failed": "वर्णन नहीं बन सका। आवाज़ पैनल में फिर कोशिश करें।",
  "storymaker.agent.check_narration_budget": "वर्णन की अक्षर सीमा पूरी हो गई। कल फिर कोशिश करें, या सीमा बढ़ाएँ।",
  "storymaker.agent.check_narration_pending": "वर्णन अभी बन रहा है। पूरा होने पर निर्यात करें।",
  "storymaker.agent.check_corrections_exhausted": "एक दृश्य के सुधार प्रयास समाप्त हो गए। लेखक से इसे पूरा करने को कहें।",
  "storymaker.agent.check_model_error": "मॉडल ने जवाब देना बंद कर दिया। लेखक से कहानी पूरी करने को कहें।",
  "storymaker.agent.check_scene_unfilled": "इस दृश्य पर अभी काम बाकी है। लेखक से इसे पूरा करने को कहें।",
  "storymaker.agent.check_compile_failed": "पूर्वावलोकन इस फ़िल्म को तैयार नहीं कर सका। लेखक से इसे ठीक करने को कहें, या समस्याओं की सूची देखें।",
  "storymaker.agent.check_over_target": "फ़िल्म आपके अनुरोध से लंबी है। लेखक से इसे छोटा करने को कहें।",
  "storymaker.agent.check_over_budget": "एक दृश्य में योजना से अधिक शब्द हैं। लेखक से इसे छोटा करने को कहें।",
  "storymaker.agent.check_unknown": "अभी कुछ देखना बाकी है।",
  "storymaker.agent.make_silent": "बिना आवाज़ निर्यात करें",
  "storymaker.agent.make_silent_done": "भाषण अब स्क्रीन पर पाठ है, और फ़िल्म निर्यात की जा सकती है।",
  "storymaker.agent.restore_speech": "भाषण वापस लाएँ",
  "storymaker.agent.refused_invalid_story": "कहानी में त्रुटियाँ हैं, इसलिए लेखक इसे संशोधित नहीं कर सकता। पहले उन्हें ठीक करें।",
  "storymaker.error.agent_job_active": "एआई लेखक इस कहानी पर काम कर रहा है। उसके पूरा होने पर फिर कोशिश करें।",
  "storymaker.error.story_not_empty": "इस कहानी में पहले से सामग्री है। इसके बजाय कोई बदलाव बताएँ।",
  "storymaker.error.model_too_small": "चुना गया मॉडल एआई लेखक के लिए बहुत छोटा है। कॉन्फ़िगरेशन में बड़ा मॉडल चुनें।",
  "storymaker.error.llm_unavailable": "अभी कोई एआई मॉडल उपलब्ध नहीं है।",
  "storymaker.error.daily_limit_reached": "एआई लेखक की दैनिक सीमा पूरी हो गई। कल फिर कोशिश करें।",
  "storymaker.job_error.model_error": "एआई मॉडल ने उपयोगी उत्तर नहीं दिया।"
```

**Edit** `ui/lang/storymaker/it.json`
Old:
```json
  "storymaker.inspector.silent": "Scena muta: questa narrazione non viene letta né mostrata"
```
New:
```json
  "storymaker.inspector.silent": "Scena muta: questa narrazione non viene letta né mostrata",
  "storymaker.side_agent": "Autore IA",
  "storymaker.banner.agent_locked": "L'autore IA sta lavorando a questa storia. La modifica è bloccata finché non ha finito.",
  "storymaker.agent.new_title": "Lascia che l'autore IA scriva un film",
  "storymaker.agent.prompt_start": "Che cosa deve spiegare il film?",
  "storymaker.agent.prompt_revise": "Che cosa deve cambiare?",
  "storymaker.agent.prompt_required": "Descrivi ciò che vuoi.",
  "storymaker.agent.length": "Durata in secondi",
  "storymaker.agent.language": "Lingua del film",
  "storymaker.agent.start": "Scrivi la storia",
  "storymaker.agent.revise": "Rivedi la storia",
  "storymaker.agent.cancel": "Ferma l'autore",
  "storymaker.agent.off": "L'autore IA è disattivato nella configurazione.",
  "storymaker.agent.unavailable": "Al momento nessun modello IA è disponibile per l'autore.",
  "storymaker.agent.save_first": "Le ultime modifiche non sono state salvate, quindi non è partito nulla.",
  "storymaker.agent.state_cancelled": "Fermato. Le scene valide restano nella storia.",
  "storymaker.agent.state_interrupted": "Un riavvio ha interrotto l'autore. Le scene valide restano e una revisione può completare il resto.",
  "storymaker.agent.phase_outline": "Pianificazione delle scene",
  "storymaker.agent.phase_cast": "Scelta delle persone",
  "storymaker.agent.phase_fill": "Messa in scena",
  "storymaker.agent.phase_check": "Controllo del film",
  "storymaker.agent.phase_revise": "Applicazione della modifica",
  "storymaker.agent.phase_narrating": "Generazione della narrazione",
  "storymaker.agent.scenes": "Scena {done} di {total}",
  "storymaker.agent.model_retry": "Il modello viene interrogato di nuovo.",
  "storymaker.agent.mode_compact": "Modalità compatta per un modello piccolo",
  "storymaker.agent.mode_text": "Chiamate agli strumenti come testo",
  "storymaker.agent.outcome_ready_title": "La tua storia è pronta. Guardala una volta, per favore.",
  "storymaker.agent.outcome_ready_text": "Scene, risorse e tempi sono stati controllati. Sovrapposizioni e spazio del testo non sono stati controllati automaticamente.",
  "storymaker.agent.outcome_incomplete_title": "La tua storia è in parte finita.",
  "storymaker.agent.outcome_incomplete_text": "{n} scene richiedono ancora lavoro. Le scene valide sono salvate.",
  "storymaker.agent.outcome_incomplete_some": "Alcune parti richiedono ancora lavoro. Le scene valide sono salvate.",
  "storymaker.agent.outcome_failed_title": "L'agente non è riuscito a scrivere questa storia.",
  "storymaker.agent.outcome_failed_text": "Riprova con un argomento più breve, oppure controlla le impostazioni del modello.",
  "storymaker.agent.finish_instruction": "Completa le scene che richiedono ancora lavoro.",
  "storymaker.agent.check_layout_unverified": "Sovrapposizioni e spazio del testo non sono stati controllati. Guarda il film una volta.",
  "storymaker.agent.check_compile_unverified": "Nessun browser era disponibile per misurare i tempi. Controlla la durata nell'anteprima.",
  "storymaker.agent.check_narration_unavailable": "Nessuna voce è pronta per questa lingua. Configurane una e genera la narrazione, oppure esporta senza voce.",
  "storymaker.agent.check_narration_failed": "Non è stato possibile generare la narrazione. Riprova nel pannello Voce.",
  "storymaker.agent.check_narration_budget": "Il limite di caratteri della narrazione è stato raggiunto. Riprova domani, oppure aumenta il limite.",
  "storymaker.agent.check_narration_pending": "La narrazione è ancora in generazione. Esporta quando è finita.",
  "storymaker.agent.check_corrections_exhausted": "Una scena ha esaurito i tentativi di correzione. Chiedi all'autore di completarla.",
  "storymaker.agent.check_model_error": "Il modello ha smesso di rispondere. Chiedi all'autore di completare la storia.",
  "storymaker.agent.check_scene_unfilled": "Questa scena richiede ancora lavoro. Chiedi all'autore di completarla.",
  "storymaker.agent.check_compile_failed": "L'anteprima non è riuscita a preparare questo film. Chiedi una correzione all'autore, oppure guarda l'elenco dei problemi.",
  "storymaker.agent.check_over_target": "Il film è più lungo di quanto richiesto. Chiedi all'autore di accorciarlo.",
  "storymaker.agent.check_over_budget": "Una scena ha più parole del previsto. Chiedi all'autore di accorciarla.",
  "storymaker.agent.check_unknown": "C'è ancora qualcosa da controllare.",
  "storymaker.agent.make_silent": "Esporta senza voce",
  "storymaker.agent.make_silent_done": "Il parlato ora è testo sullo schermo e il film può essere esportato.",
  "storymaker.agent.restore_speech": "Ripristina il parlato",
  "storymaker.agent.refused_invalid_story": "La storia contiene errori, quindi l'autore non può rivederla. Correggili prima.",
  "storymaker.error.agent_job_active": "L'autore IA sta lavorando a questa storia. Riprova quando ha finito.",
  "storymaker.error.story_not_empty": "Questa storia ha già un contenuto. Descrivi invece una modifica.",
  "storymaker.error.model_too_small": "Il modello configurato è troppo piccolo per l'autore IA. Scegline uno più grande nella configurazione.",
  "storymaker.error.llm_unavailable": "Al momento nessun modello IA è disponibile.",
  "storymaker.error.daily_limit_reached": "Il limite giornaliero dell'autore IA è stato raggiunto. Riprova domani.",
  "storymaker.job_error.model_error": "Il modello IA non ha dato una risposta utilizzabile."
```

**Edit** `ui/lang/storymaker/ja.json`
Old:
```json
  "storymaker.inspector.silent": "無音のシーン：このナレーションは読み上げも表示もされません"
```
New:
```json
  "storymaker.inspector.silent": "無音のシーン：このナレーションは読み上げも表示もされません",
  "storymaker.side_agent": "AI作家",
  "storymaker.banner.agent_locked": "AI作家がこのストーリーを作業中です。完了するまで編集はロックされます。",
  "storymaker.agent.new_title": "AI作家に映像を書いてもらう",
  "storymaker.agent.prompt_start": "映像で何を説明しますか？",
  "storymaker.agent.prompt_revise": "何を変更しますか？",
  "storymaker.agent.prompt_required": "希望する内容を入力してください。",
  "storymaker.agent.length": "長さ（秒）",
  "storymaker.agent.language": "映像の言語",
  "storymaker.agent.start": "ストーリーを書く",
  "storymaker.agent.revise": "ストーリーを修正する",
  "storymaker.agent.cancel": "作家を停止",
  "storymaker.agent.off": "AI作家は設定でオフになっています。",
  "storymaker.agent.unavailable": "現在、作家が使えるAIモデルはありません。",
  "storymaker.agent.save_first": "最新の編集を保存できなかったため、開始しませんでした。",
  "storymaker.agent.state_cancelled": "停止しました。有効なシーンはストーリーに残ります。",
  "storymaker.agent.state_interrupted": "再起動により作家が中断されました。有効なシーンは残り、修正で残りを仕上げられます。",
  "storymaker.agent.phase_outline": "シーンを計画中",
  "storymaker.agent.phase_cast": "人物を選択中",
  "storymaker.agent.phase_fill": "シーンを演出中",
  "storymaker.agent.phase_check": "映像を確認中",
  "storymaker.agent.phase_revise": "変更を適用中",
  "storymaker.agent.phase_narrating": "ナレーションを生成中",
  "storymaker.agent.scenes": "シーン {done} / {total}",
  "storymaker.agent.model_retry": "モデルにもう一度問い合わせています。",
  "storymaker.agent.mode_compact": "小さなモデル向けのコンパクトモード",
  "storymaker.agent.mode_text": "テキストによるツール呼び出し",
  "storymaker.agent.outcome_ready_title": "ストーリーができました。一度ご覧ください。",
  "storymaker.agent.outcome_ready_text": "シーン、素材、タイミングは確認済みです。重なりと文字の収まりは自動では確認していません。",
  "storymaker.agent.outcome_incomplete_title": "ストーリーは一部完成です。",
  "storymaker.agent.outcome_incomplete_text": "{n} 件のシーンにまだ作業が必要です。有効なシーンは保存されています。",
  "storymaker.agent.outcome_incomplete_some": "一部にまだ作業が必要です。有効なシーンは保存されています。",
  "storymaker.agent.outcome_failed_title": "エージェントはこのストーリーを書けませんでした。",
  "storymaker.agent.outcome_failed_text": "短いトピックでもう一度試すか、モデルの設定を確認してください。",
  "storymaker.agent.finish_instruction": "まだ作業が必要なシーンを仕上げてください。",
  "storymaker.agent.check_layout_unverified": "重なりと文字の収まりは確認していません。映像を一度ご覧ください。",
  "storymaker.agent.check_compile_unverified": "タイミングを測るブラウザーがありませんでした。プレビューで長さを確認してください。",
  "storymaker.agent.check_narration_unavailable": "この言語で使える音声がありません。音声を設定してナレーションを生成するか、音声なしで書き出してください。",
  "storymaker.agent.check_narration_failed": "ナレーションを生成できませんでした。音声パネルでもう一度お試しください。",
  "storymaker.agent.check_narration_budget": "ナレーションの文字数上限に達しました。明日もう一度試すか、上限を上げてください。",
  "storymaker.agent.check_narration_pending": "ナレーションはまだ生成中です。完了してから書き出してください。",
  "storymaker.agent.check_corrections_exhausted": "シーンの修正回数が上限に達しました。作家に仕上げを依頼してください。",
  "storymaker.agent.check_model_error": "モデルが応答しなくなりました。作家にストーリーの仕上げを依頼してください。",
  "storymaker.agent.check_scene_unfilled": "このシーンにはまだ作業が必要です。作家に仕上げを依頼してください。",
  "storymaker.agent.check_compile_failed": "プレビューでこの映像を準備できませんでした。作家に修正を依頼するか、問題の一覧を確認してください。",
  "storymaker.agent.check_over_target": "映像が希望より長くなっています。作家に短縮を依頼してください。",
  "storymaker.agent.check_over_budget": "シーンの語数が予定より多くなっています。作家に短縮を依頼してください。",
  "storymaker.agent.check_unknown": "まだ確認が必要な点があります。",
  "storymaker.agent.make_silent": "音声なしで書き出す",
  "storymaker.agent.make_silent_done": "音声は画面上の文字になり、映像を書き出せるようになりました。",
  "storymaker.agent.restore_speech": "音声を元に戻す",
  "storymaker.agent.refused_invalid_story": "ストーリーにエラーがあるため、作家は修正できません。先に直してください。",
  "storymaker.error.agent_job_active": "AI作家がこのストーリーを作業中です。完了してからもう一度お試しください。",
  "storymaker.error.story_not_empty": "このストーリーにはすでに内容があります。代わりに変更内容を入力してください。",
  "storymaker.error.model_too_small": "設定されたモデルはAI作家には小さすぎます。設定でより大きなモデルを選んでください。",
  "storymaker.error.llm_unavailable": "現在使えるAIモデルはありません。",
  "storymaker.error.daily_limit_reached": "AI作家の1日の上限に達しました。明日もう一度お試しください。",
  "storymaker.job_error.model_error": "AIモデルから使える回答が得られませんでした。"
```

**Edit** `ui/lang/storymaker/nl.json`
Old:
```json
  "storymaker.inspector.silent": "Stille scène: deze voice-over wordt niet uitgesproken of getoond"
```
New:
```json
  "storymaker.inspector.silent": "Stille scène: deze voice-over wordt niet uitgesproken of getoond",
  "storymaker.side_agent": "AI-auteur",
  "storymaker.banner.agent_locked": "De AI-auteur werkt aan dit verhaal. Bewerken is vergrendeld tot hij klaar is.",
  "storymaker.agent.new_title": "Laat de AI-auteur een film schrijven",
  "storymaker.agent.prompt_start": "Wat moet de film uitleggen?",
  "storymaker.agent.prompt_revise": "Wat moet er veranderen?",
  "storymaker.agent.prompt_required": "Beschrijf wat je wilt.",
  "storymaker.agent.length": "Lengte in seconden",
  "storymaker.agent.language": "Taal van de film",
  "storymaker.agent.start": "Verhaal schrijven",
  "storymaker.agent.revise": "Verhaal herzien",
  "storymaker.agent.cancel": "Auteur stoppen",
  "storymaker.agent.off": "De AI-auteur staat uit in de configuratie.",
  "storymaker.agent.unavailable": "Er is nu geen AI-model beschikbaar voor de auteur.",
  "storymaker.agent.save_first": "Je laatste wijzigingen konden niet worden opgeslagen, dus er is niets gestart.",
  "storymaker.agent.state_cancelled": "Gestopt. Geldige scènes blijven in het verhaal.",
  "storymaker.agent.state_interrupted": "Een herstart heeft de auteur onderbroken. Geldige scènes blijven, en een herziening kan de rest afmaken.",
  "storymaker.agent.phase_outline": "Scènes plannen",
  "storymaker.agent.phase_cast": "Personen kiezen",
  "storymaker.agent.phase_fill": "Scènes uitwerken",
  "storymaker.agent.phase_check": "Film controleren",
  "storymaker.agent.phase_revise": "Wijziging uitvoeren",
  "storymaker.agent.phase_narrating": "Voice-over genereren",
  "storymaker.agent.scenes": "Scène {done} van {total}",
  "storymaker.agent.model_retry": "Het model wordt opnieuw gevraagd.",
  "storymaker.agent.mode_compact": "Compacte modus voor een klein model",
  "storymaker.agent.mode_text": "Toolaanroepen als tekst",
  "storymaker.agent.outcome_ready_title": "Je verhaal is klaar. Bekijk het graag één keer.",
  "storymaker.agent.outcome_ready_text": "Scènes, onderdelen en timing zijn gecontroleerd. Overlappingen en de pasvorm van tekst zijn niet automatisch gecontroleerd.",
  "storymaker.agent.outcome_incomplete_title": "Je verhaal is gedeeltelijk klaar.",
  "storymaker.agent.outcome_incomplete_text": "{n} scènes hebben nog werk nodig. Geldige scènes zijn opgeslagen.",
  "storymaker.agent.outcome_incomplete_some": "Sommige delen hebben nog werk nodig. Geldige scènes zijn opgeslagen.",
  "storymaker.agent.outcome_failed_title": "De agent kon dit verhaal niet schrijven.",
  "storymaker.agent.outcome_failed_text": "Probeer het opnieuw met een korter onderwerp, of controleer de modelinstellingen.",
  "storymaker.agent.finish_instruction": "Maak de scènes af die nog werk nodig hebben.",
  "storymaker.agent.check_layout_unverified": "Overlappingen en de pasvorm van tekst zijn niet gecontroleerd. Bekijk de film één keer.",
  "storymaker.agent.check_compile_unverified": "Er was geen browser om de timing te meten. Controleer de lengte in de preview.",
  "storymaker.agent.check_narration_unavailable": "Er is geen stem klaar voor deze taal. Stel er een in en genereer de voice-over, of exporteer zonder stem.",
  "storymaker.agent.check_narration_failed": "De voice-over kon niet worden gegenereerd. Probeer het opnieuw in het paneel Stem.",
  "storymaker.agent.check_narration_budget": "De tekenlimiet voor de voice-over is bereikt. Probeer het morgen opnieuw, of verhoog de limiet.",
  "storymaker.agent.check_narration_pending": "De voice-over wordt nog gegenereerd. Exporteer zodra hij klaar is.",
  "storymaker.agent.check_corrections_exhausted": "Een scène heeft geen correctiepogingen meer. Vraag de auteur om hem af te maken.",
  "storymaker.agent.check_model_error": "Het model antwoordt niet meer. Vraag de auteur om het verhaal af te maken.",
  "storymaker.agent.check_scene_unfilled": "Deze scène heeft nog werk nodig. Vraag de auteur om hem af te maken.",
  "storymaker.agent.check_compile_failed": "De preview kon deze film niet voorbereiden. Vraag de auteur om een correctie, of bekijk de lijst met problemen.",
  "storymaker.agent.check_over_target": "De film is langer dan je vroeg. Vraag de auteur om hem in te korten.",
  "storymaker.agent.check_over_budget": "Een scène heeft meer woorden dan gepland. Vraag de auteur om hem in te korten.",
  "storymaker.agent.check_unknown": "Er moet nog iets worden bekeken.",
  "storymaker.agent.make_silent": "Exporteren zonder stem",
  "storymaker.agent.make_silent_done": "De spraak is nu tekst in beeld, en de film kan worden geëxporteerd.",
  "storymaker.agent.restore_speech": "Spraak herstellen",
  "storymaker.agent.refused_invalid_story": "Het verhaal bevat fouten, dus de auteur kan het niet herzien. Los ze eerst op.",
  "storymaker.error.agent_job_active": "De AI-auteur werkt aan dit verhaal. Probeer het opnieuw als hij klaar is.",
  "storymaker.error.story_not_empty": "Dit verhaal heeft al inhoud. Beschrijf in plaats daarvan een wijziging.",
  "storymaker.error.model_too_small": "Het ingestelde model is te klein voor de AI-auteur. Kies een groter model in de configuratie.",
  "storymaker.error.llm_unavailable": "Er is nu geen AI-model beschikbaar.",
  "storymaker.error.daily_limit_reached": "De daglimiet van de AI-auteur is bereikt. Probeer het morgen opnieuw.",
  "storymaker.job_error.model_error": "Het AI-model gaf geen bruikbaar antwoord."
```

**Edit** `ui/lang/storymaker/no.json`
Old:
```json
  "storymaker.inspector.silent": "Stille scene: denne fortellerteksten blir verken lest opp eller vist"
```
New:
```json
  "storymaker.inspector.silent": "Stille scene: denne fortellerteksten blir verken lest opp eller vist",
  "storymaker.side_agent": "KI-forfatter",
  "storymaker.banner.agent_locked": "KI-forfatteren arbeider med denne historien. Redigering er låst til den er ferdig.",
  "storymaker.agent.new_title": "La KI-forfatteren skrive en film",
  "storymaker.agent.prompt_start": "Hva skal filmen forklare?",
  "storymaker.agent.prompt_revise": "Hva skal endres?",
  "storymaker.agent.prompt_required": "Beskriv hva du ønsker.",
  "storymaker.agent.length": "Lengde i sekunder",
  "storymaker.agent.language": "Filmens språk",
  "storymaker.agent.start": "Skriv historien",
  "storymaker.agent.revise": "Revider historien",
  "storymaker.agent.cancel": "Stopp forfatteren",
  "storymaker.agent.off": "KI-forfatteren er slått av i konfigurasjonen.",
  "storymaker.agent.unavailable": "Ingen KI-modell er tilgjengelig for forfatteren akkurat nå.",
  "storymaker.agent.save_first": "De siste endringene dine kunne ikke lagres, så ingenting ble startet.",
  "storymaker.agent.state_cancelled": "Stoppet. Gyldige scener blir i historien.",
  "storymaker.agent.state_interrupted": "En omstart avbrøt forfatteren. Gyldige scener blir, og en revisjon kan fullføre resten.",
  "storymaker.agent.phase_outline": "Planlegger scenene",
  "storymaker.agent.phase_cast": "Velger personene",
  "storymaker.agent.phase_fill": "Iscenesetter scenene",
  "storymaker.agent.phase_check": "Kontrollerer filmen",
  "storymaker.agent.phase_revise": "Utfører endringen",
  "storymaker.agent.phase_narrating": "Genererer fortellingen",
  "storymaker.agent.scenes": "Scene {done} av {total}",
  "storymaker.agent.model_retry": "Modellen blir spurt på nytt.",
  "storymaker.agent.mode_compact": "Kompakt modus for en liten modell",
  "storymaker.agent.mode_text": "Verktøykall som tekst",
  "storymaker.agent.outcome_ready_title": "Historien din er klar. Se gjennom den én gang.",
  "storymaker.agent.outcome_ready_text": "Scener, elementer og timing er kontrollert. Overlapp og tekstens plass ble ikke kontrollert automatisk.",
  "storymaker.agent.outcome_incomplete_title": "Historien din er delvis ferdig.",
  "storymaker.agent.outcome_incomplete_text": "{n} scener trenger fortsatt arbeid. Gyldige scener er lagret.",
  "storymaker.agent.outcome_incomplete_some": "Noen deler trenger fortsatt arbeid. Gyldige scener er lagret.",
  "storymaker.agent.outcome_failed_title": "Agenten kunne ikke skrive denne historien.",
  "storymaker.agent.outcome_failed_text": "Prøv igjen med et kortere emne, eller kontroller modellinnstillingene.",
  "storymaker.agent.finish_instruction": "Fullfør scenene som fortsatt trenger arbeid.",
  "storymaker.agent.check_layout_unverified": "Overlapp og tekstens plass ble ikke kontrollert. Se gjennom filmen én gang.",
  "storymaker.agent.check_compile_unverified": "Ingen nettleser var tilgjengelig for å måle timingen. Kontroller lengden i forhåndsvisningen.",
  "storymaker.agent.check_narration_unavailable": "Ingen stemme er klar for dette språket. Sett opp en og generer fortellingen, eller eksporter uten stemme.",
  "storymaker.agent.check_narration_failed": "Fortellingen kunne ikke genereres. Prøv igjen i panelet Stemme.",
  "storymaker.agent.check_narration_budget": "Tegngrensen for fortelling er nådd. Prøv igjen i morgen, eller øk grensen.",
  "storymaker.agent.check_narration_pending": "Fortellingen genereres fortsatt. Eksporter når den er ferdig.",
  "storymaker.agent.check_corrections_exhausted": "En scene har brukt opp korrigeringsforsøkene. Be forfatteren om å fullføre den.",
  "storymaker.agent.check_model_error": "Modellen sluttet å svare. Be forfatteren om å fullføre historien.",
  "storymaker.agent.check_scene_unfilled": "Denne scenen trenger fortsatt arbeid. Be forfatteren om å fullføre den.",
  "storymaker.agent.check_compile_failed": "Forhåndsvisningen kunne ikke forberede filmen. Be forfatteren om å rette den, eller se listen over problemer.",
  "storymaker.agent.check_over_target": "Filmen er lengre enn du ba om. Be forfatteren om å korte den ned.",
  "storymaker.agent.check_over_budget": "En scene har flere ord enn planlagt. Be forfatteren om å korte den ned.",
  "storymaker.agent.check_unknown": "Noe må fortsatt ses på.",
  "storymaker.agent.make_silent": "Eksporter uten stemme",
  "storymaker.agent.make_silent_done": "Talen er nå tekst på skjermen, og filmen kan eksporteres.",
  "storymaker.agent.restore_speech": "Gjenopprett tale",
  "storymaker.agent.refused_invalid_story": "Historien har feil, så forfatteren kan ikke revidere den. Rett dem først.",
  "storymaker.error.agent_job_active": "KI-forfatteren arbeider med denne historien. Prøv igjen når den er ferdig.",
  "storymaker.error.story_not_empty": "Denne historien har allerede innhold. Beskriv en endring i stedet.",
  "storymaker.error.model_too_small": "Den valgte modellen er for liten for KI-forfatteren. Velg en større i konfigurasjonen.",
  "storymaker.error.llm_unavailable": "Ingen KI-modell er tilgjengelig akkurat nå.",
  "storymaker.error.daily_limit_reached": "Den daglige grensen for KI-forfatteren er nådd. Prøv igjen i morgen.",
  "storymaker.job_error.model_error": "KI-modellen ga ikke et brukbart svar."
```

**Edit** `ui/lang/storymaker/pl.json`
Old:
```json
  "storymaker.inspector.silent": "Scena bez głosu: ten tekst lektora nie jest czytany ani pokazywany"
```
New:
```json
  "storymaker.inspector.silent": "Scena bez głosu: ten tekst lektora nie jest czytany ani pokazywany",
  "storymaker.side_agent": "Autor AI",
  "storymaker.banner.agent_locked": "Autor AI pracuje nad tą historią. Edycja jest zablokowana do czasu zakończenia.",
  "storymaker.agent.new_title": "Pozwól autorowi AI napisać film",
  "storymaker.agent.prompt_start": "Co film ma wyjaśnić?",
  "storymaker.agent.prompt_revise": "Co ma się zmienić?",
  "storymaker.agent.prompt_required": "Opisz, czego chcesz.",
  "storymaker.agent.length": "Długość w sekundach",
  "storymaker.agent.language": "Język filmu",
  "storymaker.agent.start": "Napisz historię",
  "storymaker.agent.revise": "Popraw historię",
  "storymaker.agent.cancel": "Zatrzymaj autora",
  "storymaker.agent.off": "Autor AI jest wyłączony w konfiguracji.",
  "storymaker.agent.unavailable": "Teraz dla autora nie jest dostępny żaden model AI.",
  "storymaker.agent.save_first": "Nie udało się zapisać ostatnich zmian, więc nic nie zostało uruchomione.",
  "storymaker.agent.state_cancelled": "Zatrzymano. Poprawne sceny pozostają w historii.",
  "storymaker.agent.state_interrupted": "Restart przerwał pracę autora. Poprawne sceny pozostają, a poprawka może dokończyć resztę.",
  "storymaker.agent.phase_outline": "Planowanie scen",
  "storymaker.agent.phase_cast": "Wybór postaci",
  "storymaker.agent.phase_fill": "Inscenizacja scen",
  "storymaker.agent.phase_check": "Sprawdzanie filmu",
  "storymaker.agent.phase_revise": "Wprowadzanie zmiany",
  "storymaker.agent.phase_narrating": "Generowanie narracji",
  "storymaker.agent.scenes": "Scena {done} z {total}",
  "storymaker.agent.model_retry": "Model jest pytany ponownie.",
  "storymaker.agent.mode_compact": "Tryb kompaktowy dla małego modelu",
  "storymaker.agent.mode_text": "Wywołania narzędzi jako tekst",
  "storymaker.agent.outcome_ready_title": "Twoja historia jest gotowa. Obejrzyj ją raz.",
  "storymaker.agent.outcome_ready_text": "Sceny, zasoby i czas zostały sprawdzone. Nakładanie się elementów i mieszczenie tekstu nie były sprawdzane automatycznie.",
  "storymaker.agent.outcome_incomplete_title": "Twoja historia jest gotowa częściowo.",
  "storymaker.agent.outcome_incomplete_text": "Liczba scen wymagających pracy: {n}. Poprawne sceny są zapisane.",
  "storymaker.agent.outcome_incomplete_some": "Niektóre części wciąż wymagają pracy. Poprawne sceny są zapisane.",
  "storymaker.agent.outcome_failed_title": "Agent nie mógł napisać tej historii.",
  "storymaker.agent.outcome_failed_text": "Spróbuj ponownie z krótszym tematem albo sprawdź ustawienia modelu.",
  "storymaker.agent.finish_instruction": "Dokończ sceny, które wciąż wymagają pracy.",
  "storymaker.agent.check_layout_unverified": "Nakładanie się elementów i mieszczenie tekstu nie zostały sprawdzone. Obejrzyj film raz.",
  "storymaker.agent.check_compile_unverified": "Nie było przeglądarki do pomiaru czasu. Sprawdź długość w podglądzie.",
  "storymaker.agent.check_narration_unavailable": "Dla tego języka nie ma gotowego głosu. Skonfiguruj głos i wygeneruj narrację albo eksportuj bez głosu.",
  "storymaker.agent.check_narration_failed": "Nie udało się wygenerować narracji. Spróbuj ponownie w panelu Głos.",
  "storymaker.agent.check_narration_budget": "Osiągnięto limit znaków narracji. Spróbuj jutro albo zwiększ limit.",
  "storymaker.agent.check_narration_pending": "Narracja jest jeszcze generowana. Eksportuj po jej zakończeniu.",
  "storymaker.agent.check_corrections_exhausted": "Scena wyczerpała próby poprawek. Poproś autora o jej dokończenie.",
  "storymaker.agent.check_model_error": "Model przestał odpowiadać. Poproś autora o dokończenie historii.",
  "storymaker.agent.check_scene_unfilled": "Ta scena wciąż wymaga pracy. Poproś autora o jej dokończenie.",
  "storymaker.agent.check_compile_failed": "Podgląd nie mógł przygotować tego filmu. Poproś autora o poprawkę albo sprawdź listę problemów.",
  "storymaker.agent.check_over_target": "Film jest dłuższy, niż prosiłeś. Poproś autora o skrócenie.",
  "storymaker.agent.check_over_budget": "Scena ma więcej słów, niż planowano. Poproś autora o skrócenie.",
  "storymaker.agent.check_unknown": "Coś wciąż wymaga sprawdzenia.",
  "storymaker.agent.make_silent": "Eksportuj bez głosu",
  "storymaker.agent.make_silent_done": "Mowa jest teraz tekstem na ekranie, a film można wyeksportować.",
  "storymaker.agent.restore_speech": "Przywróć mowę",
  "storymaker.agent.refused_invalid_story": "Historia zawiera błędy, więc autor nie może jej poprawić. Najpierw je napraw.",
  "storymaker.error.agent_job_active": "Autor AI pracuje nad tą historią. Spróbuj ponownie po zakończeniu.",
  "storymaker.error.story_not_empty": "Ta historia ma już treść. Opisz zamiast tego zmianę.",
  "storymaker.error.model_too_small": "Ustawiony model jest za mały dla autora AI. Wybierz większy w konfiguracji.",
  "storymaker.error.llm_unavailable": "Teraz nie jest dostępny żaden model AI.",
  "storymaker.error.daily_limit_reached": "Osiągnięto dzienny limit autora AI. Spróbuj jutro.",
  "storymaker.job_error.model_error": "Model AI nie dał użytecznej odpowiedzi."
```

**Edit** `ui/lang/storymaker/pt.json`
Old:
```json
  "storymaker.inspector.silent": "Cena muda: esta narração não é falada nem mostrada"
```
New:
```json
  "storymaker.inspector.silent": "Cena muda: esta narração não é falada nem mostrada",
  "storymaker.side_agent": "Autor IA",
  "storymaker.banner.agent_locked": "O autor IA está a trabalhar nesta história. A edição fica bloqueada até terminar.",
  "storymaker.agent.new_title": "Deixe o autor IA escrever um filme",
  "storymaker.agent.prompt_start": "O que o filme deve explicar?",
  "storymaker.agent.prompt_revise": "O que deve mudar?",
  "storymaker.agent.prompt_required": "Descreva o que pretende.",
  "storymaker.agent.length": "Duração em segundos",
  "storymaker.agent.language": "Idioma do filme",
  "storymaker.agent.start": "Escrever a história",
  "storymaker.agent.revise": "Rever a história",
  "storymaker.agent.cancel": "Parar o autor",
  "storymaker.agent.off": "O autor IA está desligado na configuração.",
  "storymaker.agent.unavailable": "Neste momento não há nenhum modelo de IA disponível para o autor.",
  "storymaker.agent.save_first": "Não foi possível guardar as últimas alterações, por isso nada foi iniciado.",
  "storymaker.agent.state_cancelled": "Parado. As cenas válidas ficam na história.",
  "storymaker.agent.state_interrupted": "Um reinício interrompeu o autor. As cenas válidas ficam, e uma revisão pode terminar o resto.",
  "storymaker.agent.phase_outline": "A planear as cenas",
  "storymaker.agent.phase_cast": "A escolher as pessoas",
  "storymaker.agent.phase_fill": "A encenar as cenas",
  "storymaker.agent.phase_check": "A verificar o filme",
  "storymaker.agent.phase_revise": "A aplicar a alteração",
  "storymaker.agent.phase_narrating": "A gerar a narração",
  "storymaker.agent.scenes": "Cena {done} de {total}",
  "storymaker.agent.model_retry": "O modelo está a ser consultado de novo.",
  "storymaker.agent.mode_compact": "Modo compacto para um modelo pequeno",
  "storymaker.agent.mode_text": "Chamadas de ferramentas como texto",
  "storymaker.agent.outcome_ready_title": "A sua história está pronta. Veja-a uma vez, por favor.",
  "storymaker.agent.outcome_ready_text": "As cenas, os recursos e os tempos foram verificados. As sobreposições e o ajuste do texto não foram verificados automaticamente.",
  "storymaker.agent.outcome_incomplete_title": "A sua história está parcialmente concluída.",
  "storymaker.agent.outcome_incomplete_text": "{n} cenas ainda precisam de trabalho. As cenas válidas estão guardadas.",
  "storymaker.agent.outcome_incomplete_some": "Algumas partes ainda precisam de trabalho. As cenas válidas estão guardadas.",
  "storymaker.agent.outcome_failed_title": "O agente não conseguiu escrever esta história.",
  "storymaker.agent.outcome_failed_text": "Tente de novo com um tema mais curto, ou verifique as definições do modelo.",
  "storymaker.agent.finish_instruction": "Termina as cenas que ainda precisam de trabalho.",
  "storymaker.agent.check_layout_unverified": "As sobreposições e o ajuste do texto não foram verificados. Veja o filme uma vez.",
  "storymaker.agent.check_compile_unverified": "Não havia nenhum navegador para medir os tempos. Verifique a duração na pré-visualização.",
  "storymaker.agent.check_narration_unavailable": "Não há nenhuma voz pronta para este idioma. Configure uma e gere a narração, ou exporte sem voz.",
  "storymaker.agent.check_narration_failed": "Não foi possível gerar a narração. Tente de novo no painel Voz.",
  "storymaker.agent.check_narration_budget": "O limite de caracteres da narração foi atingido. Tente amanhã, ou aumente o limite.",
  "storymaker.agent.check_narration_pending": "A narração ainda está a ser gerada. Exporte quando terminar.",
  "storymaker.agent.check_corrections_exhausted": "Uma cena esgotou as tentativas de correção. Peça ao autor que a termine.",
  "storymaker.agent.check_model_error": "O modelo deixou de responder. Peça ao autor que termine a história.",
  "storymaker.agent.check_scene_unfilled": "Esta cena ainda precisa de trabalho. Peça ao autor que a termine.",
  "storymaker.agent.check_compile_failed": "A pré-visualização não conseguiu preparar este filme. Peça uma correção ao autor, ou veja a lista de problemas.",
  "storymaker.agent.check_over_target": "O filme é mais longo do que pediu. Peça ao autor que o encurte.",
  "storymaker.agent.check_over_budget": "Uma cena tem mais palavras do que o previsto. Peça ao autor que a encurte.",
  "storymaker.agent.check_unknown": "Ainda há algo a verificar.",
  "storymaker.agent.make_silent": "Exportar sem voz",
  "storymaker.agent.make_silent_done": "A fala é agora texto no ecrã, e o filme pode ser exportado.",
  "storymaker.agent.restore_speech": "Repor a fala",
  "storymaker.agent.refused_invalid_story": "A história tem erros, por isso o autor não a pode rever. Corrija-os primeiro.",
  "storymaker.error.agent_job_active": "O autor IA está a trabalhar nesta história. Tente de novo quando terminar.",
  "storymaker.error.story_not_empty": "Esta história já tem conteúdo. Descreva antes uma alteração.",
  "storymaker.error.model_too_small": "O modelo configurado é demasiado pequeno para o autor IA. Escolha um maior na configuração.",
  "storymaker.error.llm_unavailable": "Neste momento não há nenhum modelo de IA disponível.",
  "storymaker.error.daily_limit_reached": "O limite diário do autor IA foi atingido. Tente amanhã.",
  "storymaker.job_error.model_error": "O modelo de IA não deu uma resposta utilizável."
```

**Edit** `ui/lang/storymaker/sv.json`
Old:
```json
  "storymaker.inspector.silent": "Tyst scen: den här berättarrösten varken läses upp eller visas"
```
New:
```json
  "storymaker.inspector.silent": "Tyst scen: den här berättarrösten varken läses upp eller visas",
  "storymaker.side_agent": "AI-författare",
  "storymaker.banner.agent_locked": "AI-författaren arbetar med den här berättelsen. Redigering är låst tills den är klar.",
  "storymaker.agent.new_title": "Låt AI-författaren skriva en film",
  "storymaker.agent.prompt_start": "Vad ska filmen förklara?",
  "storymaker.agent.prompt_revise": "Vad ska ändras?",
  "storymaker.agent.prompt_required": "Beskriv vad du vill.",
  "storymaker.agent.length": "Längd i sekunder",
  "storymaker.agent.language": "Filmens språk",
  "storymaker.agent.start": "Skriv berättelsen",
  "storymaker.agent.revise": "Bearbeta berättelsen",
  "storymaker.agent.cancel": "Stoppa författaren",
  "storymaker.agent.off": "AI-författaren är avstängd i konfigurationen.",
  "storymaker.agent.unavailable": "Ingen AI-modell är tillgänglig för författaren just nu.",
  "storymaker.agent.save_first": "Dina senaste ändringar kunde inte sparas, så inget startades.",
  "storymaker.agent.state_cancelled": "Stoppad. Giltiga scener stannar i berättelsen.",
  "storymaker.agent.state_interrupted": "En omstart avbröt författaren. Giltiga scener stannar, och en bearbetning kan göra klart resten.",
  "storymaker.agent.phase_outline": "Planerar scenerna",
  "storymaker.agent.phase_cast": "Väljer personerna",
  "storymaker.agent.phase_fill": "Iscensätter scenerna",
  "storymaker.agent.phase_check": "Kontrollerar filmen",
  "storymaker.agent.phase_revise": "Gör ändringen",
  "storymaker.agent.phase_narrating": "Skapar berättarrösten",
  "storymaker.agent.scenes": "Scen {done} av {total}",
  "storymaker.agent.model_retry": "Modellen tillfrågas igen.",
  "storymaker.agent.mode_compact": "Kompakt läge för en liten modell",
  "storymaker.agent.mode_text": "Verktygsanrop som text",
  "storymaker.agent.outcome_ready_title": "Din berättelse är klar. Titta på den en gång.",
  "storymaker.agent.outcome_ready_text": "Scener, resurser och timing har kontrollerats. Överlapp och textens plats kontrollerades inte automatiskt.",
  "storymaker.agent.outcome_incomplete_title": "Din berättelse är delvis klar.",
  "storymaker.agent.outcome_incomplete_text": "{n} scener behöver fortfarande arbete. Giltiga scener är sparade.",
  "storymaker.agent.outcome_incomplete_some": "Vissa delar behöver fortfarande arbete. Giltiga scener är sparade.",
  "storymaker.agent.outcome_failed_title": "Agenten kunde inte skriva den här berättelsen.",
  "storymaker.agent.outcome_failed_text": "Försök igen med ett kortare ämne, eller kontrollera modellinställningarna.",
  "storymaker.agent.finish_instruction": "Gör klart de scener som fortfarande behöver arbete.",
  "storymaker.agent.check_layout_unverified": "Överlapp och textens plats kontrollerades inte. Titta på filmen en gång.",
  "storymaker.agent.check_compile_unverified": "Ingen webbläsare fanns tillgänglig för att mäta timingen. Kontrollera längden i förhandsvisningen.",
  "storymaker.agent.check_narration_unavailable": "Ingen röst är redo för det här språket. Ställ in en och skapa berättarrösten, eller exportera utan röst.",
  "storymaker.agent.check_narration_failed": "Berättarrösten kunde inte skapas. Försök igen i panelen Röst.",
  "storymaker.agent.check_narration_budget": "Teckengränsen för berättarrösten är nådd. Försök igen i morgon, eller höj gränsen.",
  "storymaker.agent.check_narration_pending": "Berättarrösten skapas fortfarande. Exportera när den är klar.",
  "storymaker.agent.check_corrections_exhausted": "En scen har inga korrigeringsförsök kvar. Be författaren göra klart den.",
  "storymaker.agent.check_model_error": "Modellen slutade svara. Be författaren göra klart berättelsen.",
  "storymaker.agent.check_scene_unfilled": "Den här scenen behöver fortfarande arbete. Be författaren göra klart den.",
  "storymaker.agent.check_compile_failed": "Förhandsvisningen kunde inte förbereda filmen. Be författaren rätta den, eller se listan över problem.",
  "storymaker.agent.check_over_target": "Filmen är längre än du bad om. Be författaren korta den.",
  "storymaker.agent.check_over_budget": "En scen har fler ord än planerat. Be författaren korta den.",
  "storymaker.agent.check_unknown": "Något behöver fortfarande ses över.",
  "storymaker.agent.make_silent": "Exportera utan röst",
  "storymaker.agent.make_silent_done": "Talet är nu text på skärmen, och filmen kan exporteras.",
  "storymaker.agent.restore_speech": "Återställ tal",
  "storymaker.agent.refused_invalid_story": "Berättelsen har fel, så författaren kan inte bearbeta den. Rätta dem först.",
  "storymaker.error.agent_job_active": "AI-författaren arbetar med den här berättelsen. Försök igen när den är klar.",
  "storymaker.error.story_not_empty": "Den här berättelsen har redan innehåll. Beskriv en ändring i stället.",
  "storymaker.error.model_too_small": "Den inställda modellen är för liten för AI-författaren. Välj en större i konfigurationen.",
  "storymaker.error.llm_unavailable": "Ingen AI-modell är tillgänglig just nu.",
  "storymaker.error.daily_limit_reached": "Den dagliga gränsen för AI-författaren är nådd. Försök igen i morgon.",
  "storymaker.job_error.model_error": "AI-modellen gav inget användbart svar."
```

**Edit** `ui/lang/storymaker/zh.json`
Old:
```json
  "storymaker.inspector.silent": "静音场景：此旁白不会朗读，也不会显示"
```
New:
```json
  "storymaker.inspector.silent": "静音场景：此旁白不会朗读，也不会显示",
  "storymaker.side_agent": "AI 作者",
  "storymaker.banner.agent_locked": "AI 作者正在处理这个故事。完成前编辑已锁定。",
  "storymaker.agent.new_title": "让 AI 作者写一部影片",
  "storymaker.agent.prompt_start": "影片应该讲解什么？",
  "storymaker.agent.prompt_revise": "应该修改什么？",
  "storymaker.agent.prompt_required": "请描述你想要的内容。",
  "storymaker.agent.length": "时长（秒）",
  "storymaker.agent.language": "影片语言",
  "storymaker.agent.start": "撰写故事",
  "storymaker.agent.revise": "修改故事",
  "storymaker.agent.cancel": "停止作者",
  "storymaker.agent.off": "AI 作者已在配置中关闭。",
  "storymaker.agent.unavailable": "目前没有可供作者使用的 AI 模型。",
  "storymaker.agent.save_first": "最近的编辑未能保存，因此没有开始。",
  "storymaker.agent.state_cancelled": "已停止。有效的场景保留在故事中。",
  "storymaker.agent.state_interrupted": "重启中断了作者。有效的场景会保留，修改可以完成其余部分。",
  "storymaker.agent.phase_outline": "正在规划场景",
  "storymaker.agent.phase_cast": "正在选择人物",
  "storymaker.agent.phase_fill": "正在编排场景",
  "storymaker.agent.phase_check": "正在检查影片",
  "storymaker.agent.phase_revise": "正在应用修改",
  "storymaker.agent.phase_narrating": "正在生成旁白",
  "storymaker.agent.scenes": "场景 {done} / {total}",
  "storymaker.agent.model_retry": "正在再次询问模型。",
  "storymaker.agent.mode_compact": "小模型的紧凑模式",
  "storymaker.agent.mode_text": "以文本方式调用工具",
  "storymaker.agent.outcome_ready_title": "你的故事已完成。请观看一遍。",
  "storymaker.agent.outcome_ready_text": "场景、素材和时间已检查。重叠和文字是否放得下未自动检查。",
  "storymaker.agent.outcome_incomplete_title": "你的故事已部分完成。",
  "storymaker.agent.outcome_incomplete_text": "还有 {n} 个场景需要处理。有效的场景已保存。",
  "storymaker.agent.outcome_incomplete_some": "部分内容仍需处理。有效的场景已保存。",
  "storymaker.agent.outcome_failed_title": "代理无法写出这个故事。",
  "storymaker.agent.outcome_failed_text": "请用更短的主题重试，或检查模型设置。",
  "storymaker.agent.finish_instruction": "完成仍需处理的场景。",
  "storymaker.agent.check_layout_unverified": "重叠和文字是否放得下未检查。请观看一遍影片。",
  "storymaker.agent.check_compile_unverified": "没有可用的浏览器来测量时间。请在预览中检查时长。",
  "storymaker.agent.check_narration_unavailable": "此语言没有可用的语音。请设置语音并生成旁白，或不带语音导出。",
  "storymaker.agent.check_narration_failed": "无法生成旁白。请在语音面板中重试。",
  "storymaker.agent.check_narration_budget": "已达到旁白字符上限。请明天重试，或提高上限。",
  "storymaker.agent.check_narration_pending": "旁白仍在生成。完成后再导出。",
  "storymaker.agent.check_corrections_exhausted": "某个场景的修正次数已用完。请让作者完成它。",
  "storymaker.agent.check_model_error": "模型停止了响应。请让作者完成故事。",
  "storymaker.agent.check_scene_unfilled": "这个场景仍需处理。请让作者完成它。",
  "storymaker.agent.check_compile_failed": "预览无法准备这部影片。请让作者修复，或查看问题列表。",
  "storymaker.agent.check_over_target": "影片比你要求的长。请让作者缩短。",
  "storymaker.agent.check_over_budget": "某个场景的字数超出计划。请让作者缩短。",
  "storymaker.agent.check_unknown": "还有内容需要查看。",
  "storymaker.agent.make_silent": "不带语音导出",
  "storymaker.agent.make_silent_done": "语音现在是屏幕文字，影片可以导出了。",
  "storymaker.agent.restore_speech": "恢复语音",
  "storymaker.agent.refused_invalid_story": "故事有错误，作者无法修改。请先修复错误。",
  "storymaker.error.agent_job_active": "AI 作者正在处理这个故事。请在完成后重试。",
  "storymaker.error.story_not_empty": "这个故事已有内容。请改为描述一项修改。",
  "storymaker.error.model_too_small": "配置的模型对 AI 作者来说太小。请在配置中选择更大的模型。",
  "storymaker.error.llm_unavailable": "目前没有可用的 AI 模型。",
  "storymaker.error.daily_limit_reached": "AI 作者已达到每日上限。请明天重试。",
  "storymaker.job_error.model_error": "AI 模型没有给出可用的回答。"
```

- [ ] **Step 4: Test.** The existing guards cover this task, and no new test is added.

- [ ] **Step 5: Run.**

`npm run build:ui`, then `npm run check:ui`. Expected: both exit 0.

`GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./ui -run 'Storymaker|TestDesktopStorymaker|TestConfigStorymaker' -count=1`. Expected: PASS, or failures that are a subset of `evidence/baseline.md`. `TestStorymakerAppUsesOnlyKeysItHas`, `TestStorymakerLocalesCoverAllSixteenLanguages` and `TestStorymakerAppTranslationsAreTranslated` must pass.

- [ ] **Step 6: Commit.** Stage `ui/js/desktop/apps/storymaker-core.js ui/lang/storymaker/cs.json ui/lang/storymaker/da.json ui/lang/storymaker/de.json ui/lang/storymaker/el.json ui/lang/storymaker/en.json ui/lang/storymaker/es.json ui/lang/storymaker/fr.json ui/lang/storymaker/hi.json ui/lang/storymaker/it.json ui/lang/storymaker/ja.json ui/lang/storymaker/nl.json ui/lang/storymaker/no.json ui/lang/storymaker/pl.json ui/lang/storymaker/pt.json ui/lang/storymaker/sv.json ui/lang/storymaker/zh.json`, plus `ui/js/desktop/bundles/main.bundle.js` only if `build:ui` changed it. Message: `feat(storymaker): translate the AI author and add its error texts`

### Task D6a: Lists on the outcome card, locales

**Batch:** D  **Depends on:** D6
**Files:**
Modify `ui/js/desktop/apps/storymaker-agent.js`
Modify `ui/lang/storymaker/cs.json`
Modify `ui/lang/storymaker/da.json`
Modify `ui/lang/storymaker/de.json`
Modify `ui/lang/storymaker/el.json`
Modify `ui/lang/storymaker/en.json`
Modify `ui/lang/storymaker/es.json`
Modify `ui/lang/storymaker/fr.json`
Modify `ui/lang/storymaker/hi.json`
Modify `ui/lang/storymaker/it.json`
Modify `ui/lang/storymaker/ja.json`
Modify `ui/lang/storymaker/nl.json`
Modify `ui/lang/storymaker/no.json`
Modify `ui/lang/storymaker/pl.json`
Modify `ui/lang/storymaker/pt.json`
Modify `ui/lang/storymaker/sv.json`
Modify `ui/lang/storymaker/zh.json`

- [ ] **Step 1: Impact.** Run impact for `paintOutcome` in `ui/js/desktop/apps/storymaker-agent.js`. Report callers and risk. Warn on HIGH or CRITICAL.

- [ ] **Step 2: Implement the lists on the outcome card.** The lists come from the job's `changed_scenes`, `unchanged_scenes` and `removed_scenes`, which the revise job sets (C7b). Each non-empty list is one line. A scene that the story still has shows its position and id, and a removed scene shows its id only.

**Edit** `ui/js/desktop/apps/storymaker-agent.js`
Old:
```js
        function paintOutcome() {
```
New:
```js
        // The scenes of one outcome list: the position and the id where the story still has the scene, else the id alone.
        function sceneLabels(ids) {
            const scenes = S.model.scenesOf(inst.story);
            return ids.map(id => {
                const index = scenes.findIndex(scene => scene && scene.id === id);
                return index >= 0 ? (index + 1) + ' (' + id + ')' : String(id);
            }).join(', ');
        }

        function paintOutcome() {
```

**Edit** `ui/js/desktop/apps/storymaker-agent.js`
Old:
```js
            if (checks.length) outcomeCard.appendChild(el('ul', { class: 'stm-agent-checks' }, checks.map(checkLine)));
```
New:
```js
            [['changed_scenes', 'storymaker.agent.changed_scenes'], ['unchanged_scenes', 'storymaker.agent.unchanged_scenes'], ['removed_scenes', 'storymaker.agent.removed_scenes']].forEach(([field, key]) => {
                const ids = job.agent && Array.isArray(job.agent[field]) ? job.agent[field] : [];
                if (ids.length) outcomeCard.appendChild(el('p', { dataset: { stm: 'agent-scenes-list', list: field }, text: t(key, { scenes: sceneLabels(ids) }) }));
            });
            if (checks.length) outcomeCard.appendChild(el('ul', { class: 'stm-agent-checks' }, checks.map(checkLine)));
```

- [ ] **Step 3: Add the three list keys and `storymaker.agent.restore_confirm` in all 16 locales.** Each edit adds the keys after the last key of the file, `storymaker.job_error.model_error`. The texts are translations, not English copies.

**Edit** `ui/lang/storymaker/cs.json`
Old:
```json
  "storymaker.job_error.model_error": "Model AI nedal použitelnou odpověď."
```
New:
```json
  "storymaker.job_error.model_error": "Model AI nedal použitelnou odpověď.",
  "storymaker.agent.changed_scenes": "Změněno: {scenes}",
  "storymaker.agent.unchanged_scenes": "Beze změny: {scenes}",
  "storymaker.agent.removed_scenes": "Odebráno: {scenes}",
  "storymaker.agent.restore_confirm": "Obnovit řeč? Změny příběhu od chvíle, kdy se řeč odebrala, se ztratí."
```

**Edit** `ui/lang/storymaker/da.json`
Old:
```json
  "storymaker.job_error.model_error": "AI-modellen gav ikke et brugbart svar."
```
New:
```json
  "storymaker.job_error.model_error": "AI-modellen gav ikke et brugbart svar.",
  "storymaker.agent.changed_scenes": "Ændret: {scenes}",
  "storymaker.agent.unchanged_scenes": "Uændret: {scenes}",
  "storymaker.agent.removed_scenes": "Fjernet: {scenes}",
  "storymaker.agent.restore_confirm": "Gendan tale? Ændringer i historien siden talen blev fjernet, går tabt."
```

**Edit** `ui/lang/storymaker/de.json`
Old:
```json
  "storymaker.job_error.model_error": "Das KI-Modell hat keine brauchbare Antwort gegeben."
```
New:
```json
  "storymaker.job_error.model_error": "Das KI-Modell hat keine brauchbare Antwort gegeben.",
  "storymaker.agent.changed_scenes": "Geändert: {scenes}",
  "storymaker.agent.unchanged_scenes": "Unverändert: {scenes}",
  "storymaker.agent.removed_scenes": "Entfernt: {scenes}",
  "storymaker.agent.restore_confirm": "Sprache wiederherstellen? Änderungen an der Geschichte seit dem Stummschalten gehen verloren."
```

**Edit** `ui/lang/storymaker/el.json`
Old:
```json
  "storymaker.job_error.model_error": "Το μοντέλο AI δεν έδωσε χρήσιμη απάντηση."
```
New:
```json
  "storymaker.job_error.model_error": "Το μοντέλο AI δεν έδωσε χρήσιμη απάντηση.",
  "storymaker.agent.changed_scenes": "Τροποποιήθηκαν: {scenes}",
  "storymaker.agent.unchanged_scenes": "Δεν άλλαξαν: {scenes}",
  "storymaker.agent.removed_scenes": "Αφαιρέθηκαν: {scenes}",
  "storymaker.agent.restore_confirm": "Επαναφορά της ομιλίας; Οι αλλαγές στην ιστορία από τότε που αφαιρέθηκε η ομιλία θα χαθούν."
```

**Edit** `ui/lang/storymaker/en.json`
Old:
```json
  "storymaker.job_error.model_error": "The AI model did not give a usable answer."
```
New:
```json
  "storymaker.job_error.model_error": "The AI model did not give a usable answer.",
  "storymaker.agent.changed_scenes": "Changed: {scenes}",
  "storymaker.agent.unchanged_scenes": "Unchanged: {scenes}",
  "storymaker.agent.removed_scenes": "Removed: {scenes}",
  "storymaker.agent.restore_confirm": "Restore the speech? Changes made to the story since it was made silent are lost."
```

**Edit** `ui/lang/storymaker/es.json`
Old:
```json
  "storymaker.job_error.model_error": "El modelo de IA no dio una respuesta útil."
```
New:
```json
  "storymaker.job_error.model_error": "El modelo de IA no dio una respuesta útil.",
  "storymaker.agent.changed_scenes": "Modificadas: {scenes}",
  "storymaker.agent.unchanged_scenes": "Sin cambios: {scenes}",
  "storymaker.agent.removed_scenes": "Eliminadas: {scenes}",
  "storymaker.agent.restore_confirm": "¿Restaurar el discurso? Se perderán los cambios hechos en la historia desde que se silenció."
```

**Edit** `ui/lang/storymaker/fr.json`
Old:
```json
  "storymaker.job_error.model_error": "Le modèle d'IA n'a pas donné de réponse exploitable."
```
New:
```json
  "storymaker.job_error.model_error": "Le modèle d'IA n'a pas donné de réponse exploitable.",
  "storymaker.agent.changed_scenes": "Modifiées : {scenes}",
  "storymaker.agent.unchanged_scenes": "Inchangées : {scenes}",
  "storymaker.agent.removed_scenes": "Supprimées : {scenes}",
  "storymaker.agent.restore_confirm": "Rétablir la parole ? Les modifications apportées à l'histoire depuis qu'elle est muette seront perdues."
```

**Edit** `ui/lang/storymaker/hi.json`
Old:
```json
  "storymaker.job_error.model_error": "एआई मॉडल ने उपयोगी उत्तर नहीं दिया।"
```
New:
```json
  "storymaker.job_error.model_error": "एआई मॉडल ने उपयोगी उत्तर नहीं दिया।",
  "storymaker.agent.changed_scenes": "बदली गईं: {scenes}",
  "storymaker.agent.unchanged_scenes": "बिना बदलाव: {scenes}",
  "storymaker.agent.removed_scenes": "हटाई गईं: {scenes}",
  "storymaker.agent.restore_confirm": "भाषण वापस लाएँ? भाषण हटाए जाने के बाद कहानी में किए गए बदलाव खो जाएँगे।"
```

**Edit** `ui/lang/storymaker/it.json`
Old:
```json
  "storymaker.job_error.model_error": "Il modello IA non ha dato una risposta utilizzabile."
```
New:
```json
  "storymaker.job_error.model_error": "Il modello IA non ha dato una risposta utilizzabile.",
  "storymaker.agent.changed_scenes": "Modificate: {scenes}",
  "storymaker.agent.unchanged_scenes": "Invariate: {scenes}",
  "storymaker.agent.removed_scenes": "Rimosse: {scenes}",
  "storymaker.agent.restore_confirm": "Ripristinare il parlato? Le modifiche alla storia fatte da quando è stata resa muta andranno perse."
```

**Edit** `ui/lang/storymaker/ja.json`
Old:
```json
  "storymaker.job_error.model_error": "AIモデルから使える回答が得られませんでした。"
```
New:
```json
  "storymaker.job_error.model_error": "AIモデルから使える回答が得られませんでした。",
  "storymaker.agent.changed_scenes": "変更: {scenes}",
  "storymaker.agent.unchanged_scenes": "変更なし: {scenes}",
  "storymaker.agent.removed_scenes": "削除: {scenes}",
  "storymaker.agent.restore_confirm": "音声を元に戻しますか？ 無音にしてからの物語の変更は失われます。"
```

**Edit** `ui/lang/storymaker/nl.json`
Old:
```json
  "storymaker.job_error.model_error": "Het AI-model gaf geen bruikbaar antwoord."
```
New:
```json
  "storymaker.job_error.model_error": "Het AI-model gaf geen bruikbaar antwoord.",
  "storymaker.agent.changed_scenes": "Gewijzigd: {scenes}",
  "storymaker.agent.unchanged_scenes": "Ongewijzigd: {scenes}",
  "storymaker.agent.removed_scenes": "Verwijderd: {scenes}",
  "storymaker.agent.restore_confirm": "Spraak herstellen? Wijzigingen in het verhaal sinds het stil is gemaakt, gaan verloren."
```

**Edit** `ui/lang/storymaker/no.json`
Old:
```json
  "storymaker.job_error.model_error": "KI-modellen ga ikke et brukbart svar."
```
New:
```json
  "storymaker.job_error.model_error": "KI-modellen ga ikke et brukbart svar.",
  "storymaker.agent.changed_scenes": "Endret: {scenes}",
  "storymaker.agent.unchanged_scenes": "Uendret: {scenes}",
  "storymaker.agent.removed_scenes": "Fjernet: {scenes}",
  "storymaker.agent.restore_confirm": "Gjenopprette tale? Endringer i historien siden talen ble fjernet, går tapt."
```

**Edit** `ui/lang/storymaker/pl.json`
Old:
```json
  "storymaker.job_error.model_error": "Model AI nie dał użytecznej odpowiedzi."
```
New:
```json
  "storymaker.job_error.model_error": "Model AI nie dał użytecznej odpowiedzi.",
  "storymaker.agent.changed_scenes": "Zmienione: {scenes}",
  "storymaker.agent.unchanged_scenes": "Bez zmian: {scenes}",
  "storymaker.agent.removed_scenes": "Usunięte: {scenes}",
  "storymaker.agent.restore_confirm": "Przywrócić mowę? Zmiany w historii wprowadzone od wyciszenia zostaną utracone."
```

**Edit** `ui/lang/storymaker/pt.json`
Old:
```json
  "storymaker.job_error.model_error": "O modelo de IA não deu uma resposta utilizável."
```
New:
```json
  "storymaker.job_error.model_error": "O modelo de IA não deu uma resposta utilizável.",
  "storymaker.agent.changed_scenes": "Alteradas: {scenes}",
  "storymaker.agent.unchanged_scenes": "Sem alterações: {scenes}",
  "storymaker.agent.removed_scenes": "Removidas: {scenes}",
  "storymaker.agent.restore_confirm": "Repor a fala? As alterações feitas à história desde que ficou sem fala serão perdidas."
```

**Edit** `ui/lang/storymaker/sv.json`
Old:
```json
  "storymaker.job_error.model_error": "AI-modellen gav inget användbart svar."
```
New:
```json
  "storymaker.job_error.model_error": "AI-modellen gav inget användbart svar.",
  "storymaker.agent.changed_scenes": "Ändrade: {scenes}",
  "storymaker.agent.unchanged_scenes": "Oförändrade: {scenes}",
  "storymaker.agent.removed_scenes": "Borttagna: {scenes}",
  "storymaker.agent.restore_confirm": "Återställ tal? Ändringar i berättelsen sedan den gjordes tyst går förlorade."
```

**Edit** `ui/lang/storymaker/zh.json`
Old:
```json
  "storymaker.job_error.model_error": "AI 模型没有给出可用的回答。"
```
New:
```json
  "storymaker.job_error.model_error": "AI 模型没有给出可用的回答。",
  "storymaker.agent.changed_scenes": "已修改：{scenes}",
  "storymaker.agent.unchanged_scenes": "未修改：{scenes}",
  "storymaker.agent.removed_scenes": "已删除：{scenes}",
  "storymaker.agent.restore_confirm": "恢复语音？自静音以来对故事所做的更改将会丢失。"
```

- [ ] **Step 4: Test.** The existing guards cover this task, and no new test is added. `TestStorymakerAppTranslationsAreTranslated` checks that the new keys are not English copies outside `en.json`.

- [ ] **Step 5: Run.**

`npm run build:ui`, then `npm run check:ui`. Expected: both exit 0.

`GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./ui -run 'Storymaker|TestDesktopStorymaker|TestConfigStorymaker' -count=1`. Expected: PASS, or failures that are a subset of `evidence/baseline.md`.

- [ ] **Step 6: Commit.** Stage `ui/js/desktop/apps/storymaker-agent.js`, `ui/lang/storymaker/*.json` (16 files), plus `ui/js/desktop/bundles/main.bundle.js` only if `build:ui` changed it. Message: `feat(storymaker): list the changed scenes on the AI author outcome card`

### Task D7: Desktop notification

**Batch:** D  **Depends on:** C7, D1
**Files:**
Create `internal/server/storymaker_agent_facade.go`
Modify `internal/server/storymaker_agent_handlers.go`
Modify `ui/js/desktop/core/shell-chrome-runtime.js`
Modify `ui/js/desktop/bundles/main.bundle.js`
Modify `ui/lang/backend/cs.json`
Modify `ui/lang/backend/da.json`
Modify `ui/lang/backend/de.json`
Modify `ui/lang/backend/el.json`
Modify `ui/lang/backend/en.json`
Modify `ui/lang/backend/es.json`
Modify `ui/lang/backend/fr.json`
Modify `ui/lang/backend/hi.json`
Modify `ui/lang/backend/it.json`
Modify `ui/lang/backend/ja.json`
Modify `ui/lang/backend/nl.json`
Modify `ui/lang/backend/no.json`
Modify `ui/lang/backend/pl.json`
Modify `ui/lang/backend/pt.json`
Modify `ui/lang/backend/sv.json`
Modify `ui/lang/backend/zh.json`
Test `internal/server/storymaker_agent_notify_test.go`

- [ ] **Step 1: Impact.** Run impact on `storymakerStartAgentJob`. Report callers and risk.

- [ ] **Step 2: Implement the notification.** A watcher starts in `storymakerStartAgentJob`, so every start path (the app, the project list and chat) gets one notification per job, and the runner stays unchanged. Cancelled and interrupted jobs stay silent. No model text reaches the notification. A click opens the project: `notificationContext` passes the project id as `projectId`, the key the storymaker window reads.

**Create** `internal/server/storymaker_agent_facade.go`:
```go
package server

import (
	"time"

	"aurago/internal/desktop"
	"aurago/internal/i18n"
	"aurago/internal/storymaker"
	"aurago/internal/storymaker/render"
)

// storymakerAgentNotifyWhenDone waits for the end of one agent job and posts its one notification.
// Every job reaches a terminal state, also when the manager closes, so this goroutine always ends.
func (s *Server) storymakerAgentNotifyWhenDone(manager *render.Manager, job render.Job) {
	changed, unsubscribe := manager.Subscribe(job.ProjectID)
	defer unsubscribe()
	for {
		current, ok := manager.Get(job.ID)
		if !ok {
			return
		}
		if current.Terminal() {
			s.notifyStorymakerAgentJob(current)
			return
		}
		<-changed
	}
}

// notifyStorymakerAgentJob is the Desktop sink for a finished agent job. It stays silent without
// Virtual Desktop.
func (s *Server) notifyStorymakerAgentJob(job render.Job) {
	cfg := s.ConfigSnapshot()
	if cfg == nil || !cfg.VirtualDesktop.Enabled {
		return
	}
	kind, title, message, ok := storymakerAgentNotification(desktopUILanguage(s), job)
	if !ok {
		return
	}
	s.DesktopMu.Lock()
	hub := s.DesktopHub
	s.DesktopMu.Unlock()
	broadcastDesktopEvent(s, hub, desktop.Event{Type: "notification", Payload: map[string]any{
		"title":   title,
		"message": message,
		"type":    kind,
		"appId":   "storymaker",
		"context": map[string]any{"project_id": job.ProjectID},
	}, CreatedAt: time.Now().UTC()})
}

// storymakerAgentNotification picks the text of the outcome. A cancelled or interrupted job has none.
func storymakerAgentNotification(lang string, job render.Job) (kind, title, message string, ok bool) {
	if job.Status == render.StatusCancelled || job.Status == render.StatusInterrupted {
		return "", "", "", false
	}
	outcome := ""
	if job.Agent != nil {
		outcome = job.Agent.Outcome
	}
	if job.Status == render.StatusFailed {
		outcome = storymaker.OutcomeFailed
	}
	switch outcome {
	case storymaker.OutcomeReady, storymaker.OutcomeReadyUnverified:
		return "info", i18n.T(lang, "backend.storymaker_agent_ready_title"), i18n.T(lang, "backend.storymaker_agent_ready_message"), true
	case storymaker.OutcomeIncomplete:
		return "info", i18n.T(lang, "backend.storymaker_agent_incomplete_title"),
			i18n.T(lang, "backend.storymaker_agent_incomplete_message", map[string]any{"n": storymakerAgentScenesToFinish(job.Agent)}), true
	case storymaker.OutcomeFailed:
		return "error", i18n.T(lang, "backend.storymaker_agent_failed_title"), storymakerAgentFailureMessage(lang, job), true
	}
	return "", "", "", false
}

// storymakerAgentScenesToFinish counts the scenes named by open checks, at least one.
func storymakerAgentScenesToFinish(info *storymaker.AgentJobInfo) int {
	scenes := map[string]bool{}
	if info != nil {
		for _, check := range info.OpenChecks {
			if check.SceneID != "" {
				scenes[check.SceneID] = true
			}
		}
	}
	if len(scenes) == 0 {
		return 1
	}
	return len(scenes)
}

// storymakerAgentFailureMessage is the localised job_error text of the app, which i18n merges into
// the same store as the backend texts.
func storymakerAgentFailureMessage(lang string, job render.Job) string {
	if job.Error != nil {
		key := "storymaker.job_error." + job.Error.Code
		if text := i18n.T(lang, key); text != key {
			return text
		}
	}
	return i18n.T(lang, "storymaker.job_error.unknown")
}
```

**Edit** `internal/server/storymaker_agent_handlers.go`
Old:
```go
	case replayed:
		return render.Job{}, storymakerAgentBusy(job.ID)
	}
	return job, nil
```
New:
```go
	case replayed:
		return render.Job{}, storymakerAgentBusy(job.ID)
	}
	go s.storymakerAgentNotifyWhenDone(acc.rt.agent, job)
	return job, nil
```

**Edit** `ui/js/desktop/core/shell-chrome-runtime.js`
Old:
```js
        if (payload.appId === 'meshcore' && /^[a-f0-9]{64}$/.test(c.conversation_id || '')) return { conversation_id: c.conversation_id };
```
New:
```js
        if (payload.appId === 'meshcore' && /^[a-f0-9]{64}$/.test(c.conversation_id || '')) return { conversation_id: c.conversation_id };
        if (payload.appId === 'storymaker' && /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/.test(c.project_id || '')) return { projectId: c.project_id };
```

- [ ] **Step 3: Add the notification texts in all 16 locales.**

**Edit** `ui/lang/backend/cs.json`
Old:
```json
  "backend.budget_disabled":
```
New:
```json
  "backend.storymaker_agent_ready_title": "Váš příběh je hotový. Podívejte se na něj prosím jednou.",
  "backend.storymaker_agent_ready_message": "Scény, prostředky a časování byly zkontrolovány. Překryvy a umístění textu nebyly automaticky zkontrolovány.",
  "backend.storymaker_agent_incomplete_title": "Váš příběh je částečně hotový.",
  "backend.storymaker_agent_incomplete_message": "Počet scén, které ještě potřebují práci: {n}. Platné scény jsou uloženy.",
  "backend.storymaker_agent_failed_title": "Agent nemohl tento příběh napsat.",
  "backend.budget_disabled":
```

**Edit** `ui/lang/backend/da.json`
Old:
```json
  "backend.budget_disabled":
```
New:
```json
  "backend.storymaker_agent_ready_title": "Din historie er klar. Se den igennem én gang.",
  "backend.storymaker_agent_ready_message": "Scener, ressourcer og timing er kontrolleret. Overlapninger og teksttilpasning er ikke kontrolleret automatisk.",
  "backend.storymaker_agent_incomplete_title": "Din historie er delvist færdig.",
  "backend.storymaker_agent_incomplete_message": "{n} scener mangler stadig arbejde. Gyldige scener er gemt.",
  "backend.storymaker_agent_failed_title": "Agenten kunne ikke skrive denne historie.",
  "backend.budget_disabled":
```

**Edit** `ui/lang/backend/de.json`
Old:
```json
  "backend.budget_disabled":
```
New:
```json
  "backend.storymaker_agent_ready_title": "Deine Story ist fertig. Bitte schau sie dir einmal an.",
  "backend.storymaker_agent_ready_message": "Szenen, Assets und Timing wurden geprüft. Überlappungen und Textpassung wurden nicht automatisch geprüft.",
  "backend.storymaker_agent_incomplete_title": "Deine Story ist teilweise fertig.",
  "backend.storymaker_agent_incomplete_message": "{n} Szenen brauchen noch Arbeit. Gültige Szenen sind gespeichert.",
  "backend.storymaker_agent_failed_title": "Der Agent konnte diese Story nicht schreiben.",
  "backend.budget_disabled":
```

**Edit** `ui/lang/backend/el.json`
Old:
```json
  "backend.budget_disabled":
```
New:
```json
  "backend.storymaker_agent_ready_title": "Η ιστορία σας είναι έτοιμη. Παρακαλούμε δείτε την μία φορά.",
  "backend.storymaker_agent_ready_message": "Οι σκηνές, οι πόροι και ο χρονισμός ελέγχθηκαν. Οι επικαλύψεις και η προσαρμογή του κειμένου δεν ελέγχθηκαν αυτόματα.",
  "backend.storymaker_agent_incomplete_title": "Η ιστορία σας είναι εν μέρει έτοιμη.",
  "backend.storymaker_agent_incomplete_message": "Σκηνές που χρειάζονται ακόμη δουλειά: {n}. Οι έγκυρες σκηνές έχουν αποθηκευτεί.",
  "backend.storymaker_agent_failed_title": "Ο πράκτορας δεν μπόρεσε να γράψει αυτή την ιστορία.",
  "backend.budget_disabled":
```

**Edit** `ui/lang/backend/en.json`
Old:
```json
  "backend.budget_disabled":
```
New:
```json
  "backend.storymaker_agent_ready_title": "Your story is ready. Please watch it once.",
  "backend.storymaker_agent_ready_message": "Scenes, assets and timing were checked. Overlaps and text fit were not checked automatically.",
  "backend.storymaker_agent_incomplete_title": "Your story is partly done.",
  "backend.storymaker_agent_incomplete_message": "{n} scenes still need work. Valid scenes are saved.",
  "backend.storymaker_agent_failed_title": "The agent could not write this story.",
  "backend.budget_disabled":
```

**Edit** `ui/lang/backend/es.json`
Old:
```json
  "backend.budget_disabled":
```
New:
```json
  "backend.storymaker_agent_ready_title": "Tu historia está lista. Mírala una vez, por favor.",
  "backend.storymaker_agent_ready_message": "Se comprobaron las escenas, los recursos y los tiempos. Los solapamientos y el ajuste del texto no se comprobaron automáticamente.",
  "backend.storymaker_agent_incomplete_title": "Tu historia está terminada en parte.",
  "backend.storymaker_agent_incomplete_message": "{n} escenas aún necesitan trabajo. Las escenas válidas están guardadas.",
  "backend.storymaker_agent_failed_title": "El agente no pudo escribir esta historia.",
  "backend.budget_disabled":
```

**Edit** `ui/lang/backend/fr.json`
Old:
```json
  "backend.budget_disabled":
```
New:
```json
  "backend.storymaker_agent_ready_title": "Votre histoire est prête. Regardez-la une fois, s'il vous plaît.",
  "backend.storymaker_agent_ready_message": "Les scènes, les ressources et le minutage ont été vérifiés. Les chevauchements et l'ajustement du texte n'ont pas été vérifiés automatiquement.",
  "backend.storymaker_agent_incomplete_title": "Votre histoire est en partie terminée.",
  "backend.storymaker_agent_incomplete_message": "{n} scènes demandent encore du travail. Les scènes valides sont enregistrées.",
  "backend.storymaker_agent_failed_title": "L'agent n'a pas pu écrire cette histoire.",
  "backend.budget_disabled":
```

**Edit** `ui/lang/backend/hi.json`
Old:
```json
  "backend.budget_disabled":
```
New:
```json
  "backend.storymaker_agent_ready_title": "आपकी कहानी तैयार है। कृपया इसे एक बार देख लें।",
  "backend.storymaker_agent_ready_message": "दृश्य, संसाधन और समय की जाँच हो गई है। ओवरलैप और टेक्स्ट का फिट स्वचालित रूप से नहीं जाँचा गया।",
  "backend.storymaker_agent_incomplete_title": "आपकी कहानी आंशिक रूप से तैयार है।",
  "backend.storymaker_agent_incomplete_message": "{n} दृश्यों पर अभी काम बाकी है। मान्य दृश्य सहेज लिए गए हैं।",
  "backend.storymaker_agent_failed_title": "एजेंट यह कहानी नहीं लिख सका।",
  "backend.budget_disabled":
```

**Edit** `ui/lang/backend/it.json`
Old:
```json
  "backend.budget_disabled":
```
New:
```json
  "backend.storymaker_agent_ready_title": "La tua storia è pronta. Guardala una volta, per favore.",
  "backend.storymaker_agent_ready_message": "Scene, risorse e tempi sono stati controllati. Sovrapposizioni e adattamento del testo non sono stati controllati automaticamente.",
  "backend.storymaker_agent_incomplete_title": "La tua storia è completata in parte.",
  "backend.storymaker_agent_incomplete_message": "{n} scene richiedono ancora lavoro. Le scene valide sono salvate.",
  "backend.storymaker_agent_failed_title": "L'agente non è riuscito a scrivere questa storia.",
  "backend.budget_disabled":
```

**Edit** `ui/lang/backend/ja.json`
Old:
```json
  "backend.budget_disabled":
```
New:
```json
  "backend.storymaker_agent_ready_title": "ストーリーができました。一度ご確認ください。",
  "backend.storymaker_agent_ready_message": "シーン、アセット、タイミングは確認済みです。重なりとテキストの収まりは自動では確認していません。",
  "backend.storymaker_agent_incomplete_title": "ストーリーは一部だけ完成しました。",
  "backend.storymaker_agent_incomplete_message": "あと {n} 個のシーンに作業が必要です。有効なシーンは保存されています。",
  "backend.storymaker_agent_failed_title": "エージェントはこのストーリーを作成できませんでした。",
  "backend.budget_disabled":
```

**Edit** `ui/lang/backend/nl.json`
Old:
```json
  "backend.budget_disabled":
```
New:
```json
  "backend.storymaker_agent_ready_title": "Je verhaal is klaar. Bekijk het alsjeblieft één keer.",
  "backend.storymaker_agent_ready_message": "Scènes, middelen en timing zijn gecontroleerd. Overlappingen en tekstpassing zijn niet automatisch gecontroleerd.",
  "backend.storymaker_agent_incomplete_title": "Je verhaal is gedeeltelijk klaar.",
  "backend.storymaker_agent_incomplete_message": "{n} scènes hebben nog werk nodig. Geldige scènes zijn opgeslagen.",
  "backend.storymaker_agent_failed_title": "De agent kon dit verhaal niet schrijven.",
  "backend.budget_disabled":
```

**Edit** `ui/lang/backend/no.json`
Old:
```json
  "backend.budget_disabled":
```
New:
```json
  "backend.storymaker_agent_ready_title": "Historien din er klar. Se gjennom den én gang.",
  "backend.storymaker_agent_ready_message": "Scener, ressurser og timing er kontrollert. Overlappinger og teksttilpasning er ikke kontrollert automatisk.",
  "backend.storymaker_agent_incomplete_title": "Historien din er delvis ferdig.",
  "backend.storymaker_agent_incomplete_message": "{n} scener trenger fortsatt arbeid. Gyldige scener er lagret.",
  "backend.storymaker_agent_failed_title": "Agenten kunne ikke skrive denne historien.",
  "backend.budget_disabled":
```

**Edit** `ui/lang/backend/pl.json`
Old:
```json
  "backend.budget_disabled":
```
New:
```json
  "backend.storymaker_agent_ready_title": "Twoja historia jest gotowa. Obejrzyj ją raz.",
  "backend.storymaker_agent_ready_message": "Sceny, zasoby i czas zostały sprawdzone. Nakładanie się elementów i dopasowanie tekstu nie zostały sprawdzone automatycznie.",
  "backend.storymaker_agent_incomplete_title": "Twoja historia jest częściowo gotowa.",
  "backend.storymaker_agent_incomplete_message": "Liczba scen wymagających pracy: {n}. Poprawne sceny są zapisane.",
  "backend.storymaker_agent_failed_title": "Agent nie mógł napisać tej historii.",
  "backend.budget_disabled":
```

**Edit** `ui/lang/backend/pt.json`
Old:
```json
  "backend.budget_disabled":
```
New:
```json
  "backend.storymaker_agent_ready_title": "A sua história está pronta. Veja-a uma vez, por favor.",
  "backend.storymaker_agent_ready_message": "As cenas, os recursos e os tempos foram verificados. As sobreposições e o ajuste do texto não foram verificados automaticamente.",
  "backend.storymaker_agent_incomplete_title": "A sua história está parcialmente concluída.",
  "backend.storymaker_agent_incomplete_message": "{n} cenas ainda precisam de trabalho. As cenas válidas estão guardadas.",
  "backend.storymaker_agent_failed_title": "O agente não conseguiu escrever esta história.",
  "backend.budget_disabled":
```

**Edit** `ui/lang/backend/sv.json`
Old:
```json
  "backend.budget_disabled":
```
New:
```json
  "backend.storymaker_agent_ready_title": "Din berättelse är klar. Titta igenom den en gång.",
  "backend.storymaker_agent_ready_message": "Scener, resurser och timing har kontrollerats. Överlappningar och textpassning har inte kontrollerats automatiskt.",
  "backend.storymaker_agent_incomplete_title": "Din berättelse är delvis klar.",
  "backend.storymaker_agent_incomplete_message": "{n} scener behöver fortfarande arbete. Giltiga scener är sparade.",
  "backend.storymaker_agent_failed_title": "Agenten kunde inte skriva den här berättelsen.",
  "backend.budget_disabled":
```

**Edit** `ui/lang/backend/zh.json`
Old:
```json
  "backend.budget_disabled":
```
New:
```json
  "backend.storymaker_agent_ready_title": "你的故事已准备好。请观看一遍。",
  "backend.storymaker_agent_ready_message": "场景、素材和时间已检查。重叠和文字适配未经自动检查。",
  "backend.storymaker_agent_incomplete_title": "你的故事已部分完成。",
  "backend.storymaker_agent_incomplete_message": "还有 {n} 个场景需要处理。有效的场景已保存。",
  "backend.storymaker_agent_failed_title": "智能体无法写出这个故事。",
  "backend.budget_disabled":
```

- [ ] **Step 4: Test.**

**Create** `internal/server/storymaker_agent_notify_test.go`:
```go
package server

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"aurago/internal/desktop"
	"aurago/internal/i18n"
	"aurago/internal/storymaker"
	"aurago/internal/storymaker/render"
	"aurago/ui"
)

func TestStorymakerAgentNotifiesOncePerJob(t *testing.T) {
	i18n.Load(ui.Content, slog.Default())
	s, _, token := storymakerAgentRouteFixture(t, func(_ context.Context, _ render.Job, _ any, h *render.Handle) (render.Output, error) {
		h.SetAgent(storymaker.AgentJobInfo{Outcome: storymaker.OutcomeReadyUnverified})
		return render.Output{}, nil
	})
	_, hub, err := s.getDesktopService(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	events, unsubscribe, err := hub.Subscribe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(unsubscribe)

	want := map[string]bool{}
	for _, topic := range []string{"Erste Story", "Zweite Story"} {
		w := storymakerRequest(s, token, http.MethodPost, "/agent/jobs", strings.NewReader(`{"topic":"`+topic+`"}`), nil)
		if w.Code != http.StatusAccepted {
			t.Fatalf("start = %d %s", w.Code, w.Body.String())
		}
		var started struct {
			ProjectID string `json:"project_id"`
		}
		decodeBody(t, w, &started)
		want[started.ProjectID] = true
	}

	// File writes also reach the hub, so only notifications count.
	notificationProject := func(event desktop.Event) string {
		t.Helper()
		if event.Type != "notification" {
			return ""
		}
		payload, _ := event.Payload.(map[string]any)
		launch, _ := payload["context"].(map[string]any)
		id, _ := launch["project_id"].(string)
		if payload["appId"] != "storymaker" || payload["type"] != "info" || payload["title"] != "Your story is ready. Please watch it once." {
			t.Fatalf("notification = %+v", payload)
		}
		return id
	}
	seen := map[string]bool{}
	deadline := time.After(10 * time.Second)
	for len(seen) < len(want) {
		select {
		case event, open := <-events:
			if !open {
				t.Fatal("the hub closed")
			}
			if id := notificationProject(event); id != "" {
				if !want[id] || seen[id] {
					t.Fatalf("unexpected or repeated notification for project %s", id)
				}
				seen[id] = true
			}
		case <-deadline:
			t.Fatalf("notified %d of %d jobs", len(seen), len(want))
		}
	}
	quiet := time.After(300 * time.Millisecond)
	for {
		select {
		case event, open := <-events:
			if !open {
				return
			}
			if id := notificationProject(event); id != "" {
				t.Fatalf("a repeated notification for project %s", id)
			}
		case <-quiet:
			return
		}
	}
}
```

- [ ] **Step 5: Run.** Run these commands in order.
`npm run build:ui`, then `npm run check:ui`. Expected: both exit 0, and `ui/js/desktop/bundles/main.bundle.js` is rewritten.
`GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./internal/server -run '^TestStorymakerAgentNotifiesOncePerJob$' -count=1`
Expected: `ok  	aurago/internal/server`.

- [ ] **Step 6: Commit.** Stage `internal/server/storymaker_agent_facade.go`, `internal/server/storymaker_agent_handlers.go`, `internal/server/storymaker_agent_notify_test.go`, `ui/js/desktop/core/shell-chrome-runtime.js`, `ui/js/desktop/bundles/main.bundle.js` and `ui/lang/backend/*.json`. Message: `feat(storymaker): notify the desktop once when an agent job ends`

