package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"aurago/internal/gamemaker"
	"aurago/internal/tools"
)

func dispatchGameMaker(ctx context.Context, tc ToolCall, dc *DispatchContext) (output string, handled bool) {
	switch tc.Action {
	case "game_maker_project", "game_maker_file", "game_maker_asset", "game_maker_validate", "game_maker_preview":
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
	started := time.Now()
	defer func() { recordGameMakerToolResult(ctx, service, jobID, tc, output, time.Since(started)) }()
	defer func() { output = annotateGameMakerNormalizedCall(output, tc) }()
	switch tc.Action {
	case "game_maker_project":
		operation := firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation"))
		project, job, err := service.ProjectForJob(ctx, jobID)
		if err != nil {
			return gameMakerToolError(err), true
		}
		switch operation {
		case "build", "finish":
			var result gamemaker.BuildResult
			if operation == "finish" {
				result = service.FinishJob(ctx, jobID)
			} else {
				result = service.BuildJob(ctx, jobID)
			}
			return gameMakerToolJSON(map[string]any{"status": gameMakerValidationStatus(result.OK), "result": result}), true
		case "use_template":
			if err := service.UseTemplate(ctx, jobID, toolArgString(tc.Params, "template")); err != nil {
				return gameMakerToolError(err), true
			}
			return gameMakerToolJSON(map[string]any{"status": "ok", "runtime": service.RuntimeContext(ctx, jobID)}), true
		case "reference":
			return gameMakerToolJSON(map[string]any{"status": "ok", "runtime": service.FrameworkReference(ctx, jobID, toolArgString(tc.Params, "framework"))}), true
		case "list_files":
			files, err := service.ListJobFiles(ctx, jobID)
			if err != nil {
				return gameMakerToolError(err), true
			}
			return gameMakerToolJSON(map[string]any{"status": "ok", "files": files}), true
		case "inspect":
			manifest, _ := service.ReadJobFile(ctx, jobID, "game.json")
			return gameMakerToolJSON(map[string]any{"status": "ok", "project": project, "job": job, "manifest": manifest}), true
		case "get_plan":
			plan, err := service.GetPlan(ctx, jobID)
			if err != nil {
				return gameMakerToolError(err), true
			}
			return gameMakerToolJSON(map[string]any{"status": "ok", "legacy_plan": plan}), true
		default:
			return gameMakerToolError(fmt.Errorf("unknown project operation; use inspect, list_files, build, finish, reference or use_template")), true
		}

	case "game_maker_file":
		operation := firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation"))
		if operation == "" && boundJobID != "" {
			// A complete content payload is an unambiguous write in this isolated
			// run. Keep other callers and ambiguous path-only calls explicit.
			if _, hasContent := tc.Params["content"].(string); hasContent || tc.Content != "" {
				operation = "write"
			} else if tc.Params["start_line"] != nil || tc.Params["end_line"] != nil {
				// A line range without content can only be a read.
				operation = "read"
			}
		}
		path := firstNonEmptyToolString(tc.FilePath, tc.Path, toolArgString(tc.Params, "path"), toolArgString(tc.Params, "file_path"))
		if operation == "search" {
			query := firstNonEmptyToolString(tc.Query, toolArgString(tc.Params, "query"))
			if strings.TrimSpace(path) == "" {
				// Without a path the agent is still locating the owning file.
				result, err := service.SearchJobFiles(ctx, jobID, query)
				if err != nil {
					return gameMakerToolError(err), true
				}
				answer := map[string]any{"status": "ok", "result": result, "next_action": "Read the matching range of the owning file; its sha256 is required for replace."}
				if hint := gameMakerFileHint(ctx, service, jobID, path); hint != "" {
					answer["next_action"] = hint
				}
				return gameMakerToolJSON(answer), true
			}
			result, err := service.SearchJobFile(ctx, jobID, path, query)
			if err != nil {
				return gameMakerFileToolError(ctx, service, jobID, boundJobID != "", err), true
			}
			answer := map[string]any{"status": "ok", "result": result}
			if hint := gameMakerFileHint(ctx, service, jobID, path); hint != "" {
				answer["next_action"] = hint
			}
			return gameMakerToolJSON(answer), true
		}
		if operation == "replace_many" {
			data, err := json.Marshal(tc.Params["edits"])
			if err != nil {
				return gameMakerToolError(err), true
			}
			// Native providers may return structured object arrays as JSON text.
			if value, ok := tc.Params["edits"].(string); ok {
				data = []byte(value)
			}
			var edits []gamemaker.SourceReplacement
			decoder := json.NewDecoder(strings.NewReader(string(data)))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&edits); err != nil {
				return gameMakerToolError(fmt.Errorf("edits: %w", err)), true
			}
			if err := decoder.Decode(new(any)); err != io.EOF {
				return gameMakerToolError(fmt.Errorf("edits: submit one JSON array")), true
			}
			result, err := service.ReplaceJobFiles(ctx, jobID, edits)
			if err != nil {
				return gameMakerToolError(err), true
			}
			answer := map[string]any{"status": "ok", "result": result}
			return gameMakerToolJSON(answer), true
		}
		if operation == "read" {
			start, end := 0, 0
			for name, target := range map[string]*int{"start_line": &start, "end_line": &end} {
				if value, exists := tc.Params[name]; exists && value != nil {
					n, ok := value.(float64)
					// XML fallback parameters are text; retain the same numeric bounds.
					if text, isText := value.(string); isText {
						var err error
						n, err = strconv.ParseFloat(strings.TrimSpace(text), 64)
						ok = err == nil
					}
					if !ok || math.IsNaN(n) || n < 0 || n > 10000000 || n != math.Trunc(n) {
						return gameMakerToolError(fmt.Errorf("%s must be a nonnegative integer", name)), true
					}
					*target = int(n)
				}
			}
			result, err := service.ReadJobFileRange(ctx, jobID, path, start, end)
			if err != nil {
				return gameMakerFileToolError(ctx, service, jobID, boundJobID != "", err), true
			}
			answer := map[string]any{"status": "ok", "path": path, "content": result.Content, "sha256": result.SHA256, "start_line": result.StartLine, "end_line": result.EndLine, "total_lines": result.TotalLines}
			if hint := gameMakerFileHint(ctx, service, jobID, path); hint != "" {
				answer["next_action"] = hint
			}
			return gameMakerToolJSON(answer), true
		}
		if operation != "write" && operation != "replace" {
			return `Tool Output: {"status":"error","message":"operation must be read, search, write, replace or replace_many"}`, true
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
		return gameMakerToolJSON(map[string]any{"status": "ok", "operation": operation, "path": path, "written": result.Written, "sha256": result.SHA256}), true

	case "game_maker_validate":
		result := service.BuildJob(ctx, jobID)
		return gameMakerToolJSON(map[string]any{"status": gameMakerValidationStatus(result.OK), "diagnostic_kind": "build_only", "result": result}), true
	case "game_maker_preview":
		if boundJobID == "" {
			return gameMakerToolError(fmt.Errorf("preview requires a bound Studio job")), true
		}
		data, err := json.Marshal(tc.Params["actions"])
		if err != nil {
			return gameMakerToolError(err), true
		}
		var actions []gamemaker.PreviewInput
		if err := json.Unmarshal(data, &actions); err != nil {
			return gameMakerToolError(err), true
		}
		result, err := service.AgentPreview(ctx, jobID, firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")), actions)
		if err != nil {
			return gameMakerToolError(err), true
		}
		return gameMakerToolJSON(result), true

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
		matches, err := service.SearchAssets(toolArgString(tc.Params, "query"), packID, toolArgString(tc.Params, "view"), limit, toolArgString(tc.Params, "asset_kind"))
		if err != nil {
			return gameMakerToolError(err)
		}
		return gameMakerToolJSON(map[string]any{"status": "ok", "matches": matches, "next_action": "Call game_maker_asset operation=describe_asset with pack_id AND asset_id (or assembly_id) from the same match."})
	case "describe_asset":
		// The accepted plan already owns exact bindings; never guess across packs.
		if packID == "" {
			if plan, planErr := service.GetPlan(ctx, jobID); planErr == nil && plan != nil {
				for _, asset := range plan.Assets {
					if asset.AssetID != toolArgString(tc.Params, "asset_id") || asset.AssemblyID != toolArgString(tc.Params, "assembly_id") || asset.PackID == "" {
						continue
					}
					if packID != "" && packID != asset.PackID {
						return gameMakerToolError(fmt.Errorf("asset ID is used in multiple packs; supply the exact pack_id from the plan"))
					}
					packID = asset.PackID
				}
			}
		}
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
		result, err := tools.GenerateImageContext(ctx, cfg, prompt, tools.ImageGenOptions{
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
		if !project.UseMusicGeneration || !dc.Cfg.MusicConfigured() {
			return proceduralGameMakerFallback(ctx, service, jobID, kind, path, prompt, "music generation is not configured for this project")
		}
		if !dc.Cfg.UsesLocalMusic() && dc.BudgetTracker != nil && dc.BudgetTracker.IsBlocked("music_generation") {
			return proceduralGameMakerFallback(ctx, service, jobID, kind, path, prompt, "music generation budget is exhausted")
		}
		var control tools.MusicGenParams
		encoded, _ := json.Marshal(tc.Params)
		if err := json.Unmarshal(encoded, &control); err != nil {
			return gameMakerToolError(fmt.Errorf("invalid_music_parameters"))
		}
		result := tools.GenerateMusicResult(ctx, dc.Cfg, dc.MediaRegistryDB, dc.Logger, tools.MusicGenParams{
			Prompt: prompt, Instrumental: true, Title: toolArgString(tc.Params, "title"),
			DurationSeconds: control.DurationSeconds, BPM: control.BPM, Seed: control.Seed,
		})
		if result.Status != "ok" {
			if dc.Cfg.UsesLocalMusic() {
				return gameMakerToolError(fmt.Errorf("local music: %s", result.Error))
			}
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

// gameMakerVendorHint redirects exploration of minified runtime bundles to the
// documented API; real runs spent dozens of searches reverse-engineering them.
func gameMakerVendorHint(path string) string {
	clean := strings.TrimPrefix(strings.ReplaceAll(strings.TrimSpace(path), "\\", "/"), "./")
	if !strings.HasPrefix(clean, "vendor/") || !strings.HasSuffix(clean, ".js") {
		return ""
	}
	return "vendor/*.js files are minified runtime builds. Framework documentation is available from game_maker_project/reference; managed vendor files are read-only."
}

func gameMakerFileHint(_ context.Context, _ *gamemaker.Service, _, path string) string {
	return gameMakerVendorHint(path)
}

// gameMakerToolJSON keeps <, > and & literal: results reach a model, never an
// HTML page, and source must read exactly as stored for replace edits.
func gameMakerToolJSON(value any) string {
	var data bytes.Buffer
	encoder := json.NewEncoder(&data)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return `Tool Output: {"status":"error","message":"could not serialize Game Maker result"}`
	}
	return "Tool Output: " + strings.TrimSuffix(data.String(), "\n")
}

// annotateGameMakerNormalizedCall tells the model which tool and operation a
// misaddressed call ran as (canonicalizeGameMakerOperationCall), so it learns the
// real name. Only a JSON-object result of a rewritten call is annotated.
func annotateGameMakerNormalizedCall(output string, tc ToolCall) string {
	body := strings.TrimPrefix(output, "Tool Output: ")
	if tc.normalizedFrom == "" || body == output || !strings.HasPrefix(body, "{") || !strings.HasSuffix(body, "}") || !json.Valid([]byte(body)) {
		return output
	}
	note, err := json.Marshal(struct {
		Tool      string `json:"tool"`
		Operation string `json:"operation"`
	}{tc.Action, firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation"))})
	if err != nil {
		return output
	}
	separator := ","
	if body == "{}" {
		separator = ""
	}
	return "Tool Output: " + strings.TrimSuffix(body, "}") + separator + `"normalized_call":` + string(note) + "}"
}

// gameMakerSourceFileLimit bounds the existing-source list in a missing-file answer.
const gameMakerSourceFileLimit = 12

var gameMakerSourceExtensions = map[string]bool{
	".ts": true, ".tsx": true, ".js": true, ".jsx": true, ".mjs": true, ".json": true,
	".css": true, ".html": true, ".glsl": true, ".vert": true, ".frag": true, ".wgsl": true,
	".txt": true, ".md": true,
}

// gameMakerExistingSources renders the text source files under src/ (sorted, at
// most gameMakerSourceFileLimit) so a model that guessed a helper name, such as
// src/common.ts in a 3D project, can read the real one.
func gameMakerExistingSources(files []string) string {
	var sources []string
	for _, file := range files {
		if strings.HasPrefix(file, "src/") && gameMakerSourceExtensions[strings.ToLower(filepath.Ext(file))] {
			sources = append(sources, file)
		}
	}
	if len(sources) == 0 {
		return ""
	}
	sort.Strings(sources)
	extra := len(sources) - gameMakerSourceFileLimit
	if extra > 0 {
		sources = sources[:gameMakerSourceFileLimit]
	}
	list := strings.Join(sources, ", ")
	if extra > 0 {
		list += fmt.Sprintf(" (+%d more)", extra)
	}
	return "existing src files: " + list
}

// gameMakerFileToolError answers a failed file read or search. For a bound
// Studio job a missing file also names the source files that do exist, because
// 3D projects use differently named helpers than the 2D templates.
func gameMakerFileToolError(ctx context.Context, service *gamemaker.Service, jobID string, bound bool, err error) string {
	if !bound || !errors.Is(err, fs.ErrNotExist) {
		return gameMakerToolError(err)
	}
	files, listErr := service.ListJobFiles(ctx, jobID)
	if listErr != nil {
		return gameMakerToolError(err)
	}
	return gameMakerToolErrorHint(err, gameMakerExistingSources(files))
}

func gameMakerToolError(err error) string { return gameMakerToolErrorHint(err, "") }

func gameMakerToolErrorHint(err error, existing string) string {
	// Operating-system errors quote resolved locations; the model only ever
	// receives project-relative paths.
	message := gamemaker.DefaultService().RedactHostPaths(err.Error())
	if errors.Is(err, fs.ErrNotExist) {
		message += ". Use game_maker_project operation=list_files for this project's paths"
		if existing != "" {
			message += "; " + existing
		}
	}
	var detail *gamemaker.DesignValidationError
	if errors.As(err, &detail) {
		return gameMakerToolJSON(map[string]any{"status": "error", "message": message, "errors": detail.Issues, "remaining_attempts": detail.RemainingAttempts, "retained_base": detail.RetainedBase, "correction": "Fix the listed fields together. Omitted fields are retained; supplied arrays replace the whole array."})
	}
	return gameMakerToolJSON(map[string]any{"status": "error", "message": message})
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
