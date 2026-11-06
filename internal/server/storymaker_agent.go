package server

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/memory"
	"aurago/internal/security"
	"aurago/internal/storymaker"
	"aurago/internal/storymaker/render"
	"aurago/internal/tools"
	"aurago/internal/uid"

	"github.com/sashabaranov/go-openai"
)

// storymakerAgentPayload is the payload of an agent job. It stays in memory: a job that was queued
// when the server stopped is interrupted, so it never needs to be persisted.
type storymakerAgentPayload struct {
	Kind, ProjectID, Topic, Instruction, Audience, Language, Provider, Model string
	LengthSeconds                                                            int
	Voice                                                                    *storymaker.Voice
}

// storymakerAgentRoute is the model route of one job. Compact and text mode are decided once, at job start.
type storymakerAgentRoute struct {
	cfg     *config.Config // a private copy
	client  llm.ChatClient
	compact bool
	text    bool
}

// storymakerAgentResolveRoute pins the route like Game Maker when a provider or model is named, and
// otherwise keeps the main route with its configured failover (decision 16). Every error is a *render.JobError.
func (s *Server) storymakerAgentResolveRoute(provider, model string) (*storymakerAgentRoute, error) {
	cfg, err := normalizedStorymakerConfig(s)
	if err != nil {
		return nil, render.Fail(render.CodeModelError, "The Storymaker configuration is invalid.")
	}
	providerID := strings.TrimSpace(firstNonEmpty(provider, cfg.Storymaker.Agent.Provider))
	modelID := strings.TrimSpace(firstNonEmpty(model, cfg.Storymaker.Agent.Model))
	if providerID != "" {
		entry := cfg.FindProvider(providerID)
		if entry == nil {
			return nil, render.Fail(render.CodeModelError, "The Storymaker provider %q is not configured.", providerID)
		}
		if !config.ProviderUsageAllowsGeneralChat(entry) {
			return nil, render.Fail(render.CodeModelError, "The Storymaker provider %q is not available for general chat.", providerID)
		}
		cfg = llm.TaskProviderConfig(cfg, *entry)
	}
	if modelID != "" {
		cfg.LLM.Model = modelID
	}
	if strings.TrimSpace(cfg.LLM.Model) == "" {
		return nil, render.Fail(render.CodeModelError, "The Storymaker model is empty.")
	}
	applyGameMakerStreamIdle(cfg)
	client := s.LLMClient
	if providerID != "" || modelID != "" {
		// The global failover client overwrites request.Model with its system route.
		cfg.FallbackLLM.Enabled = false
		client = llm.WrapOpenAIClient(llm.NewClientFromProviderWithConfig(cfg, cfg.LLM.ProviderType, cfg.LLM.BaseURL, cfg.LLM.APIKey, cfg.LLM.AccountID, cfg.LLM.GatewayID))
	}
	if client == nil {
		return nil, render.Fail(render.CodeModelError, "The Storymaker agent has no LLM configured.")
	}
	return &storymakerAgentRoute{
		cfg:     cfg,
		client:  client,
		compact: gameMakerRouteIsSmall(gameMakerRouteLimits(cfg)),
		// Never set cfg.LLM.UseNativeFunctions here: this is the policy the loop computes itself.
		text: !agent.BuildToolingPolicy(cfg, "").UseNativeFunctions,
	}, nil
}

// storymakerAgentNames are the catalog names a text-mode profile lists.
func storymakerAgentNames(cat *storymaker.Catalog) agent.StorymakerNames {
	return agent.StorymakerNames{
		Poses:        cat.Names("pose"),
		Gestures:     cat.Names("gesture"),
		Expressions:  cat.Names("expression"),
		Environments: cat.Names("environment"),
		Characters:   cat.Names("character"),
	}
}

// storymakerAgentToolLimit is the tool-call limit of one loop (decision 14).
func storymakerAgentToolLimit(phase string) int {
	switch phase {
	case storymaker.PhaseOutline:
		return 6
	case storymaker.PhaseCast:
		return 12
	case storymaker.PhaseFill:
		return 10
	case storymaker.PhaseCheck:
		return 20
	}
	return 30
}

// storymakerAgentTemperature is the sampling temperature of one phase.
func storymakerAgentTemperature(phase string) float64 {
	switch phase {
	case storymaker.PhaseOutline:
		return 0.6
	case storymaker.PhaseCheck:
		return 0.2
	}
	return 0.3
}

// storymakerAgentNeedsToolCall says whether a phase ends only through a tool call. Check and revise end on a text reply.
func storymakerAgentNeedsToolCall(phase string) bool {
	return phase == storymaker.PhaseOutline || phase == storymaker.PhaseCast || phase == storymaker.PhaseFill
}

// storymakerAgentLoop is one model loop of a job.
type storymakerAgentLoop struct {
	phase   string
	sceneID string
	// advice marks a check round that applies advice only. It sets no RunComplete, because a RunComplete
	// that is already true refuses every dispatch, and the advice needs a patch.
	advice bool
}

// storymakerAgentJob is the state of one running agent job.
type storymakerAgentJob struct {
	s       *Server
	rt      *storymakerRuntime
	job     render.Job
	payload storymakerAgentPayload
	h       *render.Handle
	route   *storymakerAgentRoute
	names   agent.StorymakerNames
	mem     *memory.SQLiteMemory
	logger  *slog.Logger

	studio         *storymaker.Studio
	deps           storymaker.StudioDeps
	studioJob      storymaker.StudioJob
	voiceAvailable bool

	modelError bool
	failure    string
	adviceDone bool
	roundsLeft int
	summary    string

	mu   sync.Mutex
	info storymaker.AgentJobInfo
}

// emit writes one job event. It is the only caller of the event log in this file.
func (j *storymakerAgentJob) emit(eventType string, fields map[string]any) {
	j.rt.agentLog.emit(j.job.ProjectID, j.job.ID, eventType, fields)
}

// update changes the agent block of the job document.
func (j *storymakerAgentJob) update(change func(info *storymaker.AgentJobInfo)) {
	j.mu.Lock()
	change(&j.info)
	info := j.info
	j.mu.Unlock()
	j.h.SetAgent(info)
}

// refreshProgress copies the scene counts and the story etag to the job document.
func (j *storymakerAgentJob) refreshProgress(ctx context.Context) {
	progress, err := j.studio.Progress(ctx)
	if err != nil {
		j.logger.Warn("[Storymaker] The agent progress could not be read", "job_id", j.job.ID, "error", err)
		return
	}
	j.update(func(info *storymaker.AgentJobInfo) {
		info.ScenesTotal, info.ScenesDone, info.StoryETag = progress.ScenesTotal, progress.ScenesDone, progress.ETag
	})
}

// storymakerStudioCall turns a dispatched call into the Studio's call. The Studio repairs the
// arguments; this adds only what the typed ToolCall fields took away from the flat map.
func storymakerStudioCall(tc agent.ToolCall) storymaker.StudioCall {
	args := make(map[string]any, len(tc.Params)+2)
	for key, value := range tc.Params {
		args[key] = value
	}
	// An operation sent as an argument is read before the typed fields.
	operation, _ := args["operation"].(string)
	operation = strings.TrimSpace(operation)
	if operation == "" {
		operation = strings.TrimSpace(tc.Operation)
		// The agent copies the op argument, a dry_run target, into Operation. That names no operation.
		if op, _ := args["op"].(string); operation != "" && strings.EqualFold(operation, strings.TrimSpace(op)) {
			operation = ""
		}
	}
	if operation == "" {
		operation = strings.TrimSpace(tc.ActionType)
	}
	for _, key := range []string{"action", "tool", "operation", "action_type", "_todo"} {
		delete(args, key)
	}
	if _, ok := args["query"]; !ok && tc.Query != "" {
		args["query"] = tc.Query
	}
	if _, ok := args["id"]; !ok && tc.ID != "" {
		args["id"] = tc.ID
	}
	return storymaker.StudioCall{Tool: tc.Action, Operation: operation, Args: args}
}

// noteToolResult records a tool result with closed values only, and refreshes the job after a saved write.
func (j *storymakerAgentJob) noteToolResult(ctx context.Context, tool string, result storymaker.StudioResult) {
	fields := map[string]any{"tool": tool, "operation": result.Operation, "status": result.Status}
	if result.SceneID != "" {
		fields["scene_id"] = result.SceneID
	}
	j.emit(agentEventTool, fields)
	if result.Status == "ok" {
		j.refreshProgress(ctx)
	}
}

// hooks closes over the job's Studio, so a model cannot choose which job a tool call reaches.
func (j *storymakerAgentJob) hooks() *agent.ExecutionHooks {
	return &agent.ExecutionHooks{
		HandleTool: func(ctx context.Context, tc agent.ToolCall) (string, bool) {
			if !strings.HasPrefix(tc.Action, "storymaker_") {
				return "", false
			}
			result := j.studio.Call(ctx, storymakerStudioCall(tc))
			j.noteToolResult(ctx, tc.Action, result)
			return "Tool Output: " + string(result.JSON), true
		},
		AfterResponse: func(usage openai.Usage) error {
			j.update(func(info *storymaker.AgentJobInfo) { info.Tokens += max(0, usage.TotalTokens) })
			return nil
		},
	}
}

// runConfig builds the isolated run of one loop.
func (j *storymakerAgentJob) runConfig(cfg *config.Config, profile *agent.PreparedPromptProfile, loop storymakerAgentLoop, limit int) agent.RunConfig {
	sessionID := "storymaker-" + j.job.ID + ":" + uid.New()
	runCfg := buildDesktopRunConfigForSession(j.s, cfg, j.route.client, sessionID, "storymaker")
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
	runCfg.UserIntent = storymakerAgentIntent(loop.phase)
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
	if !loop.advice {
		runCfg.RunComplete = j.studio.PhaseDone
	}
	runCfg.RecoveryNudge = storymakerRecoveryNudge(loop.phase)
	return runCfg
}

// completePacket adds what only the server knows: the request text and the budget of this loop.
// Seconds are rounded so that an unchanged situation keeps an unchanged packet.
func (j *storymakerAgentJob) completePacket(ctx context.Context, packet map[string]any, phase string, limit int) map[string]any {
	if packet == nil {
		packet = map[string]any{}
	}
	budget, _ := packet["budget"].(map[string]any)
	if budget == nil {
		budget = map[string]any{}
	}
	budget["tool_calls"] = limit
	if deadline, ok := ctx.Deadline(); ok {
		budget["seconds_left"] = max(0, int(time.Until(deadline).Seconds())/30*30)
	}
	packet["budget"] = budget
	switch phase {
	case storymaker.PhaseOutline:
		packet["request"] = j.payload.Topic
	case storymaker.PhaseRevise:
		packet["instruction"] = j.payload.Instruction
	}
	return packet
}

// runLoop runs one model loop and returns its final text. Prose never leaves through events: the
// broker withholds it, and the caller decides whether the text is released.
func (j *storymakerAgentJob) runLoop(ctx context.Context, loop storymakerAgentLoop) (string, error) {
	cfg := *j.route.cfg
	cfg.LLM.Temperature = storymakerAgentTemperature(loop.phase)
	profile, err := storymakerAgentProfile(loop.phase, j.route.compact, j.route.text, j.names)
	if err != nil {
		return "", err
	}
	limit := storymakerAgentToolLimit(loop.phase)
	packet, err := j.studio.Packet(ctx, j.route.compact)
	if err != nil {
		return "", err
	}
	// encoding/json escapes angle brackets, so data cannot close the wrapper.
	data, err := json.Marshal(j.completePacket(ctx, packet, loop.phase, limit))
	if err != nil {
		return "", err
	}
	runCfg := j.runConfig(&cfg, profile, loop, limit)
	req := openai.ChatCompletionRequest{
		Model: cfg.LLM.Model,
		Messages: []openai.ChatCompletionMessage{{
			Role:    openai.ChatMessageRoleUser,
			Content: storymakerAgentUserMessage(loop.phase, j.voiceAvailable, data),
		}},
		Stream: true,
	}
	if !j.route.text && storymakerAgentNeedsToolCall(loop.phase) {
		req.ToolChoice = "required"
	}
	if tokens := gameMakerOutputTokens(gameMakerRouteLimits(&cfg), llm.ConservativeOutputTokens); tokens > 0 {
		req.MaxTokens = tokens
	}
	callCtx := llm.WithStreamAttemptTimeout(ctx, time.Duration(cfg.CircuitBreaker.LLMTimeoutSeconds)*time.Second)
	response, err := agent.ExecuteAgentLoop(callCtx, req, runCfg, true, newStorymakerBroker(j.rt.agentLog, j.job.ProjectID, j.job.ID))
	if err != nil {
		if agent.IsToolLimitFinalResponseInvalid(err) && ctx.Err() == nil {
			// The extra call stays unexecuted, and what was saved counts.
			return "", nil
		}
		return "", err
	}
	answer := ""
	if len(response.Choices) > 0 {
		answer = strings.TrimSpace(response.Choices[0].Message.Content)
	}
	if answer == "[Empty Response]" {
		answer = ""
	}
	return answer, nil
}

// storymakerAgentIntent is the fixed server sentence of a phase. The request itself stays in the data block.
func storymakerAgentIntent(phase string) string {
	switch phase {
	case storymaker.PhaseOutline:
		return "Write the outline of this film."
	case storymaker.PhaseCast:
		return "Choose the title and the cast of this film."
	case storymaker.PhaseFill:
		return "Fill the scene named in the context."
	case storymaker.PhaseCheck:
		return "Fix what the context lists for this film."
	}
	return "Apply the instruction from the context to this film."
}

// storymakerAgentUserMessage is the only user message of a loop.
func storymakerAgentUserMessage(phase string, voiceAvailable bool, data []byte) string {
	text := storymakerAgentIntent(phase)
	if phase == storymaker.PhaseFill && !voiceAvailable {
		text += "\n" + storymakerNoVoiceNote
	}
	return text + "\n\nJob context (data):\n<external_data>\n" + string(data) + "\n</external_data>"
}

// storymakerAgentAttempts bounds one loop: it is restarted at most once, after a model error or an empty end.
const storymakerAgentAttempts = 2

// storymakerAgentStopped is the error of a run that lost its context. The manager turns a requested
// cancel or a shutdown into its own end state, whatever this returns.
func storymakerAgentStopped() *render.JobError {
	return render.Fail(render.CodeCancelled, "The desktop became read-only or its configuration changed, so the agent stopped.")
}

// storymakerAgentTargetSeconds clamps rather than refuses, so a chat call does not fail (decision 13).
func storymakerAgentTargetSeconds(cfg *config.Config, requested, saved int) int {
	target := requested
	if target <= 0 {
		target = saved
	}
	if target <= 0 {
		target = 60
	}
	upper := min(cfg.Storymaker.Agent.MaxLengthSeconds, cfg.Storymaker.Render.MaxDurationSeconds)
	lower := min(15, upper)
	return min(max(target, lower), upper)
}

func storymakerAgentRequestClip(text string) string {
	runes := []rune(security.Scrub(strings.TrimSpace(text)))
	if len(runes) > 200 {
		return string(runes[:200])
	}
	return string(runes)
}

func storymakerAgentSummary(text string) string {
	runes := []rune(security.Scrub(strings.TrimSpace(text)))
	if len(runes) > 2000 {
		return string(runes[:2000])
	}
	return string(runes)
}

func storymakerAgentSameVoice(a, b *storymaker.Voice) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func storymakerAgentVoiceReady(providers []storymaker.VoiceProvider, voice *storymaker.Voice) bool {
	if voice == nil {
		return false
	}
	id := storymaker.NormalizeProvider(voice.Provider)
	for _, row := range providers {
		if row.ID == id {
			return row.Ready
		}
	}
	return false
}

// storymakerAgentCompile runs the compile check with settings of its own: storymakerSettings fails
// without FFmpeg, and a compile check needs no FFmpeg.
func (s *Server) storymakerAgentCompile(rt *storymakerRuntime, cfg *config.Config) func(context.Context, string) (storymaker.CompileSummary, error) {
	settings := render.Settings{
		TimeoutSeconds:     cfg.Storymaker.Render.TimeoutSeconds,
		MaxDurationSeconds: cfg.Storymaker.Render.MaxDurationSeconds,
		BrowserPath:        cfg.Storymaker.Render.BrowserPath,
		Transport:          cfg.Storymaker.Render.Transport,
		Limits:             storymakerLimits(cfg),
	}
	return func(ctx context.Context, projectID string) (storymaker.CompileSummary, error) {
		return rt.runner.CompileCheck(ctx, projectID, settings)
	}
}

// prepareVoice is the first job step (decision 11). It writes story.voice and returns the language
// of the job and whether a ready voice speaks it. No voice is not a failure: the story keeps voice null.
func (j *storymakerAgentJob) prepareVoice(ctx context.Context) (string, bool, error) {
	cfg := j.route.cfg
	data, etag, err := j.rt.projects.LoadStory(ctx, j.job.ProjectID)
	if err != nil {
		return "", false, err
	}
	story, _, err := storymaker.DecodeStory(data, storymakerLimits(cfg))
	if err != nil {
		return "", false, err
	}
	revise := j.payload.Kind == storymaker.AgentKindRevise
	language := strings.TrimSpace(j.payload.Language)
	if language == "" && revise && story.Voice != nil {
		language = story.Voice.Language
	}
	if language == "" {
		language = strings.TrimSpace(cfg.Server.UILanguage)
	}
	if language == "" {
		language = "en"
	}
	providers := j.s.storymakerVoiceProviders(ctx)
	voice := story.Voice
	switch {
	case j.payload.Voice != nil:
		chosen := *j.payload.Voice
		if chosen.Language == "" {
			chosen.Language = language
		}
		voice = &chosen
	case voice == nil || (!revise && storymaker.LanguageBase(voice.Language) != storymaker.LanguageBase(language)):
		voice = nil
		if resolved, _, err := storymaker.ResolveProjectVoice(providers, language, storymakerDefaultVoiceFor(cfg, providers)); err == nil {
			voice = &resolved
		}
	}
	if !storymakerAgentSameVoice(story.Voice, voice) {
		story.Voice = voice
		storymaker.ApplyDefaults(&story)
		out, err := storymaker.CanonicalJSON(story)
		if err != nil {
			return "", false, err
		}
		if _, err := j.rt.projects.SaveStory(ctx, j.job.ProjectID, out, etag); err != nil {
			return "", false, err
		}
	}
	return language, storymakerAgentVoiceReady(providers, voice), nil
}

// newStorymakerAgentJob resolves the route and the voice and opens the session. Every error is a
// *render.JobError. The agent switch gates new jobs at the routes, not here.
func (s *Server) newStorymakerAgentJob(ctx context.Context, rt *storymakerRuntime, job render.Job, p storymakerAgentPayload, h *render.Handle) (*storymakerAgentJob, error) {
	route, err := s.storymakerAgentResolveRoute(p.Provider, p.Model)
	if err != nil {
		return nil, err
	}
	cfg := route.cfg
	if !cfg.Storymaker.Enabled || !cfg.VirtualDesktop.Enabled {
		return nil, render.Fail(render.CodeDisabled, "Storymaker was switched off before this agent job started.")
	}
	if p.Kind == "" {
		p.Kind = storymaker.AgentKindCreate
	}
	logger := s.storymakerMusicLogger()
	project, err := rt.projects.Get(ctx, job.ProjectID)
	if err != nil {
		logger.Warn("[Storymaker] The agent job could not read its project", "job_id", job.ID, "error", err)
		return nil, render.Fail(render.CodeModelError, "The project of this agent job could not be read.")
	}
	mem, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		logger.Warn("[Storymaker] The agent session could not be opened", "job_id", job.ID, "error", err)
		return nil, render.Fail(render.CodeModelError, "The agent session could not be opened.")
	}
	j := &storymakerAgentJob{s: s, rt: rt, job: job, payload: p, h: h, route: route, names: storymakerAgentNames(rt.catalog), mem: mem, logger: logger}
	language, available, err := j.prepareVoice(ctx)
	if err != nil {
		mem.Close()
		logger.Warn("[Storymaker] The agent job could not prepare the voice", "job_id", job.ID, "error", err)
		return nil, render.Fail(render.CodeModelError, "The story of this agent job could not be read.")
	}
	j.voiceAvailable = available
	j.deps = storymaker.StudioDeps{
		Projects:           rt.projects,
		Catalog:            rt.catalog,
		Limits:             storymakerLimits(cfg),
		Voices:             s.storymakerVoiceProviders,
		Compile:            s.storymakerAgentCompile(rt, cfg),
		MaxDurationSeconds: cfg.Storymaker.Render.MaxDurationSeconds,
		ReadOnly: func() bool {
			latest := s.ConfigSnapshot()
			return latest == nil || latest.VirtualDesktop.ReadOnly
		},
	}
	target := storymakerAgentTargetSeconds(cfg, p.LengthSeconds, project.TargetSeconds)
	j.studioJob = storymaker.StudioJob{
		JobID: job.ID, ProjectID: job.ProjectID, Kind: p.Kind,
		Language: language, Audience: firstNonEmpty(p.Audience, project.Audience),
		TargetSeconds: target, VoiceAvailable: available,
	}
	j.studio, err = storymaker.NewStudio(j.deps, j.studioJob)
	if err != nil {
		mem.Close()
		logger.Warn("[Storymaker] The agent Studio could not be opened", "job_id", job.ID, "error", err)
		return nil, render.Fail(render.CodeModelError, "The agent session could not be prepared.")
	}
	j.update(func(info *storymaker.AgentJobInfo) {
		info.Kind = p.Kind
		info.Provider, info.Model = cfg.LLM.Provider, cfg.LLM.Model
		info.Compact, info.TextMode = route.compact, route.text
		info.TargetSeconds = target
		info.Request = storymakerAgentRequestClip(firstNonEmpty(p.Topic, p.Instruction))
	})
	return j, nil
}

// noteModelError keeps a generic text for the job. The raw error stays in the server log, because
// provider errors can carry URLs and response bodies.
func (j *storymakerAgentJob) noteModelError(err error) {
	j.modelError = true
	j.failure = cydCreatorAgentFailureText(err)
}

// enterPhase binds the Studio to the phase and shows the phase on the job.
func (j *storymakerAgentJob) enterPhase(phase, sceneID string) {
	j.studio.SetPhase(phase, sceneID)
	j.h.SetPhase(phase)
	j.update(func(info *storymaker.AgentJobInfo) { info.Phase, info.SceneID = phase, sceneID })
}

// loopEnded says whether a loop that returned without an error reached the end of its phase.
func (j *storymakerAgentJob) loopEnded(loop storymakerAgentLoop, answer string) bool {
	if storymakerAgentNeedsToolCall(loop.phase) {
		return j.studio.PhaseDone()
	}
	return answer != "" || j.studio.PhaseDone()
}

// runPhase runs one loop with at most one restart and reports whether the phase reached its end. A
// failed loop does not fail the job: what was saved stays (decision 15). An advice round never
// counts as a model error, because advice never blocks the outcome.
func (j *storymakerAgentJob) runPhase(ctx context.Context, loop storymakerAgentLoop) (string, bool) {
	j.enterPhase(loop.phase, loop.sceneID)
	for attempt := 1; attempt <= storymakerAgentAttempts; attempt++ {
		fields := map[string]any{"phase": loop.phase, "attempt": attempt}
		if loop.sceneID != "" {
			fields["scene_id"] = loop.sceneID
		}
		j.emit(agentEventPhase, fields)
		answer, err := j.runLoop(ctx, loop)
		if ctx.Err() != nil {
			return "", false
		}
		if err == nil && j.loopEnded(loop, answer) {
			return answer, true
		}
		if err != nil {
			j.logger.Warn("[Storymaker] An agent loop failed", "job_id", j.job.ID, "phase", loop.phase, "attempt", attempt, "error", err)
			if attempt == storymakerAgentAttempts && !loop.advice {
				j.noteModelError(err)
			}
		}
	}
	return "", false
}

// check reads the report of this moment and publishes its counts.
func (j *storymakerAgentJob) check(ctx context.Context) (storymaker.CheckReport, error) {
	report, err := j.studio.Check(ctx)
	if err != nil {
		return storymaker.CheckReport{}, err
	}
	j.roundsLeft = report.RoundsLeft
	j.emitCheck(report)
	return report, nil
}

func (j *storymakerAgentJob) emitCheck(report storymaker.CheckReport) {
	j.emit(agentEventCheck, map[string]any{"issues": len(report.Issues), "advice": len(report.Advice), "rounds_left": report.RoundsLeft, "compile": report.Compile})
}

// keepAnswer remembers the last text reply. It is released only with the outcome ready_unverified.
func (j *storymakerAgentJob) keepAnswer(answer string) {
	if answer != "" {
		j.summary = answer
	}
}

// runCheckRound is the check step: one model loop, only if the report asks for one. Advice gets one chance per job.
func (j *storymakerAgentJob) runCheckRound(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	report, err := j.check(ctx)
	if err != nil {
		j.logger.Warn("[Storymaker] The agent check failed", "job_id", j.job.ID, "error", err)
		j.noteModelError(err)
		return
	}
	switch {
	case report.RoundsLeft <= 0:
	case len(report.Issues) > 0:
		answer, _ := j.runPhase(ctx, storymakerAgentLoop{phase: storymaker.PhaseCheck})
		j.keepAnswer(answer)
	case len(report.Advice) > 0 && !j.adviceDone:
		j.adviceDone = true
		answer, _ := j.runPhase(ctx, storymakerAgentLoop{phase: storymaker.PhaseCheck, advice: true})
		j.keepAnswer(answer)
	}
	if ctx.Err() == nil {
		j.refreshProgress(ctx)
	}
}

// runCreate is the create order: outline, cast, one loop per scene, then the check step.
func (j *storymakerAgentJob) runCreate(ctx context.Context) {
	if _, done := j.runPhase(ctx, storymakerAgentLoop{phase: storymaker.PhaseOutline}); !done {
		return
	}
	j.refreshProgress(ctx)
	if ctx.Err() != nil {
		return
	}
	if _, done := j.runPhase(ctx, storymakerAgentLoop{phase: storymaker.PhaseCast}); !done {
		return
	}
	if ctx.Err() != nil {
		return
	}
	progress, err := j.studio.Progress(ctx)
	if err != nil {
		j.logger.Warn("[Storymaker] The agent progress could not be read", "job_id", j.job.ID, "error", err)
		j.noteModelError(err)
		return
	}
	for _, sceneID := range progress.Unfilled {
		if ctx.Err() != nil {
			return
		}
		j.runPhase(ctx, storymakerAgentLoop{phase: storymaker.PhaseFill, sceneID: sceneID})
		j.refreshProgress(ctx)
	}
	j.runCheckRound(ctx)
}

// publishOutcome decides the outcome and writes it to the job. A story that never got an outline
// has nothing valid to keep, and only that ends the job as failed.
func (j *storymakerAgentJob) publishOutcome(ctx context.Context, in storymaker.OutcomeInput) (render.Output, error) {
	progress, err := j.studio.Progress(ctx)
	if err != nil {
		j.logger.Warn("[Storymaker] The agent progress could not be read", "job_id", j.job.ID, "error", err)
		j.noteModelError(err)
		in.ModelError = true
	}
	// The drafts come from the caller, which reads them before it drops them.
	progress.Drafts = in.Progress.Drafts
	in.Progress = progress
	outcome, checks := storymaker.DecideOutcome(in)
	j.emit(agentEventOutcome, map[string]any{"outcome": outcome})
	j.update(func(info *storymaker.AgentJobInfo) {
		info.Outcome, info.OpenChecks = outcome, checks
		info.Phase, info.SceneID = "", ""
		info.ScenesTotal, info.ScenesDone, info.StoryETag = progress.ScenesTotal, progress.ScenesDone, progress.ETag
		info.DurationSeconds = in.Check.TotalSeconds
		if outcome == storymaker.OutcomeReadyUnverified {
			info.Summary = storymakerAgentSummary(j.summary)
		}
	})
	if outcome == storymaker.OutcomeFailed {
		text := j.failure
		if text == "" {
			text = "The agent could not write an outline for this story."
		}
		return render.Output{}, render.Fail(render.CodeModelError, "%s", text)
	}
	return render.Output{}, nil
}

// runRevise is the revise order: the revise loop, then the same steps as create from the check on.
func (j *storymakerAgentJob) runRevise(ctx context.Context) {
	answer, _ := j.runPhase(ctx, storymakerAgentLoop{phase: storymaker.PhaseRevise})
	j.keepAnswer(answer)
	if ctx.Err() != nil {
		return
	}
	j.refreshProgress(ctx)
	j.runCheckRound(ctx)
}

// storymakerAgentNarrateWait bounds the wait for the narrate job (decision 20).
const storymakerAgentNarrateWait = 300 * time.Second

// Open checks of the narration that are not job error codes.
const (
	storymakerCheckNarrationBudget  = "narration_budget"
	storymakerCheckNarrationPending = "narration_pending"
)

// storymakerAgentRefusalCheck maps a refused narration to its open check.
func storymakerAgentRefusalCheck(code string) string {
	if code == render.CodeBudgetExceeded {
		return storymakerCheckNarrationBudget
	}
	return render.CodeNarrationUnavailable
}

// storymakerAgentNarrateJobCheck maps a finished narrate job to its open check, "" when it succeeded.
func storymakerAgentNarrateJobCheck(job render.Job) string {
	if job.Status == render.StatusSucceeded {
		return ""
	}
	if job.Error != nil {
		switch job.Error.Code {
		case render.CodeBudgetExceeded:
			return storymakerCheckNarrationBudget
		case render.CodeVoiceRequired, render.CodeNarrationUnavailable:
			return render.CodeNarrationUnavailable
		}
	}
	return render.CodeNarrationFailed
}

// waitNarration waits for the narrate job within the bound. A cancelled agent job takes its narrate
// job along, so nothing is billed after a cancel. A job that outlasts the bound keeps running and
// reads as pending.
func (j *storymakerAgentJob) waitNarration(ctx context.Context, jobID string, wait time.Duration) string {
	events, unsubscribe := j.rt.narrate.Subscribe(j.job.ProjectID)
	defer unsubscribe()
	timer := time.NewTimer(wait)
	defer timer.Stop()
	for {
		current, found := j.rt.narrate.Get(jobID)
		if !found {
			return render.CodeNarrationFailed
		}
		if current.Terminal() {
			return storymakerAgentNarrateJobCheck(current)
		}
		select {
		case <-ctx.Done():
			j.rt.narrate.Cancel(jobID)
			return storymakerCheckNarrationPending
		case <-timer.C:
			return storymakerCheckNarrationPending
		case <-events:
		}
	}
}

// narrate runs the narration once the story is settled, through the same refusal order as the route.
// The result never blocks the outcome: it returns an open check, or "". The audio cache key contains
// the text, so a second run bills changed scenes only.
func (j *storymakerAgentJob) narrate(runCtx, jobCtx context.Context) string {
	// A job past its deadline narrates nothing: a timed-out story is never sent to a paid voice.
	if jobCtx.Err() != nil {
		return storymakerCheckNarrationPending
	}
	j.h.SetPhase("narrating")
	j.update(func(info *storymaker.AgentJobInfo) { info.Phase, info.SceneID = "narrating", "" })
	j.emit(agentEventPhase, map[string]any{"phase": "narrating", "attempt": 1})
	id := j.job.ProjectID
	cfg := j.route.cfg
	// The reads of the final state keep working after a timeout. Only the wait below stops on a cancel.
	read := context.WithoutCancel(runCtx)
	data, _, err := j.rt.projects.LoadStory(read, id)
	if err != nil {
		j.logger.Warn("[Storymaker] The agent could not read the story for narration", "job_id", j.job.ID, "error", err)
		return render.CodeNarrationFailed
	}
	story, _, err := storymaker.DecodeStory(data, storymakerLimits(cfg))
	if err != nil {
		j.logger.Warn("[Storymaker] The agent could not decode the story for narration", "job_id", j.job.ID, "error", err)
		return render.CodeNarrationFailed
	}
	storymaker.ApplyDefaults(&story)
	answer, err := j.rt.projects.AudioAnswer(read, id, &story, nil)
	if err != nil {
		j.logger.Warn("[Storymaker] The agent could not read the narration state", "job_id", j.job.ID, "error", err)
		return render.CodeNarrationFailed
	}
	if len(answer.Missing) == 0 {
		return ""
	}
	acc := &storymakerAccess{cfg: cfg, rt: j.rt}
	if refusal := storymakerNarrateRefusal(read, j.s, acc, storymakerBlocksToSynthesize(&story, answer), answer); refusal != nil {
		return storymakerAgentRefusalCheck(refusal.Code)
	}
	queued, _, err := j.rt.narrate.Enqueue(render.EnqueueRequest{ProjectID: id, Kind: render.KindNarrate, Coalesce: true, Payload: storymakerNarratePayload{ProjectID: id}})
	switch {
	case errors.Is(err, render.ErrBusy):
		return storymakerCheckNarrationPending
	case err != nil:
		j.logger.Warn("[Storymaker] The agent could not queue the narration", "job_id", j.job.ID, "error", err)
		return render.CodeNarrationFailed
	}
	wait := storymakerAgentNarrateWait
	if deadline, ok := jobCtx.Deadline(); ok {
		if left := time.Until(deadline); left < wait {
			wait = left
		}
	}
	return j.waitNarration(runCtx, queued.ID, wait)
}

// finalCheck reads the report after narration with a second Studio. The compile cache of the job's
// Studio is keyed by the story etag, and narration changes the audio, not the story.
func (j *storymakerAgentJob) finalCheck(ctx context.Context) (storymaker.CheckReport, error) {
	fresh, err := storymaker.NewStudio(j.deps, j.studioJob)
	if err != nil {
		return storymaker.CheckReport{}, err
	}
	report, err := fresh.Check(ctx)
	if err != nil {
		return storymaker.CheckReport{}, err
	}
	report.RoundsLeft = j.roundsLeft
	j.emitCheck(report)
	return report, nil
}

// readStory reads the saved story. It returns nil when the story cannot be read or decoded.
func (j *storymakerAgentJob) readStory(ctx context.Context) *storymaker.Story {
	data, _, err := j.rt.projects.LoadStory(ctx, j.job.ProjectID)
	if err != nil {
		return nil
	}
	story, _, err := storymaker.DecodeStory(data, storymakerLimits(j.route.cfg))
	if err != nil {
		return nil
	}
	return &story
}

// finish narrates, runs the final compile and publishes the outcome. Real audio lengths can produce a
// model-fixable issue after the last fix round, so that case gets one more check loop and narrates again.
// A story that still has no outline ends here as failed. The final state is read through a detached
// context, so an outcome is still reached after a timeout.
func (j *storymakerAgentJob) finish(runCtx, jobCtx context.Context) (render.Output, error) {
	final := context.WithoutCancel(runCtx)
	// The drafts are read before they are dropped, so a draft that was never saved stays an open check.
	open, _ := j.studio.Progress(final)
	j.studio.DropDrafts()
	if story := j.readStory(final); story != nil && storymaker.StoryIsBlank(story) {
		return j.publishOutcome(final, storymaker.OutcomeInput{Progress: open, TimedOut: jobCtx.Err() == context.DeadlineExceeded, ModelError: j.modelError})
	}
	narration := j.narrate(runCtx, jobCtx)
	report, err := j.finalCheck(final)
	if err == nil && jobCtx.Err() == nil && len(report.Issues) > 0 && report.RoundsLeft > 0 {
		answer, _ := j.runPhase(jobCtx, storymakerAgentLoop{phase: storymaker.PhaseCheck})
		j.keepAnswer(answer)
		open, _ = j.studio.Progress(final)
		j.studio.DropDrafts()
		narration = j.narrate(runCtx, jobCtx)
		report, err = j.finalCheck(final)
	}
	if runCtx.Err() == context.Canceled {
		return render.Output{}, storymakerAgentStopped()
	}
	if err != nil {
		j.logger.Warn("[Storymaker] The final agent check failed", "job_id", j.job.ID, "error", err)
		j.noteModelError(err)
	}
	return j.publishOutcome(final, storymaker.OutcomeInput{
		Progress:   open,
		Check:      report,
		TimedOut:   jobCtx.Err() == context.DeadlineExceeded,
		ModelError: j.modelError,
		Narration:  narration,
	})
}

// storymakerAgentRun runs one job of the agent queue. It gets rt directly, because
// s.storymakerRuntime() holds storymakerMu while a replaced runtime waits for its running job. The
// desktop run makes a config change that sets read-only cancel the loops already in flight.
func (s *Server) storymakerAgentRun(ctx context.Context, rt *storymakerRuntime, job render.Job, payload any, h *render.Handle) (render.Output, error) {
	p, ok := payload.(storymakerAgentPayload)
	if !ok {
		return render.Output{}, render.Fail(render.CodeModelError, "This agent job has no request.")
	}
	runCtx, release, err := s.beginDesktopRun(ctx)
	if err != nil {
		return render.Output{}, storymakerAgentStopped()
	}
	defer release()
	// The desktop service can be replaced while a job waits. Requests refresh the store the same way.
	if svc, _, err := s.getDesktopService(runCtx); err == nil {
		rt.store.use(svc)
	}
	j, err := s.newStorymakerAgentJob(runCtx, rt, job, p, h)
	if err != nil {
		if runCtx.Err() == context.Canceled {
			return render.Output{}, storymakerAgentStopped()
		}
		return render.Output{}, err
	}
	defer j.mem.Close()
	jobCtx, cancel := context.WithTimeout(runCtx, time.Duration(j.route.cfg.Storymaker.Agent.JobTimeoutSeconds)*time.Second)
	defer cancel()
	if j.payload.Kind == storymaker.AgentKindRevise {
		j.runRevise(jobCtx)
	} else {
		j.runCreate(jobCtx)
	}
	if runCtx.Err() == context.Canceled {
		j.studio.DropDrafts()
		return render.Output{}, storymakerAgentStopped()
	}
	return j.finish(runCtx, jobCtx)
}
