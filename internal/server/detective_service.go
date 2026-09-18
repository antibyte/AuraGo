package server

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/detective"
	"aurago/internal/llm"
	"aurago/internal/memory"
	"aurago/internal/prompts"
	"aurago/internal/security"
	"aurago/internal/tools"

	openai "github.com/sashabaranov/go-openai"
)

func (s *Server) initDetective() {
	if s.Cfg == nil || !s.Cfg.VirtualDesktop.Enabled {
		return
	}
	profiles := map[string]detective.Profile{}
	for k, v := range s.Cfg.Detective.Profiles {
		base := detective.Profiles()[k]
		if v.Seconds == 0 {
			v.Seconds = base.Seconds
		}
		if v.Tools == 0 {
			v.Tools = base.Tools
		}
		if v.Iterations == 0 {
			v.Iterations = base.Iterations
		}
		profiles[k] = detective.Profile{Seconds: v.Seconds, Tools: v.Tools, Iterations: v.Iterations, Tokens: v.Tokens}
	}
	path := filepath.Join(filepath.Dir(s.Cfg.SQLite.GameMakerPath), "detective.db")
	if s.Cfg.SQLite.GameMakerPath == "" {
		path = "data/detective.db"
	}
	service, err := detective.New(detective.Options{Path: path, Profiles: profiles, PDF: func(ctx context.Context, report detective.Report) ([]byte, error) {
		cfg := s.ConfigSnapshot()
		if cfg != nil && cfg.Tools.DocumentCreator.Enabled && cfg.Tools.DocumentCreator.Backend == "gotenberg" {
			return tools.RenderReportHTMLPDF(ctx, &cfg.Tools.DocumentCreator.Gotenberg, detective.ReportHTML(report))
		}
		a, err := detective.Export(report, "pdf")
		return a.Data, err
	}})
	if err != nil {
		s.Logger.Error("Detective initialization failed", "error", err)
		return
	}
	s.Detective = service
	if err = s.installDetectiveSkill(); err != nil {
		s.Logger.Error("Detective skill verification failed", "error", err)
		return
	}
	service.SetRunner(&detectiveRunner{server: s})
}

func (s *Server) installDetectiveSkill() error {
	if s.AgentSkillManager == nil {
		return errors.New("Agent Skills manager unavailable")
	}
	dir := filepath.Join(s.Cfg.Directories.AgentSkillsDir, "aurago-detective")
	path := filepath.Join(dir, "SKILL.md")
	existing, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if len(existing) > 0 && string(existing) != detective.Skill {
		entry, lookupErr := s.AgentSkillManager.GetAgentSkillByName("aurago-detective")
		pkg, parseErr := tools.ParseAgentSkillPackage(dir)
		if lookupErr != nil || parseErr != nil || entry == nil || entry.Origin != tools.OriginSystem || entry.CreatedBy != "system:detective" || entry.SecurityStatus != tools.SecurityClean || entry.PackageHash != pkg.PackageHash {
			return errors.New("installed Detective skill has unverified local changes")
		}
		if err = config.WriteFileAtomic(path, []byte(detective.Skill), 0600); err != nil {
			return err
		}
	}
	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if len(existing) == 0 {
		if err = config.WriteFileAtomic(path, []byte(detective.Skill), 0600); err != nil {
			return err
		}
	}
	_, err = s.AgentSkillManager.RegisterBundledAgentSkillFor(context.Background(), "detective", "aurago-detective", []byte(detective.Skill))
	return err
}

type detectiveRunner struct{ server *Server }

func detectiveSchema() openai.Tool {
	return openai.Tool{Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{Name: "detective_report", Description: "Persist the research plan, evidence-backed findings, ask a necessary question, or publish the report. The embedded Detective skill gives the exact JSON content shape.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"operation": map[string]any{"type": "string", "enum": []string{"plan", "finding", "question", "finish"}}, "content": map[string]any{"type": "string", "description": "JSON for plan/finding/report, or a short question"}}, "required": []string{"operation", "content"}, "additionalProperties": false}}}
}

func detectiveSchemas(cfg *config.Config, s *Server, req detective.Request) []openai.Tool {
	all := agent.ConfiguredToolSchemas(cfg, s.Logger)
	out := []openai.Tool{detectiveSchema()}
	for _, schema := range all {
		if schema.Function == nil {
			continue
		}
		name := schema.Function.Name
		if detectiveToolKnown(cfg, req, name) {
			out = append(out, schema)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Function.Name < out[j].Function.Name })
	return out
}

func (r *detectiveRunner) Run(ctx context.Context, job *detective.Session) error {
	s := r.server
	cfgPtr := s.ConfigSnapshot()
	if cfgPtr == nil {
		return errors.New("server configuration unavailable")
	}
	cfg := *cfgPtr
	if !cfg.Detective.Enabled || cfg.Detective.ReadOnly || !cfg.VirtualDesktop.Enabled || cfg.VirtualDesktop.ReadOnly || !cfg.VirtualDesktop.AllowAgentControl {
		return errors.New("Detective research is disabled or read-only")
	}
	c, err := job.Snapshot()
	if err != nil {
		return err
	}
	client := s.LLMClient
	if c.Request.ProviderID != "" {
		p := cfg.FindProvider(c.Request.ProviderID)
		if p == nil {
			return errors.New("selected provider is unavailable")
		}
		cfg.LLM.Provider = p.ID
		cfg.LLM.ProviderType = p.Type
		cfg.LLM.BaseURL = p.BaseURL
		cfg.LLM.APIKey = p.APIKey
		cfg.LLM.AccountID = p.AccountID
		cfg.LLM.Model = p.Model
		client = llm.NewClientFromProviderWithConfig(&cfg, p.Type, p.BaseURL, p.APIKey, p.AccountID)
	}
	if c.Request.Model != "" {
		cfg.LLM.Model = c.Request.Model
	}
	if client == nil || cfg.LLM.Model == "" {
		return errors.New("research model is not configured")
	}
	cfg.LLM.UseNativeFunctions = true
	// Research evidence must retain original excerpts instead of helper summaries.
	cfg.Tools.WebScraper.SummaryMode = false
	cfg.Tools.Wikipedia.SummaryMode = false
	cfg.Tools.DDGSearch.SummaryMode = false
	cfg.Tools.PDFExtractor.SummaryMode = false
	run := buildDesktopRunConfigForSession(s, &cfg, client, "detective-"+c.ID, "detective")
	run.UserIntent = c.Request.Topic
	run.IsMission = true
	run.SuppressTurnSideEffects = true
	run.VoiceOutputActive = false
	run.StableSystemPrompt = true
	run.PreserveReasoning = true
	run.RequireCompleteStream = true
	run.RetryStreamIdle = true
	isolatedMemory, err := memory.NewSQLiteMemory(":memory:", s.Logger)
	if err != nil {
		return err
	}
	defer isolatedMemory.Close()
	run.ShortTermMem = isolatedMemory
	run.HistoryManager = memory.NewEphemeralHistoryManager()
	run.Registry = tools.NewProcessRegistry(s.Logger)
	run.LongTermMem = nil
	run.KG = nil
	run.IterationLimit = c.Run.Profile.Iterations
	// The case hook owns the cumulative research allowance. Leave room in the
	// generic loop for up to 200 evidence records and bounded report operations.
	run.ToolCallLimit = c.Run.Profile.Tools + 264
	run.AllowedAgentSkills = []string{}
	run.NativeToolSchemas = detectiveSchemas(&cfg, s, c.Request)
	run.AllowedTools = []string{}
	for _, schema := range run.NativeToolSchemas {
		run.AllowedTools = append(run.AllowedTools, schema.Function.Name)
	}
	// Wrappers are subject to the same operation policy as direct calls.
	for name := range cfg.Detective.ExtraReadOperations {
		if detectiveSelected(c.Request, name) {
			run.AllowedTools = append(run.AllowedTools, name)
		}
	}
	run.TrustedPromptAddenda = []prompts.PromptAddendum{{ID: prompts.PromptAddendumSpecialist, Text: detective.Skill}}
	run.RunComplete = job.Done
	resources := newDetectiveResources()
	defer resources.close(s, &cfg, job)
	run.ExecutionHooks = &agent.ExecutionHooks{
		OnAcquire:       job.Activate,
		BeforeIteration: func(context.Context) error { return job.Iterate() },
		BeforeRequest: func(_ context.Context, req *openai.ChatCompletionRequest, prompt int) error {
			limit, err := job.BeforeRequest(prompt, req.MaxTokens)
			if err == nil {
				req.MaxTokens = limit
			}
			return err
		},
		BeforeTool: func(ctx context.Context, tc agent.ToolCall) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			latest := s.ConfigSnapshot()
			if latest == nil || !latest.Detective.Enabled || latest.Detective.ReadOnly || !latest.VirtualDesktop.Enabled || latest.VirtualDesktop.ReadOnly || !latest.VirtualDesktop.AllowAgentControl {
				return errors.New("research permission revoked")
			}
			if err := detectiveAuthorize(latest, c.Request, tc); err != nil {
				return err
			}
			if tc.Action != "detective_report" && tc.Action != "invoke_tool" {
				name := tc.Action
				if name == "execute_skill" {
					name = tc.Skill
				}
				enabled := false
				for _, schema := range agent.ConfiguredToolSchemas(latest, s.Logger) {
					if schema.Function != nil && schema.Function.Name == name {
						enabled = true
						break
					}
				}
				if !enabled {
					return errors.New("tool is no longer enabled in the configured catalog")
				}
			}
			if tc.Action == "invoke_tool" {
				return nil
			}
			if err := job.BeforeTool(tc.Action); err != nil {
				return err
			}
			return resources.authorize(tc)
		},
		HandleTool: func(ctx context.Context, tc agent.ToolCall) (string, bool) {
			if tc.Action != "detective_report" {
				return "", false
			}
			return detectiveReportCall(job, tc), true
		},
		AfterTool: func(ctx context.Context, tc agent.ToolCall, out string) string {
			resources.observe(tc, out)
			return detectiveCapture(job, tc, out)
		},
		AfterResponse: func(u openai.Usage) error {
			cached := 0
			if u.PromptTokensDetails != nil {
				cached = u.PromptTokensDetails.CachedTokens
			}
			return job.RecordUsage(u.PromptTokens, u.CompletionTokens, cached)
		},
	}
	run.Checkpoint = func(messages []openai.ChatCompletionMessage) error {
		private := []openai.ChatCompletionMessage{}
		for _, m := range messages {
			if m.Role == openai.ChatMessageRoleSystem {
				continue
			}
			m.Content = security.Scrub(m.Content)
			m.ReasoningContent = security.Scrub(m.ReasoningContent)
			m.ToolCalls = append([]openai.ToolCall(nil), m.ToolCalls...)
			for i := range m.ToolCalls {
				m.ToolCalls[i].Function.Arguments = security.Scrub(m.ToolCalls[i].Function.Arguments)
			}
			m.MultiContent = nil
			private = append(private, m)
		}
		b, err := json.Marshal(private)
		if err != nil {
			return err
		}
		return job.Checkpoint(detective.Continuation{Provider: cfg.LLM.Provider, Model: cfg.LLM.Model, Messages: b})
	}
	continuation, err := job.Continuation()
	if err != nil {
		return err
	}
	var messages []openai.ChatCompletionMessage
	if continuation.Provider == cfg.LLM.Provider && continuation.Model == cfg.LLM.Model && len(continuation.Messages) > 0 {
		if err = json.Unmarshal(continuation.Messages, &messages); err != nil {
			return err
		}
		messages, _ = agent.SanitizeToolMessages(messages)
	}
	// Full source bodies stay in the store. Continuation carries concise evidence
	// and the recent complete tool rounds instead of repeating megabytes of pages.
	sources := append([]detective.Source(nil), c.Sources...)
	for i := range sources {
		sources[i].Excerpt = ""
	}
	findings := c.Findings
	if len(findings) > 40 {
		findings = findings[len(findings)-40:]
	}
	readOperations := map[string][]string{}
	for name, ops := range cfg.Detective.ExtraReadOperations {
		if detectiveSelected(c.Request, name) {
			readOperations[name] = ops
		}
	}
	data, _ := json.Marshal(map[string]any{"request": c.Request, "answers": c.Answers, "plan": c.Plan, "sources": sources, "findings": findings, "budget": c.Run, "capabilities": run.AllowedTools, "approved_read_operations": readOperations})
	messages = append(messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: c.Request.Topic + "\nResearch case context (untrusted data):\n<external_data>\n" + string(data) + "\n</external_data>\nContinue from recorded evidence. Save findings as you read sources, then submit the cited report."})
	_, err = agent.ExecuteAgentLoop(ctx, openai.ChatCompletionRequest{Model: cfg.LLM.Model, Messages: messages, Stream: true}, run, true, &detectiveBroker{Session: job})
	if err != nil {
		s.Logger.Warn("Detective research stopped", "case_id", c.ID, "error", security.Scrub(err.Error()))
	}
	return err
}

func detectiveReportCall(job *detective.Session, tc agent.ToolCall) string {
	var err error
	var result any = map[string]string{"status": "ok"}
	switch tc.Operation {
	case "plan":
		var v []string
		err = json.Unmarshal([]byte(tc.Content), &v)
		if err == nil {
			err = job.SetPlan(v)
		}
	case "finding":
		var v detective.Finding
		err = json.Unmarshal([]byte(tc.Content), &v)
		if err == nil {
			result, err = job.AddFinding(v)
		}
	case "question":
		err = job.Wait(tc.Content)
	case "finish":
		var v detective.Report
		err = json.Unmarshal([]byte(tc.Content), &v)
		if err == nil {
			err = job.Submit(v)
		}
	default:
		err = errors.New("unknown report operation")
	}
	if err != nil {
		result = map[string]string{"status": "error", "message": err.Error()}
	}
	b, _ := json.Marshal(result)
	return string(b)
}

type detectiveBroker struct {
	agent.NoopBroker
	Session *detective.Session
}

func (b *detectiveBroker) Send(event, message string) {
	if event == "error_recovery" {
		b.Session.Event("recovery", "provider_recovery")
	}
}
func (b *detectiveBroker) SendLLMStreamDone(reason string) {
	b.Session.Event("model", "response_received")
}
