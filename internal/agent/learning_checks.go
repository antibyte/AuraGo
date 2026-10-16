package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"aurago/internal/memory"
	"aurago/internal/security"
	"aurago/internal/services"
)

func learningSupportsTool(name string) bool {
	switch name {
	case "filesystem", "filesystem_op", "file_editor", "json_editor", "yaml_editor", "toml_editor", "homepage", "homepage_file", "virtual_desktop_files", "virtual_desktop", "game_maker_file", "cyd_creator_write_file", "cyd_creator_delete_file":
		return true
	}
	return false
}

func learningScalar(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	b, _ := json.Marshal(v)
	return string(b)
}
func learningToolTarget(tc ToolCall) (resource, field, value string, write bool) {
	switch tc.Action {
	case "filesystem", "filesystem_op":
		r := decodeFilesystemArgs(tc)
		r.Operation = strings.ToLower(strings.TrimSpace(r.Operation))
		if r.Operation == "write" {
			r.Operation = "write_file"
		}
		resource = r.FilePath
		if resource == "" {
			resource = r.Destination
		}
		if r.Operation == "copy" || r.Operation == "move" {
			resource = r.Destination
		}
		switch r.Operation {
		case "write_file", "delete", "create_dir", "move", "copy":
			write = true
		}
		field = "content"
		value = r.Content
		if r.Operation != "write_file" {
			field, value = "operation", r.Operation
		}
	case "file_editor":
		r := decodeFileEditorArgs(tc)
		r.Operation = strings.ToLower(strings.TrimSpace(r.Operation))
		resource = r.FilePath
		write = r.Operation != "read" && r.Operation != "view"
		field = "operation"
		value = r.Operation
	case "json_editor":
		r := decodeJSONEditorArgs(tc)
		r.Operation = strings.ToLower(strings.TrimSpace(r.Operation))
		resource = r.FilePath
		field = r.JsonPath
		value = learningScalar(r.SetValue)
		write = r.Operation == "set" || r.Operation == "delete"
		if r.Operation == "delete" {
			value = "<deleted>"
		}
	case "yaml_editor":
		r := decodeYAMLEditorArgs(tc)
		r.Operation = strings.ToLower(strings.TrimSpace(r.Operation))
		resource = r.FilePath
		field = r.JsonPath
		value = learningScalar(r.SetValue)
		write = r.Operation == "set" || r.Operation == "delete"
		if r.Operation == "delete" {
			value = "<deleted>"
		}
	case "toml_editor":
		r := decodeTOMLEditorArgs(tc)
		r.Operation = strings.ToLower(strings.TrimSpace(r.Operation))
		resource = r.FilePath
		field = r.TomlPath
		value = learningScalar(r.SetValue)
		write = r.Operation == "set" || r.Operation == "delete"
		if r.Operation == "delete" {
			value = "<deleted>"
		}
	case "homepage", "homepage_file", "virtual_desktop_files", "virtual_desktop":
		resource = firstNonEmptyToolString(tc.FilePath, tc.Path, toolArgString(tc.Params, "path", "file_path"))
		operation := firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation"))
		field, value = "operation", operation
		switch operation {
		case "write_file", "write_document":
			write, field, value = true, "content", firstNonEmptyToolString(tc.Content, toolArgString(tc.Params, "content"))
		case "patch_file", "edit_file", "patch_document", "delete", "delete_file", "delete_path", "write_workbook", "set_cell", "set_range":
			write = true
		case "json_edit", "yaml_edit":
			sub := toolArgString(tc.Params, "sub_operation")
			write = sub == "set" || sub == "delete"
			field, value = toolArgString(tc.Params, "json_path"), learningScalar(tc.Params["set_value"])
			if sub == "delete" {
				value = "<deleted>"
			}
		}
	case "game_maker_file", "cyd_creator_write_file", "cyd_creator_delete_file":
		resource = tc.FilePath
		if resource == "" {
			resource = tc.Path
		}
		if v, ok := tc.Params["path"].(string); ok {
			resource = v
		}
		field = "content"
		value = tc.Content
		if v, ok := tc.Params["content"].(string); ok {
			value = v
		}
		operation := firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation"))
		write = tc.Action != "game_maker_file" || operation == "write" || operation == "replace"
		if operation == "replace" || tc.Action == "cyd_creator_delete_file" {
			field, value = "operation", operation
			if tc.Action == "cyd_creator_delete_file" {
				value = "delete"
			}
		}
	}
	resource = strings.TrimPrefix(strings.ReplaceAll(resource, "\\", "/"), "/workspace/")
	if strings.HasPrefix(resource, "/") || strings.Contains(resource, ":") || resource == "" {
		return "", field, value, false
	}
	resource = path.Clean(resource)
	if resource == ".." || strings.HasPrefix(resource, "../") {
		write = false
	}
	return
}

type learningWriteTarget struct{ Resource, Field, Value string }

func learningToolTargets(tc ToolCall) []learningWriteTarget {
	var targets []learningWriteTarget
	add := func(call ToolCall) {
		if resource, field, value, write := learningToolTarget(call); write {
			targets = append(targets, learningWriteTarget{resource, field, value})
		}
	}
	if tc.Action == "filesystem" || tc.Action == "filesystem_op" {
		r := decodeFilesystemArgs(tc)
		r.Operation = strings.ToLower(strings.TrimSpace(r.Operation))
		if strings.HasSuffix(r.Operation, "_batch") {
			for _, item := range r.Items[:min(len(r.Items), 1000)] {
				call := ToolCall{Action: tc.Action, Operation: strings.TrimSuffix(r.Operation, "_batch"), Params: item}
				add(call)
				if call.Operation == "move" {
					add(ToolCall{Action: tc.Action, Operation: "delete", FilePath: toolArgString(item, "file_path", "path", "source")})
				}
			}
			return targets
		}
		if r.Operation == "move" {
			add(ToolCall{Action: tc.Action, Operation: "delete", FilePath: r.FilePath})
		}
	}
	add(tc)
	return targets
}

// checkLearningAction pauses only the current undispatched call, never its batch siblings.
func checkLearningAction(ctx context.Context, tc ToolCall, dc *DispatchContext) (blocked, advisory string) {
	if dc == nil || dc.MemoryCorrections == nil || dc.ShortTermMem == nil || !learningPolicy(dc.Cfg, false) {
		return "", ""
	}
	targets := learningToolTargets(tc)
	if len(targets) == 0 {
		return "", ""
	}
	var conflicts []memory.Learning
	var hints []memory.Learning
	var writes []services.WriteTarget
	seen := map[string]bool{}
	for _, target := range targets {
		scope := dc.LearningScope
		if dc.LearningTargetScope != nil {
			var ok bool
			scope, ok = dc.LearningTargetScope(tc.Action, target.Resource)
			if !ok {
				continue
			}
		} else if scope.Project == "" {
			scope.Project = dc.CodeProject.ID
		}
		if scope.Project == "" {
			continue
		}
		scope.Resource = target.Resource
		list, err := dc.ShortTermMem.ListLearnings(ctx, scope, "", "active", 200)
		if err != nil {
			return learningResponse(nil, err), ""
		}
		writeTarget := services.WriteTarget{Run: dc.MemoryCorrections.id, Lineage: dc.WorkChainID, Project: scope.Project, Resource: target.Resource}
		if scope.Project == dc.CodeProject.ID || scope.Project == dc.LearningScope.Project {
			writeTarget.Symbol = dc.CodeGraph.KnownWriteSymbol(ctx, dc.CodeProject, target.Resource, toolArgInt(tc.Params, 0, "start_line", "line_start"), toolArgInt(tc.Params, 0, "end_line"))
		}
		writes = append(writes, writeTarget)
		for _, l := range list {
			if seen[l.ID] {
				continue
			}
			if c := l.Condition; c != nil && c.Target == target.Resource && c.Field == target.Field && c.Value != target.Value {
				conflicts = append(conflicts, l)
				seen[l.ID] = true
			} else if c == nil && l.Scope.Resource == target.Resource && len(hints) < 3 {
				hints = append(hints, l)
				seen[l.ID] = true
			}
		}
	}
	if len(conflicts) > 0 {
		args := map[string]any{}
		for k, v := range tc.Params {
			if k != "learning_reason" && k != "_todo" {
				args[k] = v
			}
		}
		revisions := map[string]int{}
		for _, l := range conflicts {
			revisions[l.ID] = l.Revision
		}
		boundCall := tc
		boundCall.Params = args
		b, _ := json.Marshal([]any{boundCall, targets, writes, revisions})
		hash := sha256.Sum256(b)
		key := hex.EncodeToString(hash[:])
		run := dc.MemoryCorrections
		run.mu.Lock()
		if run.learningChecks == nil {
			run.learningChecks = map[string]bool{}
		}
		warned := run.learningChecks[key]
		reason, _ := tc.Params["learning_reason"].(string)
		if !warned || strings.TrimSpace(reason) == "" || len(reason) > 1000 {
			if len(run.learningChecks) < 128 {
				run.learningChecks[key] = true
			}
			run.mu.Unlock()
			if !dc.IsCoAgent && !isCoAgentSession(dc.SessionID) && learningPolicy(dc.Cfg, true) {
				_ = dc.ShortTermMem.RecordLearningMetric(ctx, "conflicts", 1)
			}
			return learningResponse(map[string]any{"status": "review_required", "executed": false, "conflicts": conflicts, "next_action": "Reconsider this action. Adjust it, or resubmit the same action with learning_reason explaining the deliberate deviation. Permissions still apply."}, nil), ""
		}
		run.mu.Unlock()
		if !dc.IsCoAgent && !isCoAgentSession(dc.SessionID) && learningPolicy(dc.Cfg, true) {
			if err := dc.ShortTermMem.RecordLearningDeviation(ctx, run.id, key, reason, revisions); err != nil {
				return learningResponse(nil, err), ""
			}
		} else if dc.Logger != nil {
			dc.Logger.Info("Learning conflict reconsidered without memory writes", "run", run.id, "arguments_hash", key, "reason", security.Scrub(reason), "revisions", revisions)
		} else {
			return learningResponse(nil, fmt.Errorf("learning deviation audit unavailable")), ""
		}
	}
	if dc.WriteTargets != nil {
		var overlaps []services.WriteTarget
		for _, write := range writes {
			overlaps = append(overlaps, dc.WriteTargets.Note(write)...)
		}
		if len(overlaps) > 0 {
			advisory = "\n" + learningResponse(map[string]any{"overlapping_work": overlaps[:min(20, len(overlaps))], "advisory": "Another active work chain has registered this write target. Existing project locks remain authoritative."}, nil)
		}
	}
	if len(hints) > 0 {
		advisory += "\n" + learningResponse(map[string]any{"resource_learnings": hints, "advisory": "Relevant observations; unstructured assertions did not interrupt this action."}, nil)
	}
	return "", advisory
}
