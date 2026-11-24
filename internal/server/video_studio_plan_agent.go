package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/memory"
	"aurago/internal/tools"
	"aurago/internal/uid"
	"aurago/internal/videostudio/storyplan"

	"github.com/sashabaranov/go-openai"
)

// videoStudioPlanAgentRequest is one start of the planning agent. The routes check the request; the job binds the
// project, the mode and the revise scene from it, never from tool arguments. Idea, target and language live in the
// stored plan, which the routes write before the start.
type videoStudioPlanAgentRequest struct {
	ProjectID     string
	Mode          string // "create" | "revise"
	Idea          string
	TargetSeconds int
	Language      string
	SceneID       string
	Note          string
}

// videoStudioPlanAgentStatus is what the plan routes report about a job: the live one or the last finished one.
type videoStudioPlanAgentStatus struct {
	State      string    `json:"state"` // running | done | failed | stopped
	Mode       string    `json:"mode"`
	Phase      string    `json:"phase,omitempty"` // outline | details | check | revise
	Error      string    `json:"error,omitempty"` // code: model_error | timeout | tool_limit | stopped | plan_conflict
	Tokens     int       `json:"tokens"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at,omitempty"`
}

var (
	errVideoStudioPlanAgentBusy        = errors.New("plan_agent_running")
	errVideoStudioPlanAgentDailyLimit  = errors.New("plan_daily_limit")
	errVideoStudioPlanAgentUnavailable = errors.New("plan_agent_unavailable")
)

// The states and error codes of videoStudioPlanAgentStatus.
const (
	videoStudioPlanAgentRunning = "running"
	videoStudioPlanAgentDone    = "done"
	videoStudioPlanAgentFailed  = "failed"
	videoStudioPlanAgentStopped = "stopped"

	videoStudioPlanAgentErrModel     = "model_error"
	videoStudioPlanAgentErrTimeout   = "timeout"
	videoStudioPlanAgentErrToolLimit = "tool_limit"
	videoStudioPlanAgentErrStopped   = "stopped"
	videoStudioPlanAgentErrConflict  = "plan_conflict"
)

const (
	// videoStudioPlanAgentAttempts bounds one phase: its loop is restarted at most once, after a model error or a
	// loop in which the model called no tool.
	videoStudioPlanAgentAttempts = 2
	// videoStudioPlanAgentCheckRounds is the most check loops a create job runs.
	videoStudioPlanAgentCheckRounds = 3
)

// videoStudioPlanAgentToolLimit is the tool-call limit of one loop.
func videoStudioPlanAgentToolLimit(phase string) int {
	switch phase {
	case agent.VideoStudioPlanPhaseOutline:
		return 6
	case agent.VideoStudioPlanPhaseDetails:
		return 16
	case agent.VideoStudioPlanPhaseCheck:
		return 20
	}
	return 10
}

// videoStudioPlanAgentTemperature is the sampling temperature of one phase.
func videoStudioPlanAgentTemperature(phase string) float64 {
	switch phase {
	case agent.VideoStudioPlanPhaseOutline:
		return 0.6
	case agent.VideoStudioPlanPhaseDetails:
		return 0.4
	case agent.VideoStudioPlanPhaseCheck:
		return 0.2
	}
	return 0.3
}

// videoStudioPlanAgentRoute is the model route of one job. Compact and text mode are decided once, at the start.
type videoStudioPlanAgentRoute struct {
	cfg     *config.Config // a private copy
	client  llm.ChatClient
	compact bool
	text    bool
}

// videoStudioPlanAgentResolveRoute pins the route when video_studio.plan names a provider or a model, like
// storymakerAgentResolveRoute, and otherwise keeps the main route with its configured failover.
func videoStudioPlanAgentResolveRoute(s *Server, base *config.Config) (*videoStudioPlanAgentRoute, error) {
	cp := *base
	cfg := &cp
	providerID := strings.TrimSpace(cfg.VideoStudio.Plan.AgentProvider)
	modelID := strings.TrimSpace(cfg.VideoStudio.Plan.AgentModel)
	if providerID != "" {
		entry := cfg.FindProvider(providerID)
		if entry == nil {
			return nil, fmt.Errorf("the plan agent provider %q is not configured", providerID)
		}
		if !config.ProviderUsageAllowsGeneralChat(entry) {
			return nil, fmt.Errorf("the plan agent provider %q is not available for general chat", providerID)
		}
		cfg = llm.TaskProviderConfig(cfg, *entry)
	}
	if modelID != "" {
		cfg.LLM.Model = modelID
	}
	if strings.TrimSpace(cfg.LLM.Model) == "" {
		return nil, errors.New("the plan agent model is empty")
	}
	applyGameMakerStreamIdle(cfg)
	client := s.LLMClient
	if providerID != "" || modelID != "" {
		// The global failover client overwrites request.Model with its system route.
		cfg.FallbackLLM.Enabled = false
		client = llm.WrapOpenAIClient(llm.NewClientFromProviderWithConfig(cfg, cfg.LLM.ProviderType, cfg.LLM.BaseURL, cfg.LLM.APIKey, cfg.LLM.AccountID, cfg.LLM.GatewayID))
	}
	if client == nil {
		return nil, errors.New("no LLM is configured")
	}
	return &videoStudioPlanAgentRoute{
		cfg:     cfg,
		client:  client,
		compact: gameMakerRouteIsSmall(gameMakerRouteLimits(cfg)),
		// Never set cfg.LLM.UseNativeFunctions here: this is the policy the loop computes itself.
		text: !agent.BuildToolingPolicy(cfg, "").UseNativeFunctions,
	}, nil
}

// videoStudioPlanAgents runs the planning jobs of one server: at most one per project, at most
// video_studio.plan.max_daily_jobs starts per local day. Jobs and their last status live in memory only.
type videoStudioPlanAgents struct {
	s   *Server
	now func() time.Time

	mu    sync.Mutex
	jobs  map[string]*videoStudioPlanAgentJob
	last  map[string]videoStudioPlanAgentStatus
	day   string // local date of count
	count int    // accepted starts on day
}

// videoStudioPlanAgents returns the job manager of the server, created on first use.
func (s *Server) videoStudioPlanAgents() *videoStudioPlanAgents {
	s.videoStudioPlanAgentsOnce.Do(func() {
		s.videoStudioPlanAgentsMgr = &videoStudioPlanAgents{
			s:    s,
			now:  time.Now,
			jobs: map[string]*videoStudioPlanAgentJob{},
			last: map[string]videoStudioPlanAgentStatus{},
		}
	})
	return s.videoStudioPlanAgentsMgr
}

// Start starts a job for the project. It refuses a second job of the project, a start over the daily limit and a
// start without a usable model; a refused start does not count. plan.json is read by the job, not here.
func (m *videoStudioPlanAgents) Start(req videoStudioPlanAgentRequest) (videoStudioPlanAgentStatus, error) {
	cfg, err := normalizedVideoStudioConfig(m.s)
	if err != nil {
		return videoStudioPlanAgentStatus{}, errVideoStudioPlanAgentUnavailable
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.jobs[req.ProjectID] != nil {
		return videoStudioPlanAgentStatus{}, errVideoStudioPlanAgentBusy
	}
	if day := m.now().Format("2006-01-02"); day != m.day {
		m.day, m.count = day, 0
	}
	if m.count >= cfg.VideoStudio.Plan.MaxDailyJobs {
		return videoStudioPlanAgentStatus{}, errVideoStudioPlanAgentDailyLimit
	}
	logger := m.s.Logger
	if logger == nil {
		logger = slog.Default()
	}
	route, err := videoStudioPlanAgentResolveRoute(m.s, cfg)
	if err != nil {
		logger.Warn("[Video Studio] The plan agent has no usable model route", "error", err)
		return videoStudioPlanAgentStatus{}, errVideoStudioPlanAgentUnavailable
	}
	// A desktop run: a configuration change that makes the desktop read-only cancels the job.
	runCtx, release, err := m.s.beginDesktopRun(context.Background())
	if err != nil {
		return videoStudioPlanAgentStatus{}, errVideoStudioPlanAgentUnavailable
	}
	jobCtx, cancel := context.WithTimeout(runCtx, time.Duration(cfg.VideoStudio.Plan.JobTimeoutSeconds)*time.Second)
	status := videoStudioPlanAgentStatus{State: videoStudioPlanAgentRunning, Mode: req.Mode, StartedAt: m.now().UTC()}
	j := &videoStudioPlanAgentJob{m: m, req: req, id: uid.New(), route: route, cfg: cfg, logger: logger, cancel: cancel, status: status}
	m.jobs[req.ProjectID] = j
	m.count++
	go j.run(runCtx, jobCtx, release)
	return status, nil
}

// Stop cancels the running job of the project. It reports whether there was one.
func (m *videoStudioPlanAgents) Stop(projectID string) bool {
	m.mu.Lock()
	j := m.jobs[projectID]
	m.mu.Unlock()
	if j == nil {
		return false
	}
	j.stopped.Store(true)
	j.cancel()
	return true
}

// Status returns the running job of the project, or else its last finished one.
func (m *videoStudioPlanAgents) Status(projectID string) (videoStudioPlanAgentStatus, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if j := m.jobs[projectID]; j != nil {
		return j.snapshot(), true
	}
	status, ok := m.last[projectID]
	return status, ok
}

// Running reports whether a job runs for the project.
func (m *videoStudioPlanAgents) Running(projectID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.jobs[projectID] != nil
}

// finish replaces the running job by its final status.
func (m *videoStudioPlanAgents) finish(j *videoStudioPlanAgentJob, status videoStudioPlanAgentStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.jobs[j.req.ProjectID] == j {
		delete(m.jobs, j.req.ProjectID)
	}
	m.last[j.req.ProjectID] = status
}

// videoStudioPlanAgentJob is one running planning job. Prose never leaves it: the broker withholds the model's
// text, and the outcome is the saved plan plus a closed status.
type videoStudioPlanAgentJob struct {
	m      *videoStudioPlanAgents
	req    videoStudioPlanAgentRequest
	id     string
	route  *videoStudioPlanAgentRoute
	cfg    *config.Config // the normalized configuration of the start; plan writes use its limits
	logger *slog.Logger
	cancel context.CancelFunc

	studio *videoStudioPlanStudio
	mem    *memory.SQLiteMemory

	stopped atomic.Bool

	mu      sync.Mutex
	status  videoStudioPlanAgentStatus
	failure string // the first failure code of the job
	model   string // the last model status the broker kept, for the log
}

func (j *videoStudioPlanAgentJob) snapshot() videoStudioPlanAgentStatus {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.status
}

func (j *videoStudioPlanAgentJob) noteFailure(code string) {
	j.mu.Lock()
	if j.failure == "" {
		j.failure = code
	}
	j.mu.Unlock()
}

func (j *videoStudioPlanAgentJob) noteModelStatus(status string) {
	j.mu.Lock()
	j.model = status
	j.mu.Unlock()
}

func (j *videoStudioPlanAgentJob) modelStatus() string {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.model
}

// run is the job goroutine. Whatever happens, the plan goes to review and the manager gets the final status.
func (j *videoStudioPlanAgentJob) run(runCtx, jobCtx context.Context, release context.CancelFunc) {
	defer release()
	defer j.cancel()
	if err := j.open(jobCtx); err != nil {
		if jobCtx.Err() == nil {
			j.logger.Warn("[Video Studio] The plan agent could not open its plan", "project_id", j.req.ProjectID, "error", err)
			j.noteFailure(videoStudioPlanAgentErrConflict)
		}
	} else {
		if j.req.Mode == agent.VideoStudioPlanModeRevise {
			j.requirePhase(jobCtx, agent.VideoStudioPlanPhaseRevise)
		} else {
			j.runCreate(jobCtx)
		}
		_ = j.mem.Close()
	}
	j.m.finish(j, j.close(runCtx, jobCtx))
}

// open binds the studio to the project and marks the plan as being planned.
func (j *videoStudioPlanAgentJob) open(ctx context.Context) error {
	svc, _, err := j.m.s.getDesktopService(ctx)
	if err != nil {
		return err
	}
	manager, err := j.m.s.videoStudioManager()
	if err != nil {
		return err
	}
	studio, err := newVideoStudioPlanStudio(ctx, j.m.s, svc, manager, j.cfg, j.req.ProjectID, j.req.Mode, j.req.SceneID)
	if err != nil {
		return err
	}
	j.studio = studio
	if err := studio.SetStatus(ctx, storyplan.StatusPlanning); err != nil {
		return err
	}
	mem, err := memory.NewSQLiteMemory(":memory:", j.logger)
	if err != nil {
		return err
	}
	j.mem = mem
	return nil
}

// runCreate is the create order: outline, details, then check loops while blocking issues remain and the last loop
// changed the plan.
func (j *videoStudioPlanAgentJob) runCreate(ctx context.Context) {
	if !j.requirePhase(ctx, agent.VideoStudioPlanPhaseOutline) || !j.requirePhase(ctx, agent.VideoStudioPlanPhaseDetails) {
		return
	}
	for round := 0; round < videoStudioPlanAgentCheckRounds; round++ {
		if ctx.Err() != nil || !storyplan.HasBlocking(j.studio.Issues()) {
			return
		}
		writes := j.studio.Writes()
		j.runPhase(ctx, agent.VideoStudioPlanPhaseCheck)
		if j.studio.Conflict() {
			j.noteFailure(videoStudioPlanAgentErrConflict)
			return
		}
		if j.studio.PhaseDone() || j.studio.Writes() == writes {
			return
		}
	}
}

// requirePhase runs a phase the job cannot go on without and notes why it did not end. The check step never fails
// a job: what it leaves shows as issues on the board.
func (j *videoStudioPlanAgentJob) requirePhase(ctx context.Context, phase string) bool {
	done, modelErr := j.runPhase(ctx, phase)
	switch {
	case ctx.Err() != nil:
		return false
	case j.studio.Conflict():
		j.noteFailure(videoStudioPlanAgentErrConflict)
		return false
	case done:
		return true
	case modelErr:
		j.noteFailure(videoStudioPlanAgentErrModel)
	default:
		j.noteFailure(videoStudioPlanAgentErrToolLimit)
	}
	return false
}

// runPhase runs one phase with at most one restart. It reports whether the phase reached its end and whether its
// last attempt ended on a model error.
func (j *videoStudioPlanAgentJob) runPhase(ctx context.Context, phase string) (done, modelErr bool) {
	j.studio.SetPhase(phase)
	j.mu.Lock()
	j.status.Phase = phase
	j.mu.Unlock()
	if j.studio.PhaseDone() {
		return true, false
	}
	for attempt := 1; attempt <= videoStudioPlanAgentAttempts; attempt++ {
		calls := j.studio.Calls()
		err := j.runLoop(ctx, phase)
		if ctx.Err() != nil {
			return false, false
		}
		if j.studio.PhaseDone() {
			return true, false
		}
		modelErr = err != nil
		if err != nil {
			j.logger.Warn("[Video Studio] A plan agent loop failed", "project_id", j.req.ProjectID, "phase", phase, "attempt", attempt, "model_status", j.modelStatus(), "error", err)
		} else if j.studio.Calls() > calls {
			// The model worked through its budget: a restart would only repeat it.
			return false, false
		}
	}
	return false, modelErr
}

// runLoop runs one model loop with the prepared profile of the phase and a fresh data packet.
func (j *videoStudioPlanAgentJob) runLoop(ctx context.Context, phase string) error {
	cfg := *j.route.cfg
	cfg.LLM.Temperature = videoStudioPlanAgentTemperature(phase)
	profile, err := videoStudioPlanAgentProfile(phase, j.route.compact, j.route.text)
	if err != nil {
		return err
	}
	limit := videoStudioPlanAgentToolLimit(phase)
	plan, issues, env := j.studio.Snapshot()
	secondsLeft := 0
	if deadline, ok := ctx.Deadline(); ok {
		// Rounded, so that an unchanged situation keeps an unchanged packet.
		secondsLeft = max(0, int(time.Until(deadline).Seconds())/30*30)
	}
	data, err := json.Marshal(videoStudioPlanAgentPacket(videoStudioPlanAgentPacketInput{
		Phase: phase, Plan: plan, Issues: issues, Env: env, SceneID: j.req.SceneID, Note: j.req.Note, ToolCalls: limit, SecondsLeft: secondsLeft,
	}))
	if err != nil {
		return err
	}
	req := openai.ChatCompletionRequest{
		Model:    cfg.LLM.Model,
		Messages: []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: videoStudioPlanAgentUserMessage(phase, data)}},
		Stream:   true,
	}
	if !j.route.text {
		// Every phase ends through a tool call: a saved write or vs_plan_finish.
		req.ToolChoice = "required"
	}
	if tokens := gameMakerOutputTokens(gameMakerRouteLimits(&cfg), llm.ConservativeOutputTokens); tokens > 0 {
		req.MaxTokens = tokens
	}
	callCtx := llm.WithStreamAttemptTimeout(ctx, time.Duration(cfg.CircuitBreaker.LLMTimeoutSeconds)*time.Second)
	_, err = agent.ExecuteAgentLoop(callCtx, req, j.runConfig(&cfg, profile, phase, limit), true, newVideoStudioPlanBroker(j.noteModelStatus))
	if err != nil && agent.IsToolLimitFinalResponseInvalid(err) && ctx.Err() == nil {
		// The extra call stays unexecuted, and what was saved counts.
		return nil
	}
	return err
}

// runConfig builds the isolated run of one loop.
func (j *videoStudioPlanAgentJob) runConfig(cfg *config.Config, profile *agent.PreparedPromptProfile, phase string, limit int) agent.RunConfig {
	sessionID := "video-studio-plan-" + j.id + ":" + uid.New()
	runCfg := buildDesktopRunConfigForSession(j.m.s, cfg, j.route.client, sessionID, "video_studio_plan")
	runCfg.Logger = j.logger
	runCfg.ShortTermMem = j.mem
	runCfg.HistoryManager = memory.NewEphemeralHistoryManager()
	runCfg.Registry = tools.NewProcessRegistry(j.logger)
	runCfg.LongTermMem = nil
	runCfg.KG = nil
	runCfg.TrustedPromptAddenda = nil
	runCfg.PreparedPrompt = profile
	runCfg.NativeToolSchemas = profile.Tools()
	runCfg.AllowedTools = []string{}
	for _, definition := range runCfg.NativeToolSchemas {
		runCfg.AllowedTools = append(runCfg.AllowedTools, definition.Function.Name)
	}
	runCfg.ExecutionHooks = j.hooks()
	runCfg.UserIntent = videoStudioPlanAgentIntent(phase)
	runCfg.PreserveReasoning = true
	runCfg.StableSystemPrompt = true
	runCfg.RequireCompleteStream = true
	runCfg.RetryStreamIdle = true
	runCfg.StreamThroughput = &gameMakerStreamThroughput
	runCfg.ToolCallLimit = limit
	runCfg.IterationLimit = limit + 4
	runCfg.SuppressTurnSideEffects = true
	runCfg.IsMission = true
	runCfg.VoiceOutputActive = false
	runCfg.AllowedAgentSkills = []string{}
	runCfg.RunComplete = j.studio.PhaseDone
	runCfg.RecoveryNudge = videoStudioPlanAgentRecoveryNudge(phase)
	return runCfg
}

// hooks close over the job's studio, so a model cannot choose which project a tool call reaches.
func (j *videoStudioPlanAgentJob) hooks() *agent.ExecutionHooks {
	return &agent.ExecutionHooks{
		HandleTool: func(ctx context.Context, tc agent.ToolCall) (string, bool) {
			if !strings.HasPrefix(tc.Action, agent.VideoStudioPlanToolPrefix) {
				return "", false
			}
			return "Tool Output: " + string(j.studio.Call(ctx, tc)), true
		},
		AfterResponse: func(usage openai.Usage) error {
			j.mu.Lock()
			j.status.Tokens += max(0, usage.TotalTokens)
			j.mu.Unlock()
			return nil
		},
	}
}

// close hands the plan over for review and returns the final status. The write uses the run context, which a stop
// or the timeout does not end, so a stopped job still leaves an editable plan.
func (j *videoStudioPlanAgentJob) close(runCtx, jobCtx context.Context) videoStudioPlanAgentStatus {
	if j.studio != nil {
		if err := j.studio.SetStatus(runCtx, storyplan.StatusReview); err != nil {
			j.logger.Warn("[Video Studio] The plan agent could not hand the plan over for review", "project_id", j.req.ProjectID, "error", err)
		}
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	j.status.Phase = ""
	j.status.FinishedAt = j.m.now().UTC()
	j.status.State, j.status.Error = videoStudioPlanAgentDone, ""
	switch {
	case j.stopped.Load() || errors.Is(runCtx.Err(), context.Canceled):
		j.status.State, j.status.Error = videoStudioPlanAgentStopped, videoStudioPlanAgentErrStopped
	case errors.Is(jobCtx.Err(), context.DeadlineExceeded):
		j.status.State, j.status.Error = videoStudioPlanAgentFailed, videoStudioPlanAgentErrTimeout
	case j.failure != "":
		j.status.State, j.status.Error = videoStudioPlanAgentFailed, j.failure
	}
	return j.status
}

// videoStudioPlanBroker withholds the model's prose: deltas, reasoning, tool messages and JSON events are dropped.
// Only the closed model status reaches the job; token counts come from the AfterResponse hook.
type videoStudioPlanBroker struct {
	status func(string)
}

var _ agent.FeedbackBroker = videoStudioPlanBroker{}

func newVideoStudioPlanBroker(status func(string)) videoStudioPlanBroker {
	return videoStudioPlanBroker{status: status}
}

// Send keeps the model status and drops every other event.
func (b videoStudioPlanBroker) Send(event, message string) {
	if event != "model_progress" || b.status == nil {
		return
	}
	switch message {
	case "waiting", "receiving", "retrying", "recovering":
		b.status(message)
	}
}

// SendJSON is a no-op: a JSON event can carry tool output.
func (videoStudioPlanBroker) SendJSON(string) {}

// SendLLMStreamDelta discards the delta.
func (videoStudioPlanBroker) SendLLMStreamDelta(string, string, string, int, string) {}

// SendLLMStreamDone has nothing to report.
func (videoStudioPlanBroker) SendLLMStreamDone(string) {}

// SendTokenUpdate is a no-op: the AfterResponse hook counts the tokens.
func (videoStudioPlanBroker) SendTokenUpdate(int, int, int, int, int, bool, bool, string) {}

// SendThinkingBlock discards the reasoning.
func (videoStudioPlanBroker) SendThinkingBlock(string, string, string) {}
