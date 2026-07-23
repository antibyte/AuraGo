package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"

	"aurago/internal/gamemaker"
	"aurago/internal/tools"
)

func dispatchGameMaker(ctx context.Context, tc ToolCall, dc *DispatchContext) (string, bool) {
	switch tc.Action {
	case "game_maker_project", "game_maker_file", "game_maker_asset", "game_maker_validate":
	default:
		return "", false
	}
	service := gamemaker.DefaultService()
	if service == nil {
		return `Tool Output: {"status":"error","message":"Game Maker service is unavailable"}`, true
	}
	jobID := toolArgString(tc.Params, "job_id")
	if jobID == "" {
		return `Tool Output: {"status":"error","message":"job_id is required"}`, true
	}
	switch tc.Action {
	case "game_maker_project":
		operation := firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation"))
		project, job, err := service.ProjectForJob(ctx, jobID)
		if err != nil {
			return gameMakerToolError(err), true
		}
		if operation == "list_files" {
			files, err := service.ListJobFiles(ctx, jobID)
			if err != nil {
				return gameMakerToolError(err), true
			}
			return gameMakerToolJSON(map[string]any{"status": "ok", "files": files}), true
		}
		manifest, _ := service.ReadJobFile(ctx, jobID, "game.json")
		return gameMakerToolJSON(map[string]any{"status": "ok", "project": project, "job": job, "manifest": manifest}), true

	case "game_maker_file":
		operation := firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation"))
		path := firstNonEmptyToolString(tc.FilePath, tc.Path, toolArgString(tc.Params, "path"), toolArgString(tc.Params, "file_path"))
		if operation == "read" {
			content, err := service.ReadJobFile(ctx, jobID, path)
			if err != nil {
				return gameMakerToolError(err), true
			}
			return gameMakerToolJSON(map[string]any{"status": "ok", "path": path, "content": content}), true
		}
		if operation != "write" {
			return `Tool Output: {"status":"error","message":"operation must be read or write"}`, true
		}
		content := tc.Content
		if value, ok := tc.Params["content"].(string); ok {
			content = value
		}
		if err := service.WriteJobFile(ctx, jobID, path, content); err != nil {
			return gameMakerToolError(err), true
		}
		return gameMakerToolJSON(map[string]any{"status": "ok", "path": path}), true

	case "game_maker_validate":
		result := service.BuildJob(ctx, jobID)
		return gameMakerToolJSON(map[string]any{"status": gameMakerValidationStatus(result.OK), "result": result}), true

	case "game_maker_asset":
		return dispatchGameMakerAsset(ctx, tc, dc, service, jobID), true
	}
	return `Tool Output: {"status":"error","message":"unsupported Game Maker operation"}`, true
}

func dispatchGameMakerAsset(ctx context.Context, tc ToolCall, dc *DispatchContext, service *gamemaker.Service, jobID string) string {
	project, _, err := service.ProjectForJob(ctx, jobID)
	if err != nil {
		return gameMakerToolError(err)
	}
	kind := strings.ToLower(firstNonEmptyToolString(toolArgString(tc.Params, "kind"), tc.Mode))
	prompt := firstNonEmptyToolString(tc.Query, tc.Description, toolArgString(tc.Params, "prompt"))
	path := firstNonEmptyToolString(tc.Path, tc.FilePath, toolArgString(tc.Params, "path"))
	if prompt == "" || path == "" {
		return `Tool Output: {"status":"error","message":"prompt and path are required"}`
	}
	if dc == nil || dc.Cfg == nil || !dc.Cfg.GameMaker.AllowMediaGeneration {
		return proceduralGameMakerFallback(ctx, service, jobID, kind, path, prompt, "media generation is disabled")
	}
	switch kind {
	case "image":
		if !project.UseImageGeneration || !dc.Cfg.ImageGeneration.Enabled || dc.Cfg.ImageGeneration.APIKey == "" {
			return proceduralGameMakerFallback(ctx, service, jobID, kind, path, prompt, "image generation is not configured for this project")
		}
		if dc.BudgetTracker != nil && dc.BudgetTracker.IsBlocked("image_generation") {
			return proceduralGameMakerFallback(ctx, service, jobID, kind, path, prompt, "image generation budget is exhausted")
		}
		cfg := tools.ImageGenConfig{
			ProviderType: dc.Cfg.ImageGeneration.ProviderType,
			BaseURL:      dc.Cfg.ImageGeneration.BaseURL,
			APIKey:       dc.Cfg.ImageGeneration.APIKey,
			Model:        dc.Cfg.ImageGeneration.ResolvedModel,
			DataDir:      dc.Cfg.Directories.DataDir,
		}
		result, err := tools.GenerateImage(cfg, prompt, tools.ImageGenOptions{
			Size: dc.Cfg.ImageGeneration.DefaultSize, Quality: dc.Cfg.ImageGeneration.DefaultQuality,
			Style: dc.Cfg.ImageGeneration.DefaultStyle,
		})
		if err != nil {
			return proceduralGameMakerFallback(ctx, service, jobID, kind, path, prompt, "image provider failed: "+err.Error())
		}
		data, err := os.ReadFile(filepath.Join(dc.Cfg.Directories.DataDir, "generated_images", result.Filename))
		if err != nil {
			return proceduralGameMakerFallback(ctx, service, jobID, kind, path, prompt, "generated image could not be copied")
		}
		stored, err := service.StoreJobAsset(ctx, jobID, path, "image", result.Provider, "AuraGo image generation", data)
		if err != nil {
			return gameMakerToolError(err)
		}
		if dc.BudgetTracker != nil && result.CostEstimate > 0 {
			dc.BudgetTracker.RecordCostForCategory("image_generation", result.CostEstimate)
		}
		return gameMakerToolJSON(map[string]any{"status": "ok", "path": stored, "provider": result.Provider, "model": result.Model})

	case "music":
		if !project.UseMusicGeneration || !dc.Cfg.MusicGeneration.Enabled || dc.Cfg.MusicGeneration.APIKey == "" {
			return proceduralGameMakerFallback(ctx, service, jobID, kind, path, prompt, "music generation is not configured for this project")
		}
		if dc.BudgetTracker != nil && dc.BudgetTracker.IsBlocked("music_generation") {
			return proceduralGameMakerFallback(ctx, service, jobID, kind, path, prompt, "music generation budget is exhausted")
		}
		result := tools.GenerateMusicResult(ctx, dc.Cfg, dc.MediaRegistryDB, dc.Logger, tools.MusicGenParams{
			Prompt: prompt, Instrumental: true, Title: toolArgString(tc.Params, "title"),
		})
		if result.Status != "ok" {
			return proceduralGameMakerFallback(ctx, service, jobID, kind, path, prompt, firstNonEmptyToolString(result.Error, result.Message))
		}
		data, err := os.ReadFile(result.FilePath)
		if err != nil {
			return proceduralGameMakerFallback(ctx, service, jobID, kind, path, prompt, "generated music could not be copied")
		}
		stored, err := service.StoreJobAsset(ctx, jobID, path, "music", result.Provider, "AuraGo music generation", data)
		if err != nil {
			return gameMakerToolError(err)
		}
		if dc.BudgetTracker != nil && result.CostEstimate > 0 {
			dc.BudgetTracker.RecordCostForCategory("music_generation", result.CostEstimate)
		}
		return gameMakerToolJSON(map[string]any{"status": "ok", "path": stored, "provider": result.Provider, "model": result.Model})
	default:
		return `Tool Output: {"status":"error","message":"kind must be image or music"}`
	}
}

func proceduralGameMakerFallback(ctx context.Context, service *gamemaker.Service, jobID, kind, path, prompt, reason string) string {
	if kind == "image" {
		svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="512" height="512" viewBox="0 0 512 512"><defs><linearGradient id="g" x2="1" y2="1"><stop stop-color="#22d3ee"/><stop offset="1" stop-color="#7c3aed"/></linearGradient></defs><rect width="512" height="512" fill="#081018"/><circle cx="256" cy="232" r="150" fill="url(#g)" opacity=".85"/><text x="256" y="452" text-anchor="middle" fill="white" font-family="system-ui" font-size="20">%s</text></svg>`, html.EscapeString(truncateGameMakerLabel(prompt, 34)))
		stored, err := service.StoreJobAsset(ctx, jobID, path, "image", "procedural", reason, []byte(svg))
		if err != nil {
			return gameMakerToolError(err)
		}
		return gameMakerToolJSON(map[string]any{"status": "fallback", "path": stored, "reason": reason})
	}
	return gameMakerToolJSON(map[string]any{
		"status": "fallback", "reason": reason,
		"instruction": "Use procedural Web Audio effects and music after a user gesture; no music file was created.",
	})
}

func gameMakerToolJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return `Tool Output: {"status":"error","message":"could not serialize Game Maker result"}`
	}
	return "Tool Output: " + string(data)
}

func gameMakerToolError(err error) string {
	return gameMakerToolJSON(map[string]any{"status": "error", "message": err.Error()})
}

func gameMakerValidationStatus(ok bool) string {
	if ok {
		return "ok"
	}
	return "error"
}

func truncateGameMakerLabel(value string, max int) string {
	value = strings.TrimSpace(value)
	if len(value) <= max {
		return value
	}
	return value[:max-1] + "…"
}
