package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"time"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/memory"
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
