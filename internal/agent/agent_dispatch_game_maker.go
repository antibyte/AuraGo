package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	"aurago/internal/gamemaker"
	"aurago/internal/tools"
)

func gameMakerScenePayload(params map[string]any, key string) ([]byte, error) {
	value, ok := params[key]
	if !ok || value == nil {
		return nil, fmt.Errorf("%s is required", key)
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode %s: %w", key, err)
	}
	data = bytes.TrimSpace(data)
	if len(data) > 32768 {
		return nil, fmt.Errorf("%s exceeds the 32 KiB operation limit", key)
	}
	if len(data) > 0 && data[0] == '"' {
		var encoded string
		if err := json.Unmarshal(data, &encoded); err != nil {
			return nil, fmt.Errorf("decode %s JSON string: %w", key, err)
		}
		data = bytes.TrimSpace([]byte(encoded))
	}
	if len(data) == 0 || data[0] != '{' {
		return nil, fmt.Errorf("%s must be a JSON object", key)
	}
	return data, nil
}

func gameMakerSceneDryRun(params map[string]any) bool {
	value, ok := params["dry_run"].(bool)
	return ok && value
}

type gameMakerSceneFilter struct {
	NodeIDs   []string
	NodeIDSet map[string]struct{}
	RegionID  string
}

func gameMakerSceneFilterFromParams(params map[string]any) (gameMakerSceneFilter, error) {
	filter := gameMakerSceneFilter{NodeIDSet: map[string]struct{}{}}
	if raw, ok := params["node_ids"]; ok && raw != nil {
		var values []any
		switch typed := raw.(type) {
		case []any:
			values = typed
		case []string:
			values = make([]any, len(typed))
			for i, value := range typed {
				values[i] = value
			}
		default:
			return filter, fmt.Errorf("node_ids must be an array of strings")
		}
		if len(values) > 32 {
			return filter, fmt.Errorf("node_ids accepts at most 32 IDs")
		}
		for _, value := range values {
			id, ok := value.(string)
			id = strings.TrimSpace(id)
			if !ok || id == "" {
				return filter, fmt.Errorf("node_ids must contain non-empty strings")
			}
			if len(id) > 128 {
				return filter, fmt.Errorf("node_ids entries must be at most 128 characters")
			}
			if _, seen := filter.NodeIDSet[id]; seen {
				continue
			}
			filter.NodeIDSet[id] = struct{}{}
			filter.NodeIDs = append(filter.NodeIDs, id)
		}
	}
	filter.RegionID = strings.TrimSpace(toolArgString(params, "region_id"))
	if len(filter.RegionID) > 128 {
		return filter, fmt.Errorf("region_id must be at most 128 characters")
	}
	return filter, nil
}

func (f gameMakerSceneFilter) enabled() bool {
	return len(f.NodeIDs) > 0 || f.RegionID != ""
}

func (f gameMakerSceneFilter) selectsNode(node gamemaker.SceneNode) bool {
	if len(f.NodeIDs) > 0 {
		if _, ok := f.NodeIDSet[node.ID]; !ok {
			return false
		}
	}
	return f.RegionID == "" || node.RegionID == f.RegionID
}

func compactGameMakerSceneProperties(properties map[string]any) any {
	if properties == nil {
		return nil
	}
	data, err := json.Marshal(properties)
	if err == nil && len(data) <= 2048 {
		return properties
	}
	size := len(data)
	if err != nil {
		size = 0
	}
	return map[string]any{"truncated": true, "bytes": size}
}

func gameMakerSceneDetail(scene gamemaker.Scene, filter gameMakerSceneFilter) map[string]any {
	detail := map[string]any{
		"nodes":   []map[string]any{},
		"regions": []gamemaker.SceneRegion{},
	}
	filterData := map[string]any{}
	if len(filter.NodeIDs) > 0 {
		filterData["node_ids"] = filter.NodeIDs
	}
	if filter.RegionID != "" {
		filterData["region_id"] = filter.RegionID
	}
	detail["filter"] = filterData

	selectedRegions := map[string]struct{}{}
	nodes := detail["nodes"].([]map[string]any)
	truncated := false
	for _, node := range scene.Nodes {
		if !filter.selectsNode(node) {
			continue
		}
		if len(nodes) >= 32 {
			truncated = true
			break
		}
		if node.RegionID != "" {
			selectedRegions[node.RegionID] = struct{}{}
		}
		entry := map[string]any{
			"id": node.ID, "kind": node.Kind, "position": node.Position, "size": node.Size,
			"pinned": node.Pinned, "level_id": node.LevelID, "region_id": node.RegionID,
		}
		if node.Properties != nil {
			entry["properties"] = compactGameMakerSceneProperties(node.Properties)
		}
		placements := make([]gamemaker.ScenePlacement, 0, 8)
		for _, placement := range scene.Placements {
			if placement.NodeID == node.ID {
				if len(placements) >= 8 {
					truncated = true
					break
				}
				placements = append(placements, placement)
			}
		}
		if len(placements) > 0 {
			entry["placements"] = placements
		}
		colliders := make([]gamemaker.SceneCollider, 0, 8)
		for _, collider := range scene.Colliders {
			if collider.NodeID == node.ID {
				if len(colliders) >= 8 {
					truncated = true
					break
				}
				colliders = append(colliders, collider)
			}
		}
		if len(colliders) > 0 {
			entry["colliders"] = colliders
		}
		attachments := make([]gamemaker.SceneAttachment, 0, 8)
		for _, attachment := range scene.Attachments {
			if attachment.NodeID == node.ID {
				if len(attachments) >= 8 {
					truncated = true
					break
				}
				attachments = append(attachments, attachment)
			}
		}
		if len(attachments) > 0 {
			entry["attachments"] = attachments
		}
		nodes = append(nodes, entry)
	}
	detail["nodes"] = nodes

	regions := detail["regions"].([]gamemaker.SceneRegion)
	for _, region := range scene.Regions {
		_, selectedByNode := selectedRegions[region.ID]
		if filter.RegionID != "" && region.ID != filter.RegionID {
			continue
		}
		if filter.RegionID == "" && len(filter.NodeIDs) > 0 && !selectedByNode {
			continue
		}
		if len(regions) >= 32 {
			truncated = true
			break
		}
		regions = append(regions, region)
	}
	detail["regions"] = regions
	if truncated {
		detail["truncated"] = true
	}
	return detail
}

func gameMakerSceneSummary(scene gamemaker.Scene) map[string]any {
	activeLevel := ""
	levels := make([]string, 0, len(scene.Levels))
	for _, level := range scene.Levels {
		if len(levels) < 16 {
			levels = append(levels, level.ID)
		}
		if level.Active {
			activeLevel = level.ID
		}
	}
	nodeIDs := make([]string, 0, min(64, len(scene.Nodes)))
	for _, node := range scene.Nodes {
		if len(nodeIDs) >= 64 {
			break
		}
		nodeIDs = append(nodeIDs, node.ID)
	}
	regionIDs := make([]string, 0, min(64, len(scene.Regions)))
	for _, region := range scene.Regions {
		if len(regionIDs) >= 64 {
			break
		}
		regionIDs = append(regionIDs, region.ID)
	}
	zoneIDs := make([]string, 0, min(64, len(scene.Zones)))
	for _, zone := range scene.Zones {
		if len(zoneIDs) >= 64 {
			break
		}
		zoneIDs = append(zoneIDs, zone.ID)
	}
	bindings := make([]map[string]any, 0, len(scene.Placements))
	for _, placement := range scene.Placements {
		if len(bindings) >= 64 {
			break
		}
		bindings = append(bindings, map[string]any{
			"id": placement.ID, "node_id": placement.NodeID, "asset_id": placement.AssetID,
			"asset_role": placement.AssetRole, "behavior": placement.Behavior,
		})
	}
	return map[string]any{
		"schema_version": scene.SchemaVersion, "dimension": scene.Dimension, "navigation": scene.Navigation,
		"seed": scene.Seed, "active_level": activeLevel, "levels": levels,
		"node_ids": nodeIDs, "region_ids": regionIDs, "zone_ids": zoneIDs,
		"node_count": len(scene.Nodes), "region_count": len(scene.Regions),
		"placement_count": len(scene.Placements), "collider_count": len(scene.Colliders),
		"attachment_count": len(scene.Attachments), "zone_count": len(scene.Zones),
		"route_count": len(scene.Routes), "bindings": bindings,
	}
}

func gameMakerSceneFirstFailure(result gamemaker.SceneResult, fallback error) string {
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Severity == "error" {
			return diagnostic.Path + ": " + diagnostic.Message
		}
	}
	if fallback != nil {
		return fallback.Error()
	}
	if len(result.Diagnostics) > 0 {
		diagnostic := result.Diagnostics[0]
		return diagnostic.Path + ": " + diagnostic.Message
	}
	return ""
}

func gameMakerSceneResult(operation string, result gamemaker.SceneResult, err error, details ...map[string]any) string {
	summary := gameMakerSceneSummary(result.Scene)
	out := map[string]any{
		"status": "ok", "operation": operation, "path": result.Path, "exists": result.Exists,
		"written": result.Written, "dry_run": result.DryRun, "sha256": result.SHA256,
		"current_sha256": result.CurrentSHA256, "proposed_sha256": result.ProposedSHA256,
		"scene_summary": summary, "diagnostics": result.Diagnostics,
	}
	if result.Changes != nil {
		out["changes"] = result.Changes
	}
	if len(details) > 0 && details[0] != nil {
		out["scene_detail"] = details[0]
	}
	if err != nil {
		out["status"] = "error"
		out["first_failure"] = gameMakerSceneFirstFailure(result, err)
		out["next_action"] = "Inspect the first_failure, correct only the scene payload, then retry with the current sha256. Scene structure does not certify gameplay."
	} else if operation == "scene_inspect" {
		if result.Exists {
			out["next_action"] = "Use this sha256 for a conditional scene_patch or scene_set; run gameplay validation separately."
		} else {
			out["next_action"] = "Create an optional scene with scene_set or continue with source code; custom code remains supported."
		}
	} else {
		out["next_action"] = "Run game_maker_validate with scope gameplay or full. Scene validation covers structure and references, not gameplay quality."
	}
	if len(result.Diagnostics) > 16 {
		out["diagnostics"] = result.Diagnostics[:16]
	}
	return gameMakerToolJSON(out)
}

func gameMakerPlanResult(plan *gamemaker.GamePlan) any {
	if plan == nil {
		return nil
	}
	data, err := json.Marshal(plan)
	if err != nil {
		return plan
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return plan
	}
	if plan.Scene != nil {
		result["scene"] = map[string]any{
			"path": gamemaker.SceneFilePath, "schema_version": plan.Scene.SchemaVersion,
			"dimension": plan.Scene.Dimension, "seed": plan.Scene.Seed,
			"summary": gameMakerSceneSummary(*plan.Scene),
		}
	}
	return result
}

func dispatchGameMakerScene(ctx context.Context, tc ToolCall, service *gamemaker.Service, jobID string) string {
	operation := firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation"))
	expected := toolArgString(tc.Params, "expected_sha256")
	dryRun := gameMakerSceneDryRun(tc.Params)
	switch operation {
	case "scene_inspect":
		filter, filterErr := gameMakerSceneFilterFromParams(tc.Params)
		if filterErr != nil {
			return gameMakerToolError(filterErr)
		}
		result, err := service.InspectScene(ctx, jobID)
		var detail map[string]any
		if err == nil && filter.enabled() {
			detail = gameMakerSceneDetail(result.Scene, filter)
		}
		return gameMakerSceneResult(operation, result, err, detail)
	case "scene_set":
		data, err := gameMakerScenePayload(tc.Params, "scene")
		if err != nil {
			return gameMakerToolError(err)
		}
		result, err := service.SetSceneJSON(ctx, jobID, expected, data, dryRun)
		return gameMakerSceneResult(operation, result, err)
	case "scene_patch":
		data, err := gameMakerScenePayload(tc.Params, "patch")
		if err != nil {
			return gameMakerToolError(err)
		}
		result, err := service.PatchSceneJSON(ctx, jobID, expected, data, dryRun)
		return gameMakerSceneResult(operation, result, err)
	case "scene_generate":
		data, err := gameMakerScenePayload(tc.Params, "generate")
		if err != nil {
			return gameMakerToolError(err)
		}
		var request gamemaker.GenerateSceneRegionRequest
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			return gameMakerToolError(fmt.Errorf("decode generate: %w", err))
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			if err == nil {
				return gameMakerToolError(fmt.Errorf("decode generate: multiple values"))
			}
			return gameMakerToolError(fmt.Errorf("decode generate: %w", err))
		}
		result, err := service.GenerateSceneRegionJSON(ctx, jobID, expected, request, dryRun)
		return gameMakerSceneResult(operation, result, err)
	default:
		return gameMakerToolError(fmt.Errorf("unknown scene operation %q", operation))
	}
}

func gameMakerCheckIDsFromParams(params map[string]interface{}) ([]string, error) {
	raw, exists := params["check_ids"]
	if !exists || raw == nil {
		return nil, nil
	}
	values, ok := raw.([]interface{})
	if !ok {
		if typed, typedOK := raw.([]string); typedOK {
			return append([]string(nil), typed...), nil
		}
		return nil, fmt.Errorf("check_ids must be an array of strings")
	}
	out := make([]string, len(values))
	for i, value := range values {
		id, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("check_ids must be an array of strings")
		}
		out[i] = id
	}
	return out, nil
}
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
		if strings.HasPrefix(operation, "scene_") {
			return dispatchGameMakerScene(ctx, tc, service, jobID), true
		}
		if operation == "get_plan" {
			plan, err := service.GetPlan(ctx, jobID)
			if err != nil {
				return gameMakerToolError(err), true
			}
			return gameMakerToolJSON(map[string]any{"status": "ok", "plan": gameMakerPlanResult(plan)}), true
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
		checkIDs, checkErr := gameMakerCheckIDsFromParams(tc.Params)
		if checkErr != nil {
			return gameMakerToolError(checkErr), true
		}
		result := service.ValidateJobScope(ctx, jobID, toolArgString(tc.Params, "scope"), checkIDs...)
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
