package server

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/gamemaker"
	"aurago/internal/llm"
	"aurago/internal/tools"

	"github.com/sashabaranov/go-openai"
)

var gameMakerAllowedTools = []string{
	"game_maker_project",
	"game_maker_file",
	"game_maker_asset",
	"game_maker_validate",
	"list_agent_skills",
	"activate_agent_skill",
}

func gameMakerPolicy(cfg config.GameMakerConfig) gamemaker.Policy {
	return gamemaker.Policy{
		Enabled:              cfg.Enabled,
		ReadOnly:             cfg.ReadOnly,
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
		ReadOnly:             cfg.ReadOnly,
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
	service.UpdatePolicy(gameMakerPolicy(cfg))
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

func verifyGameMakerAgentSkills(manager *tools.AgentSkillManager, install gamemaker.SkillInstallResult, logger *slog.Logger) ([]gamemaker.SkillInfo, bool) {
	ready := install.Ready && manager != nil
	skills := append([]gamemaker.SkillInfo(nil), install.Skills...)
	for i := range skills {
		if skills[i].Status == "hash_mismatch" || manager == nil {
			ready = false
			continue
		}
		entry, err := manager.GetAgentSkillByName(skills[i].Name)
		if err != nil {
			skills[i].Status = "missing"
			ready = false
			continue
		}
		if entry.SecurityStatus != tools.SecurityClean {
			skills[i].Status = string(entry.SecurityStatus)
			ready = false
			continue
		}
		if _, err := manager.LoadCurrentAgentSkillPackage(entry, "system:game-maker"); err != nil {
			skills[i].Status = "hash_mismatch"
			ready = false
			continue
		}
		if !entry.Enabled {
			if err := manager.EnableAgentSkill(entry.ID, true, "system:game-maker"); err != nil {
				skills[i].Status = "disabled"
				ready = false
				if logger != nil {
					logger.Warn("Failed to enable bundled Game Maker Agent Skill", "name", entry.Name, "error", err)
				}
				continue
			}
		}
		skills[i].Status = "ready"
	}
	return skills, ready
}

type gameMakerAgentRunner struct {
	server  *Server
	service *gamemaker.Service
}

func (r *gameMakerAgentRunner) RunGameMakerJob(ctx context.Context, run gamemaker.JobRun) error {
	s := r.server
	if s == nil || s.Cfg == nil || s.LLMClient == nil {
		return fmt.Errorf("Game Maker LLM is not configured")
	}
	s.CfgMu.RLock()
	cfg := *s.Cfg
	s.CfgMu.RUnlock()

	client := s.LLMClient
	if providerID := strings.TrimSpace(run.Job.ProviderID); providerID != "" {
		provider := cfg.FindProvider(providerID)
		if provider == nil {
			return fmt.Errorf("selected Game Maker provider %q is not configured", providerID)
		}
		cfg.LLM.Provider = provider.ID
		cfg.LLM.ProviderType = provider.Type
		cfg.LLM.BaseURL = provider.BaseURL
		cfg.LLM.APIKey = provider.APIKey
		cfg.LLM.AccountID = provider.AccountID
		cfg.LLM.Model = provider.Model
		client = llm.NewClientFromProviderWithConfig(&cfg, provider.Type, provider.BaseURL, provider.APIKey, provider.AccountID)
	}
	if model := strings.TrimSpace(run.Job.Model); model != "" {
		cfg.LLM.Model = model
	}
	if strings.TrimSpace(cfg.LLM.Model) == "" {
		return fmt.Errorf("selected Game Maker model is empty")
	}
	cfg.LLM.UseNativeFunctions = true
	engineSkill := "aurago-phaser4-gameplay"
	if run.Project.Dimension == "3d" {
		engineSkill = "aurago-threejs-gameplay"
	}
	gamePrompt := fmt.Sprintf(`You are AuraGo Game Maker Studio running isolated job %q.
The project is %s (%s) and the user request is:
%s

Activate aurago-game-maker-director, %s, aurago-game-assets, and aurago-game-qa.
Inspect the current staging project, implement a coherent playable offline game,
use only the allowed Game Maker tools, preserve the diagnostic interface, and
finish by calling game_maker_validate. Do not ask follow-up questions.`,
		run.Job.ID, run.Project.Name, run.Project.Dimension, run.Job.Prompt, engineSkill)
	cfg.Agent.AdditionalPrompt = appendDesktopAdditionalPrompt(cfg.Agent.AdditionalPrompt, gamePrompt)
	sessionID := "game-maker-" + run.Job.ID
	runCfg := buildDesktopRunConfigForSession(s, &cfg, client, sessionID, "game_maker")
	runCfg.AllowedTools = append([]string(nil), gameMakerAllowedTools...)
	runCfg.AllowedAgentSkills = gamemaker.CuratedSkillNames()
	runCfg.SuppressTurnSideEffects = true
	runCfg.IsMission = true
	runCfg.VoiceOutputActive = false

	req := openai.ChatCompletionRequest{
		Model: cfg.LLM.Model,
		Messages: []openai.ChatCompletionMessage{{
			Role:    openai.ChatMessageRoleUser,
			Content: gamePrompt,
		}},
		Stream: true,
	}
	broker := &gameMakerBroker{service: r.service, projectID: run.Project.ID, jobID: run.Job.ID}
	defer func() {
		if s.ShortTermMem != nil {
			if err := s.ShortTermMem.PurgeChatSession(sessionID); err != nil && s.Logger != nil {
				s.Logger.Warn("Failed to purge transient Game Maker agent session", "session_id", sessionID, "error", err)
			}
		}
	}()
	response, err := agent.ExecuteAgentLoop(ctx, req, runCfg, true, broker)
	if err != nil {
		return fmt.Errorf("Game Maker agent loop: %w", err)
	}
	answer := strings.TrimSpace(broker.text())
	if answer == "" && len(response.Choices) > 0 {
		answer = strings.TrimSpace(response.Choices[0].Message.Content)
	}
	if answer != "" {
		if err := r.service.RecordAgentMessage(context.Background(), run.Project.ID, run.Job.ID, answer); err != nil {
			return err
		}
	}
	return nil
}

type gameMakerBroker struct {
	service   *gamemaker.Service
	projectID string
	jobID     string
	mu        sync.Mutex
	response  strings.Builder
}

func (b *gameMakerBroker) Send(event, message string) {
	if strings.TrimSpace(message) == "" {
		return
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
		b.response.WriteString(content)
		b.mu.Unlock()
		_ = b.service.EmitAgentEvent(context.Background(), b.projectID, b.jobID, "text_delta", map[string]any{"content": content})
	}
	if toolName == "activate_agent_skill" {
		_ = b.service.EmitAgentEvent(context.Background(), b.projectID, b.jobID, "skill_activation", map[string]any{"tool_id": toolID})
	}
}

func (b *gameMakerBroker) SendLLMStreamDone(string) {}
func (b *gameMakerBroker) SendTokenUpdate(int, int, int, int, int, bool, bool, string) {
}
func (b *gameMakerBroker) SendThinkingBlock(string, string, string) {}

func (b *gameMakerBroker) text() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.response.String()
}
