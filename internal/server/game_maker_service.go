package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"sync"
	"time"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/gamemaker"
	"aurago/internal/llm"
	"aurago/internal/security"
	"aurago/internal/tools"
	"aurago/internal/uid"

	"github.com/sashabaranov/go-openai"
)

func gameMakerPolicy(cfg config.GameMakerConfig, desktopReadonly bool) gamemaker.Policy {
	return gamemaker.Policy{
		Enabled:              cfg.Enabled,
		ReadOnly:             cfg.ReadOnly || desktopReadonly,
		AllowCreate:          cfg.AllowCreate,
		AllowEdit:            cfg.AllowEdit,
		AllowDelete:          cfg.AllowDelete,
		AllowMediaGeneration: cfg.AllowMediaGeneration,
	}
}

func gameMakerRuntimeConfigChanged(oldCfg, newCfg config.GameMakerConfig) bool {
	return oldCfg.WorkspacePath != newCfg.WorkspacePath ||
		oldCfg.MaxProjects != newCfg.MaxProjects ||
		oldCfg.MaxFilesPerProject != newCfg.MaxFilesPerProject ||
		oldCfg.MaxFileSizeKB != newCfg.MaxFileSizeKB ||
		oldCfg.MaxAssetSizeMB != newCfg.MaxAssetSizeMB ||
		oldCfg.MaxProjectSizeMB != newCfg.MaxProjectSizeMB ||
		oldCfg.JobTimeoutSeconds != newCfg.JobTimeoutSeconds
}

func (s *Server) initGameMaker() {
	if s == nil || s.Cfg == nil {
		return
	}
	cfg := s.Cfg.GameMaker
	service, err := gamemaker.NewService(gamemaker.Options{
		DBPath:               s.Cfg.SQLite.GameMakerPath,
		WorkspacePath:        cfg.WorkspacePath,
		Enabled:              cfg.Enabled,
		ReadOnly:             cfg.ReadOnly || s.Cfg.VirtualDesktop.ReadOnly,
		AllowCreate:          cfg.AllowCreate,
		AllowEdit:            cfg.AllowEdit,
		AllowDelete:          cfg.AllowDelete,
		AllowMediaGeneration: cfg.AllowMediaGeneration,
		MaxProjects:          cfg.MaxProjects,
		MaxFilesPerProject:   cfg.MaxFilesPerProject,
		MaxFileBytes:         int64(cfg.MaxFileSizeKB) * 1024,
		MaxAssetBytes:        int64(cfg.MaxAssetSizeMB) * 1024 * 1024,
		MaxProjectBytes:      int64(cfg.MaxProjectSizeMB) * 1024 * 1024,
		JobTimeout:           time.Duration(cfg.JobTimeoutSeconds) * time.Second,
		Logger:               s.Logger,
	})
	if err != nil {
		if s.Logger != nil {
			s.Logger.Error("Failed to initialize Game Maker service", "error", err)
		}
		return
	}
	service.SetSkillStatus(s.gameMakerSkills, s.gameMakerSkillsReady)
	service.UpdatePolicy(gameMakerPolicy(cfg, s.Cfg.VirtualDesktop.ReadOnly))
	service.SetRunner(&gameMakerAgentRunner{server: s, service: service})
	s.GameMaker = service
	gamemaker.SetDefaultService(service)
	if s.Logger != nil {
		s.Logger.Info("Game Maker service initialized",
			"enabled", cfg.Enabled,
			"readonly", cfg.ReadOnly,
			"skills_ready", s.gameMakerSkillsReady,
		)
	}
}

func verifyGameMakerAgentSkills(ctx context.Context, manager *tools.AgentSkillManager, install gamemaker.SkillInstallResult, logger *slog.Logger) ([]gamemaker.SkillInfo, bool) {
	ready := install.Ready && manager != nil
	skills := append([]gamemaker.SkillInfo(nil), install.Skills...)
	if len(skills) != len(gamemaker.CuratedSkillNames()) {
		ready = false
	}
	seen := map[string]bool{}
	for i := range skills {
		name := skills[i].Name
		if skills[i].Status == "hash_mismatch" || manager == nil || ctx.Err() != nil || seen[name] {
			ready = false
			continue
		}
		seen[name] = true
		markdown, err := gamemaker.BundledSkillMarkdown(name)
		if err == nil {
			_, err = manager.RegisterBundledAgentSkill(ctx, name, markdown)
		}
		if err != nil {
			skills[i].Status = "verification_failed"
			ready = false
			if logger != nil {
				logger.Warn("Bundled Game Maker Agent Skill verification failed", "name", name, "error", err)
			}
			continue
		}
		skills[i].Status = "ready"
	}
	return skills, ready
}

type gameMakerAgentRunner struct {
	server     *Server
	service    *gamemaker.Service
	usageMu    sync.Mutex
	usageJobID string
	usage      *agent.PromptUsageObserver
}

// gameMakerBudget tells the agent how much room this round has. Values are
// rounded so that an unchanged situation keeps an unchanged context packet.
func gameMakerBudget(ctx context.Context, toolCalls int, run gamemaker.JobRun) map[string]any {
	budget := map[string]any{"tool_calls": toolCalls}
	if deadline, ok := ctx.Deadline(); ok {
		remaining := int(time.Until(deadline).Seconds())
		budget["seconds_remaining"] = max(0, remaining/30*30)
	}
	return budget
}

// Reasoning models spend output tokens before their answer, and providers count
// both against max_tokens. Qwen 3.8 drafted whole games in its reasoning and hit
// the 4096-token general reserve before any tool call. The answer keeps its own
// reserve and reasoning gets this separate allowance on top.
const (
	gameMakerBuildAnswerTokens = 16384
	gameMakerReasoningTokens   = 16384
	// MiniMax-M3.1-Flash reasoned 23-33K tokens before writing main.ts; the 16K
	// allowance cut 9 of 10 building replies on aurago-test (2026-11-08/09).
	gameMakerWideReasoningTokens = 49152
)

// gameMakerStreamThroughput lets a slow model that keeps streaming outlast the
// fixed per-call timeout. The LLM timeout stays the base until output is
// measured; then the deadline follows the time the output reserve needs at
// the measured rate, up to four times the base and never past the job.
// Streams under three tokens per second look stalled and get no extension.
var gameMakerStreamThroughput = llm.ThroughputPolicy{MinTokensPerSecond: 3, MaxFactor: 4, MinSample: 5 * time.Second}

// gameMakerStreamIdleSeconds is how long a Game Maker stream may stay silent
// once it has shown progress. Some gateways send nothing while the model writes
// a large tool-call argument such as a complete source file. The first token
// keeps the whole call budget, and a larger configured
// circuit_breaker.llm_stream_chunk_timeout_seconds is kept.
const gameMakerStreamIdleSeconds = 120

// applyGameMakerStreamIdle raises the stream idle window of a job's config copy
// to gameMakerStreamIdleSeconds and never lowers a larger configured value.
func applyGameMakerStreamIdle(cfg *config.Config) {
	if cfg.CircuitBreaker.LLMStreamChunkTimeoutSeconds < gameMakerStreamIdleSeconds {
		cfg.CircuitBreaker.LLMStreamChunkTimeoutSeconds = gameMakerStreamIdleSeconds
	}
}

// gameMakerReasoningAllowance returns the reasoning tokens granted on top of an
// answer reserve. Building reserves get the wide allowance when the route's
// context window holds at least four times the resulting reserve, so the
// history keeps most of the window. Planning and smaller routes keep the
// standard allowance.
func gameMakerReasoningAllowance(limits llm.ModelLimits, answer int) int {
	if answer >= gameMakerBuildAnswerTokens && limits.ContextWindow >= 4*(answer+gameMakerWideReasoningTokens) {
		return gameMakerWideReasoningTokens
	}
	return gameMakerReasoningTokens
}

// gameMakerOutputTokens returns the answer reserve plus the reasoning allowance
// (wide for building on large routes), clamped to the route's output limit.
// Routes below 64K context or without room beyond the general reasoning reserve
// return 0 and keep the agent's reserve.
func gameMakerOutputTokens(limits llm.ModelLimits, answer int) int {
	if limits.ContextWindow < 65536 || limits.MaxOutputTokens <= llm.ReasoningOutputTokens {
		return 0
	}
	return min(answer+gameMakerReasoningAllowance(limits, answer), limits.MaxOutputTokens)
}

// gameMakerRouteLimits resolves the selected route with the configured provider
// overrides, the same way request fitting will.
func gameMakerRouteLimits(cfg *config.Config) llm.ModelLimits {
	route := llm.ModelRoute{ProviderID: cfg.LLM.Provider, ProviderType: cfg.LLM.ProviderType, BaseURL: cfg.LLM.BaseURL, Model: cfg.LLM.Model, Primary: true}
	if provider := cfg.FindProvider(cfg.LLM.Provider); provider != nil && provider.Model == cfg.LLM.Model && strings.EqualFold(provider.Type, cfg.LLM.ProviderType) && strings.TrimRight(provider.BaseURL, "/") == strings.TrimRight(cfg.LLM.BaseURL, "/") {
		route.ContextWindowOverride, route.MaxOutputTokensOverride = provider.ContextWindow, provider.MaxOutputTokens
	}
	return llm.ResolveModelLimitsCached(route, cfg.Agent.ContextWindow)
}

func gameMakerToolCallLimit(systemLimit int) int {
	base := max(0, systemLimit)
	bonus := base / 4
	if base%4 != 0 {
		bonus++
	}
	return max(40, base+min(bonus, math.MaxInt-base))
}

func (r *gameMakerAgentRunner) RunGameMakerJob(ctx context.Context, run gamemaker.JobRun) error {
	s := r.server
	if s == nil || s.Cfg == nil || s.LLMClient == nil {
		return fmt.Errorf("Game Maker LLM is not configured")
	}
	ctx, release, err := s.beginDesktopRun(ctx)
	if err != nil {
		return err
	}
	defer release()
	s.CfgMu.RLock()
	cfg := *s.Cfg
	s.CfgMu.RUnlock()

	client := s.LLMClient
	if providerID := strings.TrimSpace(run.Job.ProviderID); providerID != "" {
		provider := cfg.FindProvider(providerID)
		if provider == nil {
			return fmt.Errorf("selected Game Maker provider %q is not configured", providerID)
		}
		if !config.ProviderUsageAllowsGeneralChat(provider) {
			return fmt.Errorf("selected Game Maker provider is not available for general chat")
		}
		cfg = *llm.TaskProviderConfig(&cfg, *provider)
	}
	if model := strings.TrimSpace(run.Job.Model); model != "" {
		cfg.LLM.Model = model
	}
	if strings.TrimSpace(cfg.LLM.Model) == "" {
		return fmt.Errorf("selected Game Maker model is empty")
	}
	applyGameMakerStreamIdle(&cfg)
	if strings.TrimSpace(run.Job.ProviderID) != "" || strings.TrimSpace(run.Job.Model) != "" {
		// The global failover client overwrites request.Model with its system route.
		cfg.FallbackLLM.Enabled = false
		client = llm.WrapOpenAIClient(llm.NewClientFromProviderWithConfig(&cfg, cfg.LLM.ProviderType, cfg.LLM.BaseURL, cfg.LLM.APIKey, cfg.LLM.AccountID, cfg.LLM.GatewayID))
	}
	profile, err := gameMakerPromptProfile("building", run.Project.Dimension)
	if err != nil {
		return err
	}
	files, err := r.service.ListJobFiles(ctx, run.Job.ID)
	if err != nil {
		return err
	}
	contextData := map[string]any{
		"project":               run.Project,
		"files":                 files,
		"diagnostics":           run.Diagnostics,
		"selected_model_assets": run.ModelAssetIDs,
		"selected_assets":       run.AssetSelections,
		"presentation":          run.Presentation,
		"capabilities": map[string]any{
			"sfx_generation_ready":     true,
			"media_generation_allowed": cfg.GameMaker.AllowMediaGeneration,
			"image_generation_ready":   cfg.GameMaker.AllowMediaGeneration && run.Project.UseImageGeneration && cfg.ImageGeneration.Enabled && cfg.ImageGeneration.APIKey != "",
			"music_generation_ready":   cfg.GameMaker.AllowMediaGeneration && run.Project.UseMusicGeneration && cfg.MusicConfigured(),
			"available_templates":      gamemaker.StudioTemplateNames(),
			"available_libraries":      []string{"phaser", "three", "voxel", "effects-2d", "effects-3d", "zzfx", "all"},
			"available_topics":         gamemaker.CraftRecipeTopics(),
		},
	}
	if lang := gameMakerReplyLanguage(&cfg); lang != "" {
		contextData["reply_language"] = lang
	}
	requests, err := r.service.PreviousJobRequests(ctx, run.Job.ID)
	if err != nil {
		return err
	}
	contextData["previous_user_requests"] = gameMakerPreviousRequests(requests)
	contextData["working_copy_restored"] = run.Job.ResumeFrom != ""
	contextData["workspace_changes"] = run.WorkspaceDrift
	history, checkpoint, err := r.gameConversation(ctx, &cfg, run, true)
	if err != nil {
		return err
	}
	sessionID := "game-maker-" + run.Job.ID + ":" + uid.New()
	runCfg := buildDesktopRunConfigForSession(s, &cfg, client, sessionID, "game_maker")
	if cfg.Learnings.Enabled || cfg.CodeGraph.Enabled {
		root, err := r.service.JobDirectory(run.Job.ID)
		if err != nil {
			return fmt.Errorf("resolve project knowledge: %w", err)
		}
		defer s.configureGameKnowledge(&runCfg, run.Project.ID, run.Job.ID, root)()
		if cfg.CodeGraph.Enabled {
			profile, err = agent.WithCodeGraphProfile(profile)
			if err != nil {
				return err
			}
		}
	}
	runCfg.TrustedPromptAddenda = nil
	runCfg.PreparedPrompt = profile
	runCfg.UsageObserver = r.gameUsageObserver(run)
	runCfg.NativeToolSchemas = profile.Tools()
	runCfg.AllowedTools = nil
	for _, definition := range runCfg.NativeToolSchemas {
		runCfg.AllowedTools = append(runCfg.AllowedTools, definition.Function.Name)
	}
	runCfg.UserIntent = gameMakerUserIntent(run)
	runCfg.Checkpoint = checkpoint
	runCfg.PreserveReasoning = true
	runCfg.StableSystemPrompt = true
	runCfg.RequireCompleteStream = true
	runCfg.RetryStreamIdle = true
	runCfg.StreamThroughput = &gameMakerStreamThroughput
	runCfg.ToolCallLimit = cfg.CircuitBreaker.MaxToolCalls
	runCfg.AllowedAgentSkills = gamemaker.CuratedSkillNames()
	runCfg.SuppressTurnSideEffects = true
	runCfg.RunComplete = func() bool { return r.service.JobFinished(run.Job.ID) }
	r.attachGamePreviewImages(&runCfg, &cfg, run.Job.ID)
	runCfg.IsMission = true
	runCfg.VoiceOutputActive = false

	contextData["budget"] = gameMakerBudget(ctx, runCfg.ToolCallLimit, run)
	slog.Info("game maker job starting", "job_id", run.Job.ID, "project_id", run.Project.ID,
		"provider_id", cfg.LLM.Provider, "provider_type", cfg.LLM.ProviderType, "model", cfg.LLM.Model, "tool_limit", runCfg.ToolCallLimit)
	data, _ := json.Marshal(contextData)
	req := openai.ChatCompletionRequest{
		Model: cfg.LLM.Model,
		Messages: []openai.ChatCompletionMessage{{
			Role:    openai.ChatMessageRoleUser,
			Content: gameMakerUserIntent(run) + "\n\nJob context (data):\n" + security.IsolateExternalData(string(data)),
		}},
		Stream: true,
	}
	// The agent writes whole source files inside tool-call arguments, and reasoning
	// models spend output tokens before them. Without an explicit reserve the agent
	// loop falls back to its generic 4096 (8192 on reasoning routes) and cuts every
	// large reply; this is what failed all jobs on aurago-test after the free-agent
	// rewrite dropped the reserve. Small routes still keep the generic reserve.
	if tokens := gameMakerOutputTokens(gameMakerRouteLimits(&cfg), gameMakerBuildAnswerTokens); tokens > 0 {
		req.MaxTokens = tokens
	}
	broker := &gameMakerBroker{service: r.service, projectID: run.Project.ID, jobID: run.Job.ID}
	req.Messages = append(history, req.Messages...)
	defer func() {
		if s.ShortTermMem != nil {
			if err := s.ShortTermMem.PurgeChatSession(sessionID); err != nil && s.Logger != nil {
				s.Logger.Warn("Failed to purge transient Game Maker agent session", "session_id", sessionID, "error", err)
			}
		}
	}()
	callCtx := llm.WithStreamAttemptTimeout(gamemaker.WithJobContext(ctx, run.Job.ID), time.Duration(cfg.CircuitBreaker.LLMTimeoutSeconds)*time.Second)
	response, err := agent.ExecuteAgentLoop(callCtx, req, runCfg, true, broker)
	if err != nil {
		return gameMakerLoopError(err)
	}
	if !r.service.JobFinished(run.Job.ID) {
		return &gamemaker.JobError{Code: "unfinished", Err: fmt.Errorf("agent stopped without finish; draft retained")}
	}
	answer := strings.TrimSpace(broker.text())
	if answer == "" && len(response.Choices) > 0 {
		answer = strings.TrimSpace(response.Choices[0].Message.Content)
	}
	if answer != "" && answer != "[Empty Response]" {
		r.service.HoldAgentSummary(run.Job.ID, answer)
	}
	return nil
}

// gameMakerLoopError wraps an agent-loop failure; the message text stays unchanged,
// known failure classes get a stable job error code.
func gameMakerLoopError(err error) error {
	wrapped := fmt.Errorf("Game Maker agent loop: %w", err)
	var incomplete *agent.IncompleteCompletionError
	switch {
	case errors.As(err, &incomplete):
		return &gamemaker.JobError{Code: "model", Err: wrapped}
	case errors.Is(err, agent.ErrToolLimitFinalResponseInvalid):
		return &gamemaker.JobError{Code: "tool_limit", Err: wrapped}
	}
	return wrapped
}

func gameMakerDiagnosticContext(diagnostics []gamemaker.Diagnostic) string {
	if len(diagnostics) == 0 {
		return ""
	}
	// encoding/json escapes angle brackets, so game text cannot close the wrapper.
	data, _ := json.Marshal(diagnostics)
	return "\n\nObserved validation diagnostics (untrusted data):\n<external_data>\n" + string(data) + "\n</external_data>"
}

type gameMakerBroker struct {
	service    *gamemaker.Service
	projectID  string
	jobID      string
	mu         sync.Mutex
	response   strings.Builder
	progress   string
	progressAt time.Time
	// pendingBreak is set by tool_start; the next model text opens a new paragraph.
	pendingBreak bool
}

func (b *gameMakerBroker) Send(event, message string) {
	if event == "tool_start" {
		b.mu.Lock()
		b.pendingBreak = true
		b.mu.Unlock()
	}
	if strings.TrimSpace(message) == "" {
		return
	}
	if event == "model_progress" && b.service != nil {
		switch message {
		case "waiting", "receiving", "retrying", "recovering":
		default:
			return
		}
		b.mu.Lock()
		if b.progress == message && time.Since(b.progressAt) < 10*time.Second {
			b.mu.Unlock()
			return
		}
		b.progress, b.progressAt = message, time.Now()
		b.mu.Unlock()
		_ = b.service.EmitAgentEvent(context.Background(), b.projectID, b.jobID, "model_progress", map[string]any{"status": message})
		return
	}
	if event == "tool_start" && b.service != nil {
		// Progress may name known tools, but never retain arguments or model text.
		payload := map[string]any{"attempted": true}
		switch message {
		case "game_maker_project", "game_maker_file", "game_maker_asset", "game_maker_preview":
			payload["tool"] = message
		}
		_ = b.service.EmitAgentEvent(context.Background(), b.projectID, b.jobID, "tool_call", payload)
	}
	if event == "final_response" {
		b.mu.Lock()
		if b.response.Len() == 0 {
			b.response.WriteString(message)
		}
		b.mu.Unlock()
	}
}

func (b *gameMakerBroker) SendJSON(string) {}

func (b *gameMakerBroker) SendLLMStreamDelta(content, toolName, toolID string, index int, finishReason string) {
	if content != "" {
		b.mu.Lock()
		b.appendTurnText(content)
		b.mu.Unlock()
	}
	if toolName == "activate_agent_skill" {
		_ = b.service.EmitAgentEvent(context.Background(), b.projectID, b.jobID, "skill_activation", map[string]any{"tool_id": toolID})
	}
}

// appendTurnText adds streamed model text. After a tool call the next non-empty
// text starts a new paragraph, so separate model turns do not run together.
// Caller holds b.mu.
func (b *gameMakerBroker) appendTurnText(content string) {
	if b.pendingBreak {
		if strings.TrimSpace(content) == "" {
			b.response.WriteString(content)
			return
		}
		b.pendingBreak = false
		if b.response.Len() > 0 {
			existing := b.response.String()
			trailing := len(existing) - len(strings.TrimRight(existing, "\n"))
			leading := len(content) - len(strings.TrimLeft(content, "\n"))
			if need := 2 - trailing - leading; need > 0 {
				b.response.WriteString(strings.Repeat("\n", need))
			}
		}
	}
	b.response.WriteString(content)
}

func (b *gameMakerBroker) SendLLMStreamDone(string) {}

const gameMakerTokenMetricMax = 10_000_000

func boundedGameMakerTokenMetric(value int) int {
	if value < 0 {
		return 0
	}
	if value > gameMakerTokenMetricMax {
		return gameMakerTokenMetricMax
	}
	return value
}

func gameMakerTokenSource(source string) string {
	switch source {
	case "provider_usage", "fallback_estimate":
		return source
	default:
		return "unknown"
	}
}

func (b *gameMakerBroker) SendTokenUpdate(prompt, completion, total, _, _ int, estimated, _ bool, source string) {
	if b == nil || b.service == nil {
		return
	}
	_ = b.service.EmitAgentEvent(context.Background(), b.projectID, b.jobID, "token_usage", map[string]any{
		"prompt_tokens":     boundedGameMakerTokenMetric(prompt),
		"completion_tokens": boundedGameMakerTokenMetric(completion),
		"total_tokens":      boundedGameMakerTokenMetric(total),
		"estimated":         estimated,
		"token_source":      gameMakerTokenSource(source),
	})
}
func (b *gameMakerBroker) SendThinkingBlock(string, string, string) {}

func (b *gameMakerBroker) text() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.response.String()
}

// gameMakerReplyLanguage returns the configured agent language for user-facing text.
func gameMakerReplyLanguage(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	return strings.TrimSpace(cfg.Agent.SystemLanguage)
}
