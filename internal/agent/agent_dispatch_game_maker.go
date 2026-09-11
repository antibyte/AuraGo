package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"math"
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
	boundJobID := gamemaker.JobIDFromContext(ctx)
	if boundJobID != "" {
		if jobID != "" && jobID != boundJobID {
			return gameMakerToolError(fmt.Errorf("job_id does not match this Studio job")), true
		}
		jobID = boundJobID
	}
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
		if operation == "get_plan" {
			plan, err := service.GetPlan(ctx, jobID)
			if err != nil {
				return gameMakerToolError(err), true
			}
			return gameMakerToolJSON(map[string]any{"status": "ok", "plan": plan}), true
		}
		if operation == "set_plan" || operation == "set_design" {
			field := "plan"
			if operation == "set_design" {
				field = "design"
			}
			data, err := json.Marshal(tc.Params[field])
			if err != nil {
				return gameMakerToolError(err), true
			}
			if operation == "set_design" {
				err = service.SetDesignJSON(ctx, jobID, data)
			} else {
				err = service.SetPlanJSON(ctx, jobID, data)
			}
			if err != nil {
				return gameMakerToolError(err), true
			}
			return gameMakerToolJSON(map[string]any{"status": "ok", "next_action": "Plan accepted. End this planning turn; the server will import assets and start implementation."}), true
		}
		if operation == "list_files" {
			files, err := service.ListJobFiles(ctx, jobID)
			if err != nil {
				return gameMakerToolError(err), true
			}
			return gameMakerToolJSON(map[string]any{"status": "ok", "files": files}), true
		}
		manifest, _ := service.ReadJobFile(ctx, jobID, "game.json")
		if operation != "inspect" {
			return gameMakerToolError(fmt.Errorf("unknown project operation")), true
		}
		return gameMakerToolJSON(map[string]any{"status": "ok", "project": project, "job": job, "manifest": manifest, "plan_example": gamemaker.ExampleGamePlan(project), "design_example": gamemaker.ExampleGameDesign(project), "next_action": gamemaker.JobNextAction(job)}), true

	case "game_maker_file":
		operation := firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation"))
		if operation == "" && boundJobID != "" {
			// A complete content payload is an unambiguous write in this isolated
			// run. Keep other callers and ambiguous path-only calls explicit.
			if _, hasContent := tc.Params["content"].(string); hasContent || tc.Content != "" {
				operation = "write"
			}
		}
		path := firstNonEmptyToolString(tc.FilePath, tc.Path, toolArgString(tc.Params, "path"), toolArgString(tc.Params, "file_path"))
		if operation == "read" {
			start, end := 0, 0
			for name, target := range map[string]*int{"start_line": &start, "end_line": &end} {
				if value, exists := tc.Params[name]; exists && value != nil {
					n, ok := value.(float64)
					if !ok || math.IsNaN(n) || n < 0 || n > 10000000 || n != math.Trunc(n) {
						return gameMakerToolError(fmt.Errorf("%s must be a nonnegative integer", name)), true
					}
					*target = int(n)
				}
			}
			result, err := service.ReadJobFileRange(ctx, jobID, path, start, end)
			if err != nil {
				return gameMakerToolError(err), true
			}
			return gameMakerToolJSON(map[string]any{"status": "ok", "path": path, "content": result.Content, "sha256": result.SHA256, "start_line": result.StartLine, "end_line": result.EndLine, "total_lines": result.TotalLines}), true
		}
		if operation != "write" && operation != "replace" {
			return `Tool Output: {"status":"error","message":"operation must be read, write or replace"}`, true
		}
		content := tc.Content
		if value, ok := tc.Params["content"].(string); ok {
			content = value
		}
		var result gamemaker.SourceWrite
		var err error
		if operation == "replace" {
			if _, ok := tc.Params["new_text"].(string); !ok {
				return gameMakerToolError(fmt.Errorf("replace requires explicit new_text; use an empty string only for deletion")), true
			}
			result, err = service.ReplaceJobFile(ctx, jobID, path, toolArgString(tc.Params, "old_text"), toolArgString(tc.Params, "new_text"), toolArgString(tc.Params, "expected_sha256"))
		} else {
			if _, ok := tc.Params["content"].(string); !ok && tc.Content == "" {
				return gameMakerToolError(fmt.Errorf("write requires explicit string content")), true
			}
			result, err = service.WriteJobFileChecked(ctx, jobID, path, content, toolArgString(tc.Params, "expected_sha256"))
		}
		if err != nil {
			return gameMakerToolError(err), true
		}
		return gameMakerToolJSON(map[string]any{"status": "ok", "operation": operation, "path": path, "written": result.Written, "sha256": result.SHA256, "build": result.Build, "next_action": "If build.ok is false, repair the reported source location. A saved file is not a validated game."}), true

	case "game_maker_validate":
		result := service.ValidateJobScope(ctx, jobID, toolArgString(tc.Params, "scope"))
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
	operation := firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation"), "generate")
	packID := toolArgString(tc.Params, "pack_id")
	switch operation {
	case "search_assets":
		limit := 6
		if n, ok := tc.Params["limit"].(float64); ok {
			limit = int(n)
		}
		matches, err := service.SearchAssets(toolArgString(tc.Params, "query"), packID, toolArgString(tc.Params, "view"), limit)
		if err != nil {
			return gameMakerToolError(err)
		}
		return gameMakerToolJSON(map[string]any{"status": "ok", "matches": matches, "next_action": "Use describe_asset with one exact asset_id or assembly_id before adding the selection to the plan."})
	case "describe_asset":
		detail, err := service.DescribeAsset(packID, toolArgString(tc.Params, "asset_id"), toolArgString(tc.Params, "assembly_id"))
		if err != nil {
			return gameMakerToolError(err)
		}
		return gameMakerToolJSON(map[string]any{"status": "ok", "detail": detail})
	case "list_packs":
		packs, err := service.ListAssetPacks()
		if err != nil {
			return gameMakerToolError(err)
		}
		return gameMakerToolJSON(map[string]any{"status": "ok", "packs": packs})
	case "describe_pack":
		pack, err := service.DescribeAssetPack(packID)
		if err != nil {
			return gameMakerToolError(err)
		}
		return gameMakerToolJSON(map[string]any{"status": "ok", "pack": pack})
	case "import_pack":
		var ids []string
		if raw, ok := tc.Params["asset_ids"]; ok {
			encoded, err := json.Marshal(raw)
			if err != nil || json.Unmarshal(encoded, &ids) != nil {
				return gameMakerToolError(fmt.Errorf("asset_ids must be an array of exact model IDs"))
			}
		}
		pack, err := service.ImportAssetPack(ctx, jobID, packID, ids...)
		if err != nil {
			return gameMakerToolError(err)
		}
		return gameMakerToolJSON(map[string]any{"status": "ok", "pack": pack})
	case "generate":
	default:
		return gameMakerToolError(fmt.Errorf("unsupported asset operation"))
	}
	if err := service.CheckJobMutation(ctx, jobID); err != nil {
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
		path = strings.TrimSuffix(path, filepath.Ext(path)) + ".svg"
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
